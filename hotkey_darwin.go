package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Carbon -framework Cocoa -framework CoreGraphics

#include <Carbon/Carbon.h>
#include <Cocoa/Cocoa.h>
#include <CoreGraphics/CoreGraphics.h>
#include <objc/runtime.h>

// Implemented in Go below; Carbon calls the C handler, which calls these.
extern void launcherHotkeyPressed(UInt32 id);
extern void launcherHotkeyRegistered(UInt32 id, OSStatus status);
extern void launcherReopened(void);

// One handler serves every shortcut: the hot key id stored at registration
// comes back in the event, and Go decides what each id means.
static OSStatus hotkeyHandler(EventHandlerCallRef next, EventRef event, void *data) {
	EventHotKeyID hotkeyID;
	GetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID, NULL, sizeof(hotkeyID), NULL, &hotkeyID);
	launcherHotkeyPressed(hotkeyID.id);
	return noErr;
}

// Carbon hot keys are global and need no accessibility permission, but they
// must be registered from the main thread, hence the dispatch to the main
// queue. Wails owns that thread, so this runs once its loop is up. The
// handler is installed once; every registration after the first only adds
// a key. The status goes back to Go so it ends up in the log: a combination
// the system already uses is refused here, silently otherwise.
static void registerHotkey(UInt32 id, UInt32 keyCode, UInt32 modifiers) {
	dispatch_async(dispatch_get_main_queue(), ^{
		static BOOL installed = NO;
		if (!installed) {
			EventTypeSpec spec = { kEventClassKeyboard, kEventHotKeyPressed };
			InstallApplicationEventHandler(&hotkeyHandler, 1, &spec, NULL, NULL);
			installed = YES;
		}

		// Four-letter signature that identifies our hot keys to Carbon.
		EventHotKeyID hotkeyID = { 'hopt', id };
		EventHotKeyRef ref;
		OSStatus status = RegisterEventHotKey(keyCode, modifiers, hotkeyID, GetApplicationEventTarget(), 0, &ref);
		launcherHotkeyRegistered(id, status);
	});
}

// The screen the launcher should appear on: the one under the mouse, which
// is what Alfred does by default and matches where the user is looking.
// Falls back to the main screen when the pointer is nowhere (it happens
// during display changes).
static NSScreen *screenUnderMouse(void) {
	NSPoint mouse = [NSEvent mouseLocation];
	for (NSScreen *screen in [NSScreen screens]) {
		if (NSMouseInRect(mouse, screen.frame, NO)) {
			return screen;
		}
	}
	return [NSScreen mainScreen];
}

// How centerOnScreen picks the screen; centerWindow maps the setting.
enum { placeMain = 0, placeMouse = 1, placeDisplay = 2 };

// hopto's own window. Looked up by the Wails class rather than taken as
// the first window: the menu bar item is a window of the app as well.
static NSWindow *launcherWindow(void) {
	Class wailsClass = NSClassFromString(@"WailsWindow");
	for (NSWindow *window in [NSApp windows]) {
		if (wailsClass != nil && [window isKindOfClass:wailsClass]) {
			return window;
		}
	}
	return [NSApp windows].firstObject;
}

// The id macOS gives a screen, which is its CGDirectDisplayID; 0 for no
// screen at all.
static uint32_t displayNumber(NSScreen *screen) {
	NSNumber *number = screen.deviceDescription[@"NSScreenNumber"];
	return number.unsignedIntValue;
}

// The attached screen with that id, or nil once it has been unplugged.
static NSScreen *screenWithNumber(uint32_t display) {
	for (NSScreen *screen in [NSScreen screens]) {
		if (displayNumber(screen) == display) {
			return screen;
		}
	}
	return nil;
}

// The screen for this show. The main screen is the one with the menu
// bar, the first of [NSScreen screens]: mainScreen is the one holding
// the key window, which says nothing while the panel is hidden. A
// display unplugged since Go checked falls back to the main screen too.
static NSScreen *chosenScreen(int placement, uint32_t display) {
	if (placement == placeMouse) {
		return screenUnderMouse();
	}

	if (placement == placeDisplay) {
		NSScreen *screen = screenWithNumber(display);
		if (screen != nil) {
			return screen;
		}
	}

	NSScreen *primary = [NSScreen screens].firstObject;
	return primary != nil ? primary : [NSScreen mainScreen];
}

