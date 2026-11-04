package native

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

// Where statusBarAdd puts the next entry: the menu itself, or the
// submenu being filled between statusBarBeginSubmenu and
// statusBarEndSubmenu. Only ever touched on the main queue.
static NSMenu *statusBarCurrent = nil;

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

// The system's grid symbol, drawn as a template so it follows light and
// dark menu bars. The symbol API is newer than the 10.13 deployment
// target, so a Mac on 10.13-10.15 gets nil here and the word instead,
// not a crash.
static NSImage *statusBarSymbol(void) {
	if (@available(macOS 11.0, *)) {
		NSImage *image = [NSImage imageWithSystemSymbolName:@"square.grid.2x2" accessibilityDescription:@"hopto"];
		image.template = YES;

		return image;
	}

	return nil;
}

// Shows an image on the item, or the word hopto when there is none.
static void statusBarShow(NSImage *image) {
	if (image != nil) {
		statusBarItem.button.image = image;
		statusBarItem.button.title = @"";
	} else {
		statusBarItem.button.image = nil;
		statusBarItem.button.title = @"hopto";
	}
}

// Creates the item with the system symbol; the launcher puts its own
// drawing on it with statusBarSetIcon right after. A second install
// only empties the menu.
static void statusBarInstall(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (statusBarItem != nil) {
			[statusBarMenu removeAllItems];
			statusBarCurrent = statusBarMenu;
			return;
		}

		statusBarTarget = statusBarNewTarget();
		statusBarItem = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength] retain];
		statusBarShow(statusBarSymbol());

		statusBarMenu = [[NSMenu alloc] initWithTitle:@""];
		statusBarMenu.autoenablesItems = NO;
		statusBarItem.menu = statusBarMenu;
		statusBarCurrent = statusBarMenu;
	});
}

// A drawing of the launcher's own: two PNGs of the same picture, 18 and
// 36 pixels, as one image of 18 points, so a Retina screen takes the
// large one. A template is painted by the system in the colour of the
// bar or the menu, like its own symbols. Empty data on both sides, the
// way Go says "the system symbol", gives that symbol; data that is not
// a picture gives it too. Autoreleased, for the main queue.
static NSImage *statusBarImage(NSData *smallData, NSData *largeData, int template) {
	if (smallData.length == 0 && largeData.length == 0) {
		return statusBarSymbol();
	}

	NSImage *image = [[[NSImage alloc] initWithSize:NSMakeSize(18, 18)] autorelease];

	for (NSData *data in @[smallData, largeData]) {
		if (data.length == 0) {
			continue;
		}

		NSBitmapImageRep *rep = [NSBitmapImageRep imageRepWithData:data];
		if (rep != nil) {
			// The point size, not the pixel size: this is what makes
			// the 36 pixel rep the 2x one.
			rep.size = NSMakeSize(18, 18);
			[image addRepresentation:rep];
		}
	}

	if (image.representations.count == 0) {
		return statusBarSymbol();
	}

	image.template = template != 0;
	image.accessibilityDescription = @"hopto";

	return image;
}

// Puts a drawing of the launcher's own on the item. The buffers come
// from Go's C.CString and are copied into NSData and freed here,
// synchronously, like the titles of statusBarAdd.
static void statusBarSetIcon(char *small, int smallLength, char *large, int largeLength, int template) {
	NSData *smallData = [[NSData alloc] initWithBytes:small length:smallLength];
	NSData *largeData = [[NSData alloc] initWithBytes:large length:largeLength];
	free(small);
	free(large);

	dispatch_async(dispatch_get_main_queue(), ^{
		statusBarShow(statusBarImage(smallData, largeData, template));

		[smallData release];
		[largeData release];
	});
}

// Puts the system symbol back on the item.
static void statusBarUseSymbol(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		statusBarShow(statusBarSymbol());
	});
}

// Opens a submenu: the entries added until statusBarEndSubmenu go into
// it. The title is freed here like in statusBarAdd.
static void statusBarBeginSubmenu(char *title) {
	NSString *text = [[NSString alloc] initWithUTF8String:title];
	free(title);

	dispatch_async(dispatch_get_main_queue(), ^{
		NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:text action:nil keyEquivalent:@""];
		NSMenu *submenu = [[NSMenu alloc] initWithTitle:text];
		submenu.autoenablesItems = NO;
		item.submenu = submenu;
		[statusBarMenu addItem:item];
		statusBarCurrent = submenu;

		[submenu release];
		[item release];
		[text release];
	});
}

// Closes the submenu: the next entries go into the menu itself again.
static void statusBarEndSubmenu(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		statusBarCurrent = statusBarMenu;
	});
}

