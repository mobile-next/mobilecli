import FlyingFox
import FlyingSocks
import Foundation
import os
import XCTest
import CoreGraphics
import ImageIO

private enum MJPEGConstants {
    static let defaultFPS: Int = 10
    static let maxFPS: Int = 60
    static let defaultQuality: CGFloat = 0.25
    static let minQuality: CGFloat = 0.01
    static let maxQuality: CGFloat = 1.0
    static let defaultScale: Int = 100
    static let minScale: Int = 10
    static let maxScale: Int = 100
    static let serverName = "DeviceKit-iOS"
}

@MainActor
struct MJPEGHTTPHandler: HTTPHandler {
    private let boundary = "mjpeg-frame-boundary"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: "MJPEGStream"
    )

    func handleRequest(_ request: HTTPRequest) async throws -> HTTPResponse {
        let fps = request.queryInt(name: "fps", default: MJPEGConstants.defaultFPS, min: 1, max: MJPEGConstants.maxFPS)
        let qualityPercent = request.queryInt(name: "quality", default: 25, min: 1, max: 100)
        let scalePercent = request.queryInt(name: "scale", default: MJPEGConstants.defaultScale, min: MJPEGConstants.minScale, max: MJPEGConstants.maxScale)

        let quality = max(MJPEGConstants.minQuality, min(MJPEGConstants.maxQuality, CGFloat(qualityPercent) / 100.0))
        let scale = CGFloat(scalePercent) / 100.0

        logger.info("Starting MJPEG stream: fps=\(fps), quality=\(qualityPercent)%, scale=\(scalePercent)%")

        let stream = MJPEGByteStream(fps: fps, quality: quality, scale: scale)
        let bodySequence = HTTPBodySequence(from: stream)

        var headers: [HTTPHeader: String] = [:]
        headers[.contentType] = "multipart/x-mixed-replace; boundary=\(boundary)"
        headers[HTTPHeader("Server")] = MJPEGConstants.serverName
        headers[HTTPHeader("Connection")] = "close"
        headers[HTTPHeader("Cache-Control")] = "no-cache, no-store, must-revalidate"
        headers[HTTPHeader("Pragma")] = "no-cache"
        headers[HTTPHeader("Expires")] = "0"

        return HTTPResponse(
            statusCode: .ok,
            headers: headers,
            body: bodySequence
        )
    }

}

struct MJPEGByteStream: AsyncBufferedSequence, Sendable {
    typealias Element = UInt8

    let fps: Int
    let quality: CGFloat
    let scale: CGFloat

    init(fps: Int, quality: CGFloat, scale: CGFloat = 1.0) {
        self.fps = fps
        self.quality = quality
        self.scale = scale
    }

    func makeAsyncIterator() -> MJPEGByteIterator {
        MJPEGByteIterator(fps: fps, quality: quality, scale: scale)
    }
}

final class MJPEGByteIterator: AsyncBufferedIteratorProtocol, @unchecked Sendable {
    private let fps: Int
    private let quality: CGFloat
    private let scale: CGFloat

    private let frameProducer: MJPEGFrameProducer

    private var isCancelled: Bool = false

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier ?? "devicekit-ios",
        category: "MJPEGIterator"
    )

    init(fps: Int, quality: CGFloat, scale: CGFloat) {
        self.fps = fps
        self.quality = quality
        self.scale = scale

        self.frameProducer = MJPEGFrameProducer(imageScaler: MJPEGImageScaler(), frameInterval: UInt64(1_000_000_000 / max(1, fps)))
    }

    func next() async throws -> UInt8? {
        guard !isCancelled, !Task.isCancelled else { return nil }
        let buffer = try await nextBuffer(suggested: 1)
        return buffer?.first
    }

    func nextBuffer(suggested count: Int) async throws -> [UInt8]? {
        guard !isCancelled, !Task.isCancelled else {
            return nil
        }

        do {
            let frameData = try await frameProducer.captureFrame(quality: quality, scale: scale)
            return Array(frameData)
        } catch {
            logger.error("Frame capture error: \(error.localizedDescription)")
            isCancelled = true
            return nil
        }
    }

}

