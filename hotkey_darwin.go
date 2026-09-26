package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Carbon -framework Cocoa

#include <Carbon/Carbon.h>
#include <Cocoa/Cocoa.h>

// Implemented in Go below; Carbon calls the C handler, which calls these.
extern void launcherHotkeyPressed(UInt32 id);
extern void launcherHotkeyRegistered(UInt32 id, OSStatus status);

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

// Centres the window on the screen under the mouse, a little above the
// middle so it reads as an overlay, and makes sure the window behaves as
// one: present in every Space (with "displays have separate Spaces" a
// window bound to one Space made macOS switch Spaces, so the launcher
// seemed to jump to the other display), allowed over full-screen apps,
// with a shadow. Synchronous so the window is in place before it is shown;
// the caller is never the main thread, otherwise this would deadlock.
static void centerOnActiveScreen(void) {
	dispatch_sync(dispatch_get_main_queue(), ^{
		NSWindow *window = [NSApp windows].firstObject;
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

		NSRect screen = screenUnderMouse().visibleFrame;
		NSRect frame = window.frame;

		frame.origin.x = screen.origin.x + (screen.size.width - frame.size.width) / 2;
		frame.origin.y = screen.origin.y + (screen.size.height - frame.size.height) / 2 + screen.size.height * 0.08;
		[window setFrameOrigin:frame.origin];
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

// Virtual key code of the space bar and the modifiers, from Carbon's
// Events.h. Cmd+Shift+Space opens the apps tab: it is free in a stock macOS
// (Option+Space belongs to Alfred or Raycast, Cmd+Space to Spotlight).
// Cmd+Option+Space opens the links tab; macOS assigns it to "Show Finder
// search window", which has to be disabled in System Settings for the
// launcher to receive it.
const (
	keySpace          = 49
	modifierCmdShift  = C.cmdKey | C.shiftKey
	modifierCmdOption = C.cmdKey | C.optionKey
)

// The hot key ids handed to Carbon; they come back in the event so the
// handler knows which tab to open.
const (
	hotkeyApps  = 1
	hotkeyLinks = 2
)

// onHotkey is what the Carbon handler calls, with the tab of the pressed
// shortcut. It is a variable so the App can install its own toggle without
// the C side knowing about it.
var onHotkey = func(tab string) {}

//export launcherHotkeyPressed
func launcherHotkeyPressed(id C.UInt32) {
	tab := tabApps
	if id == hotkeyLinks {
		tab = tabLinks
	}

	log.Printf("hotkey %d pressed: %s", id, tab)

	// The handler runs on the main thread; the Wails runtime calls are
	// dispatched from a goroutine so they never block that thread.
	go onHotkey(tab)
}

//export launcherHotkeyRegistered
func launcherHotkeyRegistered(id C.UInt32, status C.OSStatus) {
	if status != 0 {
		log.Printf("hotkey %d: RegisterEventHotKey failed with status %d", id, status)
		return
	}

	log.Printf("hotkey %d registered", id)
}

// becomeAccessory removes the launcher from the Dock and from Cmd+Tab once
// the Wails main loop is up. Called from startup.
func becomeAccessory() {
	C.becomeAccessory()
}

// registerToggleHotkeys binds Cmd+Shift+Space to the apps tab and
// Cmd+Option+Space to the links tab.
func registerToggleHotkeys(toggle func(tab string)) {
	onHotkey = toggle
	C.registerHotkey(hotkeyApps, C.UInt32(keySpace), C.UInt32(modifierCmdShift))
	C.registerHotkey(hotkeyLinks, C.UInt32(keySpace), C.UInt32(modifierCmdOption))
}

// centerOnActiveScreen moves the hidden window to the middle of the screen
// under the mouse and sets the overlay window behaviour. Call it from a
// goroutine, never from the main thread.
func centerOnActiveScreen() {
	C.centerOnActiveScreen()
}

// activateApp brings the launcher to the front so the shown window has focus.
func activateApp() {
	C.activateApp()
}
