#!/usr/bin/env osascript -l JavaScript
// Composes a contact sheet of every PNG in a folder (eight per row, each
// with its file name underneath) on the dark grey of the launcher panel, so
// the link icons can be checked at a glance: black icons on dark, blurry
// favicons and wrong pictures show up here before they show up in the app.
// Usage:
//   osascript -l JavaScript scripts/icon-sheet.js frontend/src/assets/links /tmp/sheet.png
ObjC.import('AppKit');

function run(argv) {
    const folder = $.NSString.alloc.initWithUTF8String(argv[0]).stringByStandardizingPath.js;
    const outPath = argv[1];

    // One cell per icon: the 128 px image plus room for the label.
    const cell = 150;
    const columns = 8;

    const manager = $.NSFileManager.defaultManager;
    const names = ObjC.deepUnwrap(manager.contentsOfDirectoryAtPathError(folder, $()))
        .filter((name) => name.endsWith('.png'))
        .sort();
    const rows = Math.ceil(names.length / columns);

    const sheet = $.NSImage.alloc.initWithSize($.NSMakeSize(columns * cell, rows * cell));
    sheet.lockFocus;

    // The same dark grey as the launcher panel, so contrast is judged for real.
    $.NSColor.colorWithSRGBRedGreenBlueAlpha(0.12, 0.12, 0.12, 1).setFill;
    $.NSRectFill($.NSMakeRect(0, 0, columns * cell, rows * cell));

    const labelAttributes = $.NSDictionary.dictionaryWithObjectsForKeys(
        $([$.NSFont.systemFontOfSize(12), $.NSColor.whiteColor]),
        $([$.NSFontAttributeName, $.NSForegroundColorAttributeName]));

    names.forEach((name, index) => {
        // AppKit's origin is bottom-left, so the first row goes on top.
        const x = (index % columns) * cell;
        const y = (rows - 1 - Math.floor(index / columns)) * cell;

        const icon = $.NSImage.alloc.initWithContentsOfFile(folder + '/' + name);
        icon.drawInRectFromRectOperationFraction($.NSMakeRect(x + 11, y + 30, 128, 128), $.NSZeroRect, $.NSCompositingOperationSourceOver, 1);

        $(name.replace('.png', '')).drawAtPointWithAttributes($.NSMakePoint(x + 8, y + 8), labelAttributes);
    });

    sheet.unlockFocus;

    const bitmap = $.NSBitmapImageRep.imageRepWithData(sheet.TIFFRepresentation);
    bitmap.representationUsingTypeProperties($.NSBitmapImageFileTypePNG, $()).writeToFileAtomically(outPath, true);
    return outPath;
}
