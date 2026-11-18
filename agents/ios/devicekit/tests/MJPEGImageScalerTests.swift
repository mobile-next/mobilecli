import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

// Run from the repository root on macOS:
// swiftc DeviceKitTests/Streamer/MJPEG/MJPEGImageScaler.swift tests/MJPEGImageScalerTests.swift -o /tmp/mjpeg-tests
// /tmp/mjpeg-tests
//
// Standalone macOS regression test: checks the decoded raster, ignoring EXIF,
// just like the desktop Go JPEG decoder. No device or XCTest runner is needed.
@main
enum MJPEGImageScalerTests {
    static func require(_ condition: Bool, _ message: String) {
        if !condition { fatalError(message) }
    }

    static func properties(_ data: Data) -> [CFString: Any] {
        let source = CGImageSourceCreateWithData(data as CFData, nil)!
        return CGImageSourceCopyPropertiesAtIndex(source, 0, nil) as! [CFString: Any]
    }

    static func main() throws {
        let width = 80, height = 160
        let colors: [[UInt8]] = [[255, 0, 0], [0, 255, 0], [0, 0, 255], [255, 255, 0]]
        var pixels = [UInt8](repeating: 255, count: width * height * 4)
        for y in 0..<height {
            for x in 0..<width {
                let color = colors[(y < height / 2 ? 0 : 2) + (x < width / 2 ? 0 : 1)]
                for c in 0..<3 { pixels[(y * width + x) * 4 + c] = color[c] }
            }
        }
        let bitmapInfo = CGBitmapInfo(rawValue: CGImageAlphaInfo.noneSkipLast.rawValue)
            .union(.byteOrder32Big)
        let input = CGImage(
            width: width, height: height, bitsPerComponent: 8, bitsPerPixel: 32,
            bytesPerRow: width * 4, space: CGColorSpaceCreateDeviceRGB(),
            bitmapInfo: bitmapInfo,
            provider: CGDataProvider(data: Data(pixels) as CFData)!,
            decode: nil, shouldInterpolate: false, intent: .defaultIntent
        )!
        let expected = [
            [0, 1, 2, 3], [1, 0, 3, 2], [3, 2, 1, 0], [2, 3, 0, 1],
            [0, 2, 1, 3], [2, 0, 3, 1], [3, 1, 2, 0], [1, 3, 0, 2]
        ]
        let scaler = MJPEGImageScaler()
        var uprightJPEG = Data()
        for orientation in 1...8 {
            let encoded = NSMutableData()
            let destination = CGImageDestinationCreateWithData(
                encoded, UTType.jpeg.identifier as CFString, 1, nil
            )!
            CGImageDestinationAddImage(destination, input, [
                kCGImagePropertyOrientation: orientation,
                kCGImageDestinationLossyCompressionQuality: 1
            ] as CFDictionary)
            require(CGImageDestinationFinalize(destination), "encode input")
            if orientation == 1 { uprightJPEG = encoded as Data }
            for scale: CGFloat in [0.125, 0.3, 0.5, 0.625, 1] {
                let result = scaler.scaleJPEG(encoded as Data, scaleFactor: scale, quality: 0.9)!
                let source = CGImageSourceCreateWithData(result as CFData, nil)!
                let image = CGImageSourceCreateImageAtIndex(source, 0, nil)!
                let swapped = orientation >= 5
                let label = "orientation=\(orientation), scale=\(scale)"
                require(image.width == Int(CGFloat(swapped ? height : width) * scale), "width: \(label)")
                require(image.height == Int(CGFloat(swapped ? width : height) * scale), "height: \(label)")
                require((properties(result)[kCGImagePropertyOrientation] as? NSNumber)?.intValue == 1,
                        "orientation must be normalized: \(label)")
                let context = CGContext(
                    data: nil, width: image.width, height: image.height,
                    bitsPerComponent: 8, bytesPerRow: image.width * 4,
                    space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: bitmapInfo.rawValue
                )!
                context.draw(image, in: CGRect(x: 0, y: 0, width: image.width, height: image.height))
                let decoded = context.data!.assumingMemoryBound(to: UInt8.self)
                for quadrant in 0..<4 {
                    let x = image.width * (quadrant % 2 == 0 ? 1 : 3) / 4
                    let y = image.height * (quadrant < 2 ? 1 : 3) / 4
                    let offset = (y * image.width + x) * 4
                    let wanted = colors[expected[orientation - 1][quadrant]]
                    for channel in 0..<3 {
                        require(abs(Int(decoded[offset + channel]) - Int(wanted[channel])) < 40,
                                "pixel quadrant=\(quadrant): \(label)")
                    }
                }
                if scale == 1 && orientation == 1 {
                    require(result == encoded as Data, "upright full size must bypass recompression")
                }
            }
        }
        require(scaler.scaleJPEG(Data("invalid".utf8), scaleFactor: 0.5, quality: 0.5) == nil,
                "invalid JPEG must fail")
        for scale: CGFloat in [0, -1, 2, .infinity, .nan] {
            require(scaler.scaleJPEG(uprightJPEG, scaleFactor: scale, quality: 0.5) == nil,
                    "invalid scale must fail")
        }
        print("PASS: all 8 EXIF orientations at multiple scales; pixels, dimensions, invalid JPEG/scale")
    }
}
