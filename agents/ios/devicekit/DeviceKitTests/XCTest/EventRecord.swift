// Portions of this file are derived from Maestro
// (https://github.com/mobile-dev-inc/maestro), licensed under the Apache License 2.0,
// and have been modified by Mobile Next HQ, Inc. See THIRD_PARTY_NOTICES.md.

import Foundation
import UIKit

@objc
final class EventRecord: NSObject {
    let eventRecord: NSObject

    static let defaultTapDuration = 0.1

    static let swipeSampleRate: TimeInterval = 60
    static let maxSwipeSamples = 240

    enum Style: String {
        case singleFinger = "Single-Finger Touch Action"
        case multiFinger = "Multi-Finger Touch Action"
    }

    init(orientation: UIInterfaceOrientation, style: Style = .singleFinger, displayID: UInt64? = nil) {
        let recordClass: AnyClass = objc_lookUpClass("XCSynthesizedEventRecord")!
        let displaySelector = NSSelectorFromString("initWithName:displayID:interfaceOrientation:")
        if let displayID, recordClass.instancesRespond(to: displaySelector) {
            let instance = recordClass.alloc() as! NSObject
            typealias Method = @convention(c) (NSObject, Selector, NSString, UInt64, Int) -> NSObject
            let method = unsafeBitCast(instance.method(for: displaySelector), to: Method.self)
            eventRecord = method(instance, displaySelector, style.rawValue as NSString, displayID, orientation.rawValue)
            return
        }

        eventRecord =
            (recordClass.alloc() as AnyObject)
            .perform(
                NSSelectorFromString("initWithName:interfaceOrientation:"),
                with: style.rawValue,
                with: orientation
            )
            .takeUnretainedValue() as! NSObject
    }

    func addPointerTouchEvent(at point: CGPoint, touchUpAfter: TimeInterval?)
        -> Self
    {
        var path = PointerEventPath.pathForTouch(at: point)
        path.offset += touchUpAfter ?? Self.defaultTapDuration
        path.liftUp()
        return add(path)
    }

    func addSwipeEvent(start: CGPoint, end: CGPoint, duration: TimeInterval) -> Self {
        var path = PointerEventPath.pathForTouch(at: start)
        path.offset += Self.defaultTapDuration

        let sampleCount = Self.swipeSampleCount(for: duration)
        let step = duration / TimeInterval(sampleCount)
        for sample in 1...sampleCount {
            let progress = CGFloat(sample) / CGFloat(sampleCount)
            path.offset += step
            path.moveTo(point: CGPoint(
                x: start.x + (end.x - start.x) * progress,
                y: start.y + (end.y - start.y) * progress
            ))
        }

        path.liftUp()
        return add(path)
    }

    private static func swipeSampleCount(for duration: TimeInterval) -> Int {
        let sampled = Int((duration * Self.swipeSampleRate).rounded())
        return min(max(sampled, 1), Self.maxSwipeSamples)
    }

    func add(_ path: PointerEventPath) -> Self {
        let selector = NSSelectorFromString("addPointerEventPath:")
        let imp = eventRecord.method(for: selector)
        typealias Method = @convention(c) (NSObject, Selector, NSObject) -> Void
        let method = unsafeBitCast(imp, to: Method.self)
        method(eventRecord, selector, path.path)
        return self
    }
}
