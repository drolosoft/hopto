#!/usr/bin/env osascript -l JavaScript
// Draws a 128 px icon for a link that has no usable icon of its own: an SF
// Symbol in white, or a PNG logo, over a rounded square of one colour, in the
// style of the app icons of this repo. A dark logo on a transparent
// background becomes visible again on a white tile.
// Usage:
//   osascript -l JavaScript scripts/draw-icon.js <symbol|logo.png> <rrggbb> <out.png>
ObjC.import('AppKit');

function run(argv) {
    const symbolName = argv[0];
    const hex = argv[1];
    const outPath = argv[2];
    const size = 128;

    const red = parseInt(hex.slice(0, 2), 16) / 255;
    const green = parseInt(hex.slice(2, 4), 16) / 255;
    const blue = parseInt(hex.slice(4, 6), 16) / 255;

    const image = $.NSImage.alloc.initWithSize($.NSMakeSize(size, size));
    image.lockFocus;

    // The rounded square, with the corner radius macOS uses for app icons.
    const background = $.NSBezierPath.bezierPathWithRoundedRectXRadiusYRadius(
        $.NSMakeRect(0, 0, size, size), size * 0.22, size * 0.22);
    $.NSColor.colorWithSRGBRedGreenBlueAlpha(red, green, blue, 1).setFill;
    background.fill;

    if (symbolName.endsWith('.png')) {
        // A logo file: fitted inside the tile with some margin.
        const logo = $.NSImage.alloc.initWithContentsOfFile(symbolName);
        const margin = size * 0.15;
        logo.drawInRectFromRectOperationFraction($.NSMakeRect(margin, margin, size - 2 * margin, size - 2 * margin), $.NSZeroRect, $.NSCompositingOperationSourceOver, 1);
    } else {
        // An SF Symbol, centred, at about half the icon, coloured white
        // through a hierarchical-colour configuration (template tinting does
        // not survive drawInRect on a locked-focus image).
        const sizing = $.NSImageSymbolConfiguration.configurationWithPointSizeWeight(size * 0.5, $.NSFontWeightMedium);
        const colouring = $.NSImageSymbolConfiguration.configurationWithHierarchicalColor($.NSColor.whiteColor);
        let symbol = $.NSImage.imageWithSystemSymbolNameAccessibilityDescription(symbolName, $());
        symbol = symbol.imageWithSymbolConfiguration(sizing.configurationByApplyingConfiguration(colouring));

        const symbolSize = symbol.size;
        const rect = $.NSMakeRect((size - symbolSize.width) / 2, (size - symbolSize.height) / 2, symbolSize.width, symbolSize.height);
        symbol.drawInRectFromRectOperationFraction(rect, $.NSZeroRect, $.NSCompositingOperationSourceOver, 1);
    }

    image.unlockFocus;

    const tiff = image.TIFFRepresentation;
    const bitmap = $.NSBitmapImageRep.imageRepWithData(tiff);
    const png = bitmap.representationUsingTypeProperties($.NSBitmapImageFileTypePNG, $());
    png.writeToFileAtomically(outPath, true);
    return outPath;
}
