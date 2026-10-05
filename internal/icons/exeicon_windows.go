//go:build windows

package icons

import (
	"bytes"
	"errors"
	"image/png"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The icon of a program on Windows, read the way the Explorer reads it:
// the shell's jumbo image list (256 px) for the file, which resolves a
// shortcut on its own, and SHGetFileInfo's 32 px icon as the fallback.

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	user32  = windows.NewLazySystemDLL("user32.dll")
	gdi32   = windows.NewLazySystemDLL("gdi32.dll")

	procSHGetFileInfoW = shell32.NewProc("SHGetFileInfoW")
	procSHGetImageList = shell32.NewProc("SHGetImageList")
	procGetIconInfo    = user32.NewProc("GetIconInfo")
	procDestroyIcon    = user32.NewProc("DestroyIcon")
	procGetDC          = user32.NewProc("GetDC")
	procReleaseDC      = user32.NewProc("ReleaseDC")
	procGetDIBits      = gdi32.NewProc("GetDIBits")
	procGetObjectW     = gdi32.NewProc("GetObjectW")
	procDeleteObject   = gdi32.NewProc("DeleteObject")
)

// SHGetFileInfo flags (shellapi.h): SHGFI_ICON (0x100) gives an icon
// handle, SHGFI_SYSICONINDEX (0x4000) the system image list index, and
// SHGFI_LARGEICON (0, the default) the large (32 px) size.
const (
	shgfiIcon         = 0x000000100
	shgfiSysIconIndex = 0x000004000
	shgfiLargeIcon    = 0x000000000
)

// shilJumbo is SHIL_JUMBO (4), the 256 px image list, and
// ildTransparent is ILD_TRANSPARENT (1), the draw flag GetIcon wants.
const (
	shilJumbo      = 4
	ildTransparent = 0x1
)

// imageListGetIcon is the slot of IImageList::GetIcon in the vtable:
// three of IUnknown, then Add, ReplaceIcon, SetOverlayImage, Replace,
// AddMasked, Draw, Remove, GetIcon.
const imageListGetIcon = 10

// iidImageList is IID_IImageList.
var iidImageList = windows.GUID{
	Data1: 0x46EB5926, Data2: 0x582E, Data3: 0x4017,
	Data4: [8]byte{0x9F, 0xDF, 0xE8, 0x99, 0x8D, 0xAA, 0x09, 0x50},
}

// shFileInfo is SHFILEINFOW (696 bytes on 64-bit Windows).
type shFileInfo struct {
	Icon        windows.Handle
	IconIndex   int32
	Attributes  uint32
	DisplayName [260]uint16
	TypeName    [80]uint16
}

// iconInfo is ICONINFO (32 bytes on 64-bit Windows).
type iconInfo struct {
	IsIcon             int32
	HotspotX, HotspotY uint32
	Mask               windows.Handle
	Colour             windows.Handle
}

// bitmap is BITMAP (32 bytes on 64-bit Windows), read with GetObject to
// know an icon's size.
type bitmap struct {
	Type, Width, Height, WidthBytes int32
	Planes, BitsPixel               uint16
	Bits                            uintptr
}

// bitmapInfoHeader is BITMAPINFOHEADER (40 bytes); a negative height
// asks GetDIBits for a top-down image.
type bitmapInfoHeader struct {
	Size          uint32
	Width, Height int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// bitmapInfo is BITMAPINFO for a 1-bit image: the header followed by
// the two palette entries GDI writes behind it. Without room for them
// GetDIBits would write past the header.
type bitmapInfo struct {
	Header  bitmapInfoHeader
	Palette [2]uint32
}

// shellIconPNG is the icon of path as a PNG: jumbo first, large second.
// The shell resolves shortcuts through COM, which has to be initialised
// on the calling thread, and the device context from GetDC must be
// released on the thread that took it; callers are goroutines that Go
// may move between threads, so the thread is locked for the whole call.
func shellIconPNG(path string) ([]byte, bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// S_FALSE (already initialised here) still counts and needs its
	// CoUninitialize; x/sys reports it as an error value. A thread in a
	// different mode (RPC_E_CHANGED_MODE, 0x80010106) is usable as it
	// is, but must not be uninitialised by us.
	err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	if err == nil || errors.Is(err, syscall.Errno(windows.S_FALSE)) {
		defer windows.CoUninitialize()
	}

	// A handle that cannot be drawn (a broken jumbo icon, say) must not
	// hide the 32 px one, so the second source gets its turn.
	icon := jumboIcon(path)
	if icon != 0 {
		raw, ok := drawAndDestroy(icon)
		if ok {
			return raw, true
		}
	}

	icon = largeIcon(path)
	if icon == 0 {
		return nil, false
	}

	return drawAndDestroy(icon)
}

// drawAndDestroy turns an icon handle into a PNG and always destroys the
// handle, whether or not the drawing worked.
func drawAndDestroy(icon windows.Handle) ([]byte, bool) {
	defer func() { _, _, _ = procDestroyIcon.Call(uintptr(icon)) }()

	return iconPNG(icon)
}

// largeIcon is SHGetFileInfo's 32 px icon.
func largeIcon(path string) windows.Handle {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}

	var info shFileInfo
	found, _, _ := procSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
		shgfiIcon|shgfiLargeIcon,
	)
	if found == 0 {
		return 0
	}

	return info.Icon
}

