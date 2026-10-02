package discover

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf16"
)

// A Windows shortcut (.lnk) as MS-SHLLINK describes it. hopto reads the
// target path, the arguments and the icon location: enough to tell a
// program from an uninstaller, to draw its icon and to see an Edge web
// app behind msedge_proxy.exe. The shortcut itself is what gets opened.

// ShellLink is what hopto keeps of a shortcut.
type ShellLink struct {
	Target       string
	Args         string
	IconLocation string
	IconIndex    int32
}

// ErrNotShellLink answers a file that is not a shortcut, or one that is
// cut short or damaged.
var ErrNotShellLink = errors.New("not a shell link")

// headerSize is the fixed size of ShellLinkHeader (2.1); every shortcut
// starts with this value as a little-endian uint32.
const headerSize = 76

// linkCLSID is the class id every shortcut carries right after the size
// of its header (2.1, LinkCLSID).
var linkCLSID = []byte{
	0x01, 0x14, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00,
	0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46,
}

// LinkFlags (2.1.1): which optional structures follow the header, and
// whether StringData is UTF-16.
const (
	flagHasIDList       = 1 << 0
	flagHasLinkInfo     = 1 << 1
	flagHasName         = 1 << 2
	flagHasRelativePath = 1 << 3
	flagHasWorkingDir   = 1 << 4
	flagHasArguments    = 1 << 5
	flagHasIconLocation = 1 << 6
	flagIsUnicode       = 1 << 7
)

// LinkInfo (2.3): the flag that says a local path is present, and the
// smallest header that carries the Unicode offsets.
const (
	linkInfoLocalPath     = 1 << 0
	linkInfoUnicodeHeader = 0x24
)

// environmentBlock is the signature of EnvironmentVariableDataBlock
// (2.5.4), which carries the target with %VARIABLES% unexpanded.
const environmentBlock = 0xA0000001

// environmentBlockSize is the only size the specification allows for
// that block: size, signature, 260 ANSI bytes and 260 UTF-16 units.
const environmentBlockSize = 8 + 260 + 520

// envVariable matches %NAME% the way Windows expands it.
var envVariable = regexp.MustCompile(`%([^%]+)%`)

// reader walks the bytes; every read checks the bounds so a truncated
// or hostile file ends in an error, never in a panic. The first failure
// is kept in err and every later read becomes a no-op.
type reader struct {
	data []byte
	pos  int
	err  error
}

// uint16At reads a little-endian uint16 at the cursor and moves it.
func (r *reader) uint16At() uint16 {
	if r.err != nil || r.pos+2 > len(r.data) {
		r.err = ErrNotShellLink

		return 0
	}

	value := binary.LittleEndian.Uint16(r.data[r.pos:])
	r.pos += 2

	return value
}

// uint32At reads a little-endian uint32 at the cursor and moves it.
func (r *reader) uint32At() uint32 {
	if r.err != nil || r.pos+4 > len(r.data) {
		r.err = ErrNotShellLink

		return 0
	}

	value := binary.LittleEndian.Uint32(r.data[r.pos:])
	r.pos += 4

	return value
}

// bytesAt returns the next count bytes and moves the cursor past them.
func (r *reader) bytesAt(count int) []byte {
	if r.err != nil || count < 0 || r.pos+count > len(r.data) {
		r.err = ErrNotShellLink

		return nil
	}

	out := r.data[r.pos : r.pos+count]
	r.pos += count

	return out
}

// skip moves the cursor past count bytes.
func (r *reader) skip(count int) {
	r.bytesAt(count)
}

// stringData reads one StringData field: a character count, then the
// characters, UTF-16 or ANSI (taken as Latin-1, close enough for a path).
func (r *reader) stringData(unicode bool) string {
	count := int(r.uint16At())
	if unicode {
		raw := r.bytesAt(2 * count)
		if r.err != nil {
			return ""
		}

		units := make([]uint16, count)
		for index := range units {
			units[index] = binary.LittleEndian.Uint16(raw[2*index:])
		}

		return string(utf16.Decode(units))
	}

	raw := r.bytesAt(count)
	if r.err != nil {
		return ""
	}

	return latin1(raw)
}

// latin1 turns ANSI bytes into a string one rune per byte.
func latin1(raw []byte) string {
	runes := make([]rune, len(raw))
	for index, value := range raw {
		runes[index] = rune(value)
	}

	return string(runes)
}

// cString reads a NUL-terminated ANSI string at an absolute offset.
func cString(data []byte, offset int) string {
	if offset < 0 || offset >= len(data) {
		return ""
	}

	end := offset
	for end < len(data) && data[end] != 0 {
		end++
	}

	return latin1(data[offset:end])
}

// cWideString reads a NUL-terminated UTF-16 string at an absolute offset.
func cWideString(data []byte, offset int) string {
	if offset < 0 || offset+1 >= len(data) {
		return ""
	}

	var units []uint16
	for pos := offset; pos+1 < len(data); pos += 2 {
		unit := binary.LittleEndian.Uint16(data[pos:])
		if unit == 0 {
			break
		}

		units = append(units, unit)
	}

	return string(utf16.Decode(units))
}