// Adds one entry, or a rule for tag 0. The title is copied into an
// NSString synchronously, then freed here: Go's C.CString allocates it
// with malloc, and the block below only ever touches the NSString. The
// same for the drawing next to the label, when hasIcon says there is
// one; with it, the entry's image is the drawing or the system symbol.
static void statusBarAdd(char *title, int tag, int checked, int hasIcon, char *small, int smallLength, char *large, int largeLength, int template) {
	NSString *text = [[NSString alloc] initWithUTF8String:title];
	NSData *smallData = [[NSData alloc] initWithBytes:small length:smallLength];
	NSData *largeData = [[NSData alloc] initWithBytes:large length:largeLength];
	free(title);
	free(small);
	free(large);

	dispatch_async(dispatch_get_main_queue(), ^{
		if (tag == 0) {
			[statusBarCurrent addItem:[NSMenuItem separatorItem]];
		} else {
			NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:text action:@selector(picked:) keyEquivalent:@""];
			item.target = statusBarTarget;
			item.tag = tag;
			item.state = checked ? NSControlStateValueOn : NSControlStateValueOff;
			if (hasIcon) {
				item.image = statusBarImage(smallData, largeData, template);
			}
			[statusBarCurrent addItem:item];
			[item release];
		}

		[text release];
		[smallData release];
		[largeData release];
	});
}

// Finds the entry with a tag in a menu or in any of its submenus;
// itemWithTag: only looks at one level.
static NSMenuItem *statusBarFind(NSMenu *menu, int tag) {
	for (NSMenuItem *item in menu.itemArray) {
		if (item.tag == tag) {
			return item;
		}

		if (item.submenu != nil) {
			NSMenuItem *found = statusBarFind(item.submenu, tag);
			if (found != nil) {
				return found;
			}
		}
	}

	return nil;
}

// Ticks or unticks one entry, wherever it is.
static void statusBarCheck(int tag, int checked) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSMenuItem *item = statusBarFind(statusBarMenu, tag);
		item.state = checked ? NSControlStateValueOn : NSControlStateValueOff;
	});
}
*/
import "C"

// StatusBar is the real menu bar item.
type StatusBar struct{}

// Install creates the item (once) and fills its menu; the blocks run on
// the main queue in the order they were queued.
func (StatusBar) Install(items []MenuItem) {
	C.statusBarInstall()
	addEntries(items)
}

// addEntries queues one statusBarAdd per entry; one with children opens
// a submenu first and closes it after them.
func addEntries(items []MenuItem) {
	for _, item := range items {
		// statusBarBeginSubmenu and statusBarAdd free the title
		// themselves, once they have built the NSString they need.
		if len(item.Children) > 0 {
			C.statusBarBeginSubmenu(C.CString(item.Label))
			addEntries(item.Children)
			C.statusBarEndSubmenu()

			continue
		}

		// statusBarAdd draws a rule for tag 0, so a separator goes
		// over as that whatever its Tag says.
		tag := item.Tag
		if item.Separator {
			tag = 0
		}

		// An entry without a drawing sends empty buffers it ignores.
		var icon Icon
		if item.Icon != nil {
			icon = *item.Icon
		}

		C.statusBarAdd(
			C.CString(item.Label), C.int(tag), boolToInt(item.Checked),
			boolToInt(item.Icon != nil),
			C.CString(string(icon.Small)), C.int(len(icon.Small)),
			C.CString(string(icon.Large)), C.int(len(icon.Large)),
			boolToInt(icon.Template),
		)
	}
}

// SetIcon puts a drawing on the item, or the system symbol back for an
// empty one. The PNGs go over as C copies, which statusBarSetIcon frees
// once it has its NSData. C.CString and not C.CBytes: it copies every
// byte all the same, zero ones included, with the length alongside,
// and it keeps the unsafe package out of this file, where the linter
// takes it for a second import of C.
func (StatusBar) SetIcon(icon Icon) {
	if icon.Empty() {
		C.statusBarUseSymbol()
		return
	}

	C.statusBarSetIcon(
		C.CString(string(icon.Small)), C.int(len(icon.Small)),
		C.CString(string(icon.Large)), C.int(len(icon.Large)),
		boolToInt(icon.Template),
	)
}

// SetChecked moves the tick of one entry.
func (StatusBar) SetChecked(tag int, on bool) {
	C.statusBarCheck(C.int(tag), boolToInt(on))
}

// boolToInt is the C side's idea of a boolean.
func boolToInt(on bool) C.int {
	if on {
		return 1
	}

	return 0
}

// hoptoMenuPicked is called by the click handler of every entry, on the
// main thread, with the tag of the one chosen.
//
//export hoptoMenuPicked
func hoptoMenuPicked(tag C.int) {
	menuPicked(int(tag))
}