// jumboIcon asks the system image list for the 256 px icon of path:
// SHGetFileInfo gives the index, SHGetImageList the list, and GetIcon
// (called through the COM vtable) the handle.
func jumboIcon(path string) windows.Handle {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}

	var info shFileInfo
	found, _, _ := procSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
		shgfiSysIconIndex,
	)
	if found == 0 {
		return 0
	}

	var list uintptr
	result, _, _ := procSHGetImageList.Call(
		shilJumbo,
		uintptr(unsafe.Pointer(&iidImageList)),
		uintptr(unsafe.Pointer(&list)),
	)
	if result != 0 || list == 0 {
		return 0
	}
	defer release(list)

	// The first word of a COM object is a pointer to its vtable.
	vtable := *(**[imageListGetIcon + 1]uintptr)(comObject(&list))

	var icon windows.Handle
	result, _, _ = syscall.SyscallN(
		vtable[imageListGetIcon],
		list,
		uintptr(info.IconIndex),
		ildTransparent,
		uintptr(unsafe.Pointer(&icon)),
	)
	if result != 0 {
		return 0
	}

	return icon
}

// release calls IUnknown::Release, slot 2 of any COM vtable.
func release(object uintptr) {
	vtable := *(**[3]uintptr)(comObject(&object))
	_, _, _ = syscall.SyscallN(vtable[2], object)
}

// comObject reads the address of a COM object out of the variable that
// holds it. Going through the variable's own address, instead of
// converting the uintptr back, is what keeps go vet quiet about
// unsafe.Pointer: the address came from a syscall, not from Go memory.
func comObject(holder *uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(holder))
}

// iconPNG draws an icon's pixels into a PNG: GetIconInfo gives the
// colour and mask bitmaps, GetDIBits their bytes, nrgbaFromDIB the image.
func iconPNG(icon windows.Handle) ([]byte, bool) {
	var info iconInfo
	found, _, _ := procGetIconInfo.Call(
		uintptr(icon), uintptr(unsafe.Pointer(&info)),
	)
	if found == 0 {
		return nil, false
	}

	// GetIconInfo hands over two bitmaps the caller must delete.
	defer func() { _, _, _ = procDeleteObject.Call(uintptr(info.Mask)) }()
	defer func() { _, _, _ = procDeleteObject.Call(uintptr(info.Colour)) }()

	var bmp bitmap
	_, _, _ = procGetObjectW.Call(
		uintptr(info.Colour),
		unsafe.Sizeof(bmp),
		uintptr(unsafe.Pointer(&bmp)),
	)

	width, height := int(bmp.Width), int(bmp.Height)
	if width <= 0 || height <= 0 {
		return nil, false
	}

	screen, _, _ := procGetDC.Call(0)
	defer func() { _, _, _ = procReleaseDC.Call(0, screen) }()

	colour := dibBits(screen, info.Colour, width, height, 32)
	mask := dibBits(screen, info.Mask, width, height, 1)

	// A blank canvas is refused here, so that shellIconPNG goes on to
	// the 32 px icon instead of saving a transparent square.
	img, drawn := drawnPart(nrgbaFromDIB(width, height, colour, mask))
	if !drawn {
		return nil, false
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, false
	}

	return out.Bytes(), true
}

// dibBits reads a bitmap top-down at the given depth.
func dibBits(
	screen uintptr, handle windows.Handle, width, height int, bits uint16,
) []byte {
	info := bitmapInfo{
		Header: bitmapInfoHeader{
			Width:    int32(width),
			Height:   -int32(height),
			Planes:   1,
			BitCount: bits,
		},
	}
	info.Header.Size = uint32(unsafe.Sizeof(info.Header))

	// Each row is padded to a multiple of 4 bytes.
	stride := ((width*int(bits) + 31) / 32) * 4
	buffer := make([]byte, stride*height)

	// The last argument is DIB_RGB_COLORS (0).
	lines, _, _ := procGetDIBits.Call(
		screen,
		uintptr(handle),
		0,
		uintptr(height),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&info)),
		0,
	)
	if lines == 0 {
		return nil
	}

	return buffer
}