// ParseShellLink reads the bytes of a shortcut. The target comes from
// LinkInfo when there is one, else from the environment block, else from
// the relative path.
func ParseShellLink(data []byte) (ShellLink, error) {
	var link ShellLink

	if len(data) < headerSize ||
		binary.LittleEndian.Uint32(data) != headerSize ||
		string(data[4:20]) != string(linkCLSID) {
		return link, ErrNotShellLink
	}

	flags := binary.LittleEndian.Uint32(data[20:])
	link.IconIndex = int32(binary.LittleEndian.Uint32(data[56:]))

	reading := &reader{data: data, pos: headerSize}

	// The IDList is a uint16 size and a list we do not need: the target
	// is read from LinkInfo, whose path is plain text.
	if flags&flagHasIDList != 0 {
		reading.skip(int(reading.uint16At()))
	}

	if flags&flagHasLinkInfo != 0 {
		start := reading.pos
		size := int(reading.uint32At())
		link.Target = linkInfoPath(data, start, size)

		// The structure is skipped by its own size, counted from where
		// it started, whatever linkInfoPath made of its contents.
		reading.pos = start
		reading.skip(size)
	}

	unicode := flags&flagIsUnicode != 0

	if flags&flagHasName != 0 {
		reading.stringData(unicode)
	}

	relative := ""
	if flags&flagHasRelativePath != 0 {
		relative = reading.stringData(unicode)
	}

	if flags&flagHasWorkingDir != 0 {
		reading.stringData(unicode)
	}

	if flags&flagHasArguments != 0 {
		link.Args = reading.stringData(unicode)
	}

	if flags&flagHasIconLocation != 0 {
		link.IconLocation = reading.stringData(unicode)
	}

	if reading.err != nil {
		return link, reading.err
	}

	if link.Target == "" {
		link.Target = environmentTarget(reading)
	}

	if link.Target == "" {
		link.Target = relative
	}

	return link, nil
}

// linkInfoPath reads LocalBasePath (Unicode when the header has the
// offset) plus CommonPathSuffix out of a LinkInfo at start.
func linkInfoPath(data []byte, start, size int) string {
	if size < 28 || start < 0 || start+size > len(data) {
		return ""
	}

	info := data[start : start+size]
	headerLen := int(binary.LittleEndian.Uint32(info[4:]))
	flags := binary.LittleEndian.Uint32(info[8:])
	if flags&linkInfoLocalPath == 0 {
		return ""
	}

	base := cString(info, int(binary.LittleEndian.Uint32(info[16:])))
	suffix := cString(info, int(binary.LittleEndian.Uint32(info[24:])))

	// A header of 0x24 bytes or more adds the Unicode offsets, which
	// win over the ANSI ones because they do not lose characters.
	if headerLen >= linkInfoUnicodeHeader && len(info) >= 36 {
		offset := int(binary.LittleEndian.Uint32(info[28:]))
		if offset != 0 {
			base = cWideString(info, offset)
		}

		offset = int(binary.LittleEndian.Uint32(info[32:]))
		if offset != 0 {
			suffix = cWideString(info, offset)
		}
	}

	return base + suffix
}

// environmentTarget walks the extra data blocks after StringData for
// the environment block, whose target still has its %VARIABLES%; the
// Unicode copy is preferred.
func environmentTarget(reading *reader) string {
	for reading.err == nil {
		start := reading.pos
		size := int(reading.uint32At())
		if reading.err != nil || size < 4 {
			return ""
		}

		signature := reading.uint32At()

		// The size is checked against the data before slicing: a cut
		// file may announce a block it does not hold.
		if signature == environmentBlock &&
			size == environmentBlockSize &&
			start+size <= len(reading.data) {
			block := reading.data[start : start+size]
			target := cWideString(block, 268)
			if target == "" {
				target = cString(block, 8)
			}

			return expandEnvironment(target)
		}

		reading.pos = start
		reading.skip(size)
	}

	return ""
}

// expandEnvironment replaces %NAME% with the variable's value; an
// unknown name is left as it was so the path still reads.
func expandEnvironment(path string) string {
	return envVariable.ReplaceAllStringFunc(path, func(match string) string {
		name := strings.Trim(match, "%")
		if value, ok := os.LookupEnv(name); ok {
			return value
		}

		return match
	})
}

// ReadShellLink reads a shortcut from disk. A relative target is taken
// from the shortcut's own folder.
func ReadShellLink(path string) (ShellLink, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ShellLink{}, err
	}

	link, err := ParseShellLink(data)
	if err != nil {
		return link, err
	}

	// The colon test stands in for a drive letter: filepath.IsAbs only
	// knows C:\ on Windows, and the parser is tested on every OS.
	if link.Target != "" && !filepath.IsAbs(link.Target) &&
		!strings.Contains(link.Target, ":") {
		link.Target = filepath.Join(filepath.Dir(path), link.Target)
	}

	return link, nil
}