// Centres the window on the chosen screen, a little above the middle so
// it reads as an overlay, and makes sure the window behaves as one:
// present in every Space (with "displays have separate Spaces" a window
// bound to one Space made macOS switch Spaces, so the launcher seemed to
// jump to the other display), allowed over full-screen apps, with a
// shadow. Synchronous so the window is in place before it is shown; the
// caller is never the main thread, otherwise this would deadlock.
static void centerOnScreen(int placement, uint32_t display) {
	dispatch_sync(dispatch_get_main_queue(), ^{
		NSWindow *window = launcherWindow();
		window.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces
			| NSWindowCollectionBehaviorFullScreenAuxiliary
			| NSWindowCollectionBehaviorStationary;

		// A window macOS believes opaque gets a rectangular shadow and a
		// faint square edge around the rounded panel; telling it the truth
		// (transparent, no colour) makes the shadow follow the panel.
		window.opaque = NO;
		window.backgroundColor = [NSColor clearColor];
		window.alphaValue = 1.0;
		window.hasShadow = YES;

		// Whatever the web view paints outside the panel (a faint square was
		// still visible), the content view is clipped to the same radius as
		// the panel, so the window itself has round corners.
		NSView *content = window.contentView;
		content.wantsLayer = YES;
		content.layer.cornerRadius = 20;
		content.layer.masksToBounds = YES;
		content.layer.backgroundColor = [NSColor clearColor].CGColor;

		// The WKWebView must not draw its own (white) background either.
		for (NSView *view in content.subviews) {
			if ([view isKindOfClass:NSClassFromString(@"WKWebView")]) {
				[view setValue:@NO forKey:@"drawsBackground"];
			}
		}

		NSRect screen = chosenScreen(placement, display).visibleFrame;
		NSRect frame = window.frame;

		frame.origin.x = screen.origin.x + (screen.size.width - frame.size.width) / 2;
		frame.origin.y = screen.origin.y + (screen.size.height - frame.size.height) / 2 + screen.size.height * 0.08;
		[window setFrameOrigin:frame.origin];
	});
}

// The display hopto's window is on now, 0 when it is on none. AppKit
// only answers on the main thread, so this waits for it: never call it
// from the main thread.
static uint32_t windowDisplayNumber(void) {
	__block uint32_t display = 0;
	dispatch_sync(dispatch_get_main_queue(), ^{
		display = displayNumber(launcherWindow().screen);
	});
	return display;
}

// Fills ids with the attached displays, at most max, and says how many.
// CoreGraphics answers from any thread, so there is no dispatch here.
static int listDisplays(uint32_t *ids, int max) {
	uint32_t count = 0;
	if (CGGetActiveDisplayList(max, ids, &count) != kCGErrorSuccess) {
		return 0;
	}
	return (int)count;
}

// The delegate's own reopen method, if it ever has one: ours runs first
// and then hands over to it.
static IMP originalReopen = NULL;

// What macOS calls when the running app is launched again (Alfred,
// `open -a`, the Finder, the Dock). Go only takes note; the window is
// shown from a goroutine, never from here.
static BOOL reopenHandler(id delegate, SEL command, NSApplication *app, BOOL visible) {
	launcherReopened();

	if (originalReopen != NULL) {
		return ((BOOL (*)(id, SEL, NSApplication *, BOOL))originalReopen)(delegate, command, app, visible);
	}

	// NO: hopto has shown its panel itself, AppKit has nothing to add.
	return NO;
}

// Wails' AppDelegate has no applicationShouldHandleReopen:, so a launch
// of the running app does nothing. The method is added to the delegate's
// class at run time, or put in front of the one it has. The block runs
// on the main queue, which only starts once Wails has set the delegate.
static void installReopenHandler(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		id delegate = [NSApp delegate];
		if (delegate == nil) {
			return;
		}

		Class delegateClass = object_getClass(delegate);
		SEL selector = @selector(applicationShouldHandleReopen:hasVisibleWindows:);
		struct objc_method_description description = protocol_getMethodDescription(@protocol(NSApplicationDelegate), selector, NO, YES);

		if (!class_addMethod(delegateClass, selector, (IMP)reopenHandler, description.types)) {
			Method existing = class_getInstanceMethod(delegateClass, selector);
			originalReopen = method_setImplementation(existing, (IMP)reopenHandler);
		}
	});
}

