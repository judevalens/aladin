import AppKit

// Regenerates the dev flavour's app icon from the prod one, so the two builds are
// distinguishable on the home screen without maintaining two drawings. The background
// colour is swapped and every pixel is treated as a blend between that base and white,
// so antialiased edges survive. Re-run it after replacing the prod icon:
//
//   cd anchor/iosApp/iosApp/Assets.xcassets
//   xcrun swift ../../tools/recolor-icon.swift \
//     AppIcon.appiconset/app-icon-1024.png \
//     AppIconDev.appiconset/app-icon-dev-1024.png c9925a   # c9925a = the Aladin amber
//
// Args: in.png out.png RRGGBB
let args = CommandLine.arguments
let src = NSBitmapImageRep(data: try! Data(contentsOf: URL(fileURLWithPath: args[1])))!
let hex = UInt32(args[3], radix: 16)!
let tr = Double((hex >> 16) & 0xff), tg = Double((hex >> 8) & 0xff), tb = Double(hex & 0xff)

let w = src.pixelsWide, h = src.pixelsHigh
let out = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: w, pixelsHigh: h,
                           bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
                           colorSpaceName: .deviceRGB, bytesPerRow: w * 4, bitsPerPixel: 32)!
let base = src.colorAt(x: 0, y: 0)!.usingColorSpace(.deviceRGB)!
let br = base.redComponent * 255, bg = base.greenComponent * 255, bb = base.blueComponent * 255

for y in 0..<h {
    for x in 0..<w {
        let c = src.colorAt(x: x, y: y)!.usingColorSpace(.deviceRGB)!
        let r = c.redComponent * 255, g = c.greenComponent * 255, b = c.blueComponent * 255
        // How far this pixel sits between the base colour and white, summed over all
        // three channels so a pure-white pixel lands exactly at 1.
        let denom = (255 - br) + (255 - bg) + (255 - bb)
        let t = min(1.0, max(0.0, ((r - br) + (g - bg) + (b - bb)) / denom))
        out.setColor(NSColor(deviceRed: (tr + (255 - tr) * t) / 255,
                             green: (tg + (255 - tg) * t) / 255,
                             blue: (tb + (255 - tb) * t) / 255,
                             alpha: c.alphaComponent), atX: x, y: y)
    }
}
try! out.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: args[2]))
print("wrote \(args[2])")
