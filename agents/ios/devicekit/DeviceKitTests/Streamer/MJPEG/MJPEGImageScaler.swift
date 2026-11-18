import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

final class MJPEGImageScaler {
    func scaleJPEG(_ data: Data, scaleFactor: CGFloat, quality: CGFloat) -> Data? {
        guard scaleFactor.isFinite, scaleFactor > 0, scaleFactor <= 1,
              let imageSource = CGImageSourceCreateWithData(data as CFData, nil),
              let properties = CGImageSourceCopyPropertiesAtIndex(imageSource, 0, nil) as? [CFString: Any],
              let width = properties[kCGImagePropertyPixelWidth] as? NSNumber,
              let height = properties[kCGImagePropertyPixelHeight] as? NSNumber else {
            return nil
        }

        let orientation = (properties[kCGImagePropertyOrientation] as? NSNumber)?.intValue ?? 1
        if scaleFactor == 1, orientation == 1 {
            return data
        }

        let maxPixelSize = Int(CGFloat(max(width.intValue, height.intValue)) * scaleFactor)
        guard maxPixelSize > 0 else {
            return nil
        }

        // XCTest may encode portrait pixels with EXIF orientation even in landscape.
        // Apply that orientation while scaling, so every JPEG has upright pixels and
        // its actual width/height describe the display. Do not rely on client EXIF support.
        let thumbnailOptions: [CFString: Any] = [
            kCGImageSourceCreateThumbnailFromImageAlways: true,
            kCGImageSourceCreateThumbnailWithTransform: true,
            kCGImageSourceThumbnailMaxPixelSize: maxPixelSize
        ]
        guard let scaledImage = CGImageSourceCreateThumbnailAtIndex(imageSource, 0, thumbnailOptions as CFDictionary) else {
            return nil
        }

        let mutableData = NSMutableData()
        guard let destination = CGImageDestinationCreateWithData(
            mutableData as CFMutableData,
            UTType.jpeg.identifier as CFString,
            1,
            nil
        ) else {
            return nil
        }

        let options: [CFString: Any] = [
            kCGImageDestinationLossyCompressionQuality: quality,
            kCGImagePropertyOrientation: 1
        ]
        CGImageDestinationAddImage(destination, scaledImage, options as CFDictionary)
        guard CGImageDestinationFinalize(destination) else {
            return nil
        }
        return mutableData as Data
    }
}