// Wails sets the activation policy to Regular while starting, which
// overrides LSUIElement in the plist: the launcher would get a Dock icon and
// a slot in Cmd+Tab. Accessory is what a Cmd+Tab-like overlay wants: no
// Dock, no app switcher, and it can still take the keyboard when shown.
static void becomeAccessory(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
	});
}

// An accessory app (no Dock icon) does not come to the front on its own, so
// the window would show without keyboard focus. This forces the activation.
// The policy is set again first in case something regular slipped through.
static void activateApp(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
		[NSApp activateIgnoringOtherApps:YES];
		// The shadow is computed from the drawn content; once the page has
		// painted the panel, ask for it again so it hugs the rounded shape.
		[[NSApp windows].firstObject invalidateShadow];
	});
}
*/
import "C"

import "log"

// onHotkey is what the Carbon handler calls, with the tab of the pressed
// shortcut. It is a variable so the App can install its own toggle without
// the C side knowing about it.
var onHotkey = func(tab string) {}

// onReopen is what a launch of the running app ends in; startup puts
// showFromOutside here through handleReopen.
var onReopen = func() {}

// maxDisplays is more screens than a Mac drives at once.
const maxDisplays = 16

//export launcherHotkeyPressed
func launcherHotkeyPressed(id C.UInt32) {
	tab := tabForHotkey(uint32(id))

	log.Printf("hotkey %d pressed: %s", id, tab)

	// The handler runs on the main thread; the Wails runtime calls are
	// dispatched from a goroutine so they never block that thread.
	go onHotkey(tab)
}

//export launcherHotkeyRegistered
func launcherHotkeyRegistered(id C.UInt32, status C.OSStatus) {
	recordHotkeyStatus(uint32(id), int32(status))

	if status != 0 {
		log.Printf("hotkey %d: RegisterEventHotKey failed with status %d", id, status)
		return
	}

	log.Printf("hotkey %d registered", id)
}

//export launcherReopened
func launcherReopened() {
	log.Printf("reopen: hopto launched again")

	// The delegate calls this on the main thread; the Wails runtime is
	// only ever used from a goroutine, as for the hotkeys.
	go onReopen()
}

// becomeAccessory removes the launcher from the Dock and from Cmd+Tab once
// the Wails main loop is up. Called from startup.
func becomeAccessory() {
	C.becomeAccessory()
}

// registerToggleHotkeys binds the apps shortcut to the apps tab and the
// links shortcut to the links tab. The specs come from the library; the
// defaults are in hotkeyspec.go.
func registerToggleHotkeys(toggle func(tab string), apps, links Hotkey) {
	onHotkey = toggle
	C.registerHotkey(hotkeyApps, C.UInt32(apps.KeyCode), C.UInt32(apps.Modifiers))
	C.registerHotkey(hotkeyLinks, C.UInt32(links.KeyCode), C.UInt32(links.Modifiers))
}

// centerWindow moves the hidden window to the middle of the screen
// screenChoice picked and sets the overlay window behaviour. Call it
// from a goroutine, never from the main thread.
func centerWindow(mode string, display uint32) {
	placement := C.placeMain
	switch mode {
	case screenMouse:
		placement = C.placeMouse
	case screenLast:
		placement = C.placeDisplay
	}

	C.centerOnScreen(C.int(placement), C.uint32_t(display))
}

// currentDisplay is the display hopto's window is on, 0 for none. Call
// it from a goroutine, never from the main thread.
func currentDisplay() uint32 {
	return uint32(C.windowDisplayNumber())
}

// activeDisplays lists the attached displays, so screenChoice can tell
// whether the remembered one is still there.
func activeDisplays() []uint32 {
	var ids [maxDisplays]C.uint32_t
	count := int(C.listDisplays(&ids[0], maxDisplays))

	displays := make([]uint32, 0, count)
	for _, id := range ids[:count] {
		displays = append(displays, uint32(id))
	}

	return displays
}

// handleReopen makes a launch of the running app call show. Called from
// startup, like registerToggleHotkeys.
func handleReopen(show func()) {
	onReopen = show
	C.installReopenHandler()
}

// activateApp brings the launcher to the front so the shown window has focus.
func activateApp() {
	C.activateApp()
}
