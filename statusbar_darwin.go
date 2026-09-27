package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#include <Cocoa/Cocoa.h>
#include <objc/runtime.h>
#include <stdlib.h>

// Implemented in Go below.
extern void hoptoMenuPicked(int tag);

// The item, its menu and the object that receives the clicks. They live
// for the whole run, so they are retained once and never released.
static NSStatusItem *statusBarItem = nil;
static NSMenu *statusBarMenu = nil;
static id statusBarTarget = nil;

// The click handler of every entry: the tag says which one.
static void statusBarPicked(id receiver, SEL command, NSMenuItem *item) {
	hoptoMenuPicked((int)item.tag);
}

// The target class is made at run time rather than with @implementation:
// this preamble is copied into two C files by cgo because of the export
// above, and a class defined twice would not link.
static id statusBarNewTarget(void) {
	Class targetClass = objc_allocateClassPair([NSObject class], "HoptoMenuTarget", 0);
	class_addMethod(targetClass, @selector(picked:), (IMP)statusBarPicked, "v@:@");
	objc_registerClassPair(targetClass);

	return [[targetClass alloc] init];
}

// Creates the item with a template symbol (it follows light and dark
// menu bars), or the word hopto on a system without the symbol.
static void statusBarInstall(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (statusBarItem != nil) {
			[statusBarMenu removeAllItems];
			return;
		}

		statusBarTarget = statusBarNewTarget();
		statusBarItem = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength] retain];

		// The symbol API is newer than the 10.13 deployment target, so a
		// Mac on 10.13-10.15 must take the word instead, not crash.
		if (@available(macOS 11.0, *)) {
			NSImage *image = [NSImage imageWithSystemSymbolName:@"square.grid.2x2" accessibilityDescription:@"hopto"];
			if (image != nil) {
				image.template = YES;
				statusBarItem.button.image = image;
			} else {
				statusBarItem.button.title = @"hopto";
			}
		} else {
			statusBarItem.button.title = @"hopto";
		}

		statusBarMenu = [[NSMenu alloc] initWithTitle:@""];
		statusBarMenu.autoenablesItems = NO;
		statusBarItem.menu = statusBarMenu;
	});
}

// Adds one entry, or a rule for tag 0. The title is copied into an
// NSString synchronously, then freed here: Go's C.CString allocates it
// with malloc, and the block below only ever touches the NSString.
static void statusBarAdd(char *title, int tag, int checked) {
	NSString *text = [[NSString alloc] initWithUTF8String:title];
	free(title);

	dispatch_async(dispatch_get_main_queue(), ^{
		if (tag == 0) {
			[statusBarMenu addItem:[NSMenuItem separatorItem]];
		} else {
			NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:text action:@selector(picked:) keyEquivalent:@""];
			item.target = statusBarTarget;
			item.tag = tag;
			item.state = checked ? NSControlStateValueOn : NSControlStateValueOff;
			[statusBarMenu addItem:item];
			[item release];
		}

		[text release];
	});
}

// Ticks or unticks one entry.
static void statusBarCheck(int tag, int checked) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSMenuItem *item = [statusBarMenu itemWithTag:tag];
		item.state = checked ? NSControlStateValueOn : NSControlStateValueOff;
	});
}
*/
import "C"

import "log"

// statusBar is the real menu bar item.
type statusBar struct{}

// Install creates the item (once) and fills its menu; the blocks run on
// the main queue in the order they were queued.
func (statusBar) Install(items []menuItem) {
	C.statusBarInstall()

	for _, item := range items {
		// statusBarAdd frees this string itself, once it has built the
		// NSString it needs.
		title := C.CString(item.Title)
		C.statusBarAdd(title, C.int(item.Tag), boolToInt(item.Checked))
	}
}

// SetChecked moves the tick of one entry.
func (statusBar) SetChecked(tag int, on bool) {
	C.statusBarCheck(C.int(tag), boolToInt(on))
}

// boolToInt is the C side's idea of a boolean.
func boolToInt(on bool) C.int {
	if on {
		return 1
	}

	return 0
}

//export hoptoMenuPicked
func hoptoMenuPicked(tag C.int) {
	log.Printf("menu %d picked", tag)

	// The click arrives on the main thread; the Wails runtime is only
	// ever called from a goroutine, as for the hotkeys.
	go onMenu(int(tag))
}
