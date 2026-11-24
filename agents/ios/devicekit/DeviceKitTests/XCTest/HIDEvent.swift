import Foundation
import XCTest

// Sends a raw HID usage (e.g. page 0x07 keyboard, page 0x0C consumer) through
// XCUIDevice's private device-event API. The selectors take primitive
// arguments, so they're called through their IMPs instead of perform(_:).
func sendHIDEvent(page: UInt32, usage: UInt32, duration: Double = 0.01) throws {
    typealias DeviceEventFactory = @convention(c) (AnyClass, Selector, UInt32, UInt32, Double) -> AnyObject?
    let selector = NSSelectorFromString("deviceEventWithPage:usage:duration:")
    guard let eventClass = NSClassFromString("XCDeviceEvent"),
          let method = class_getClassMethod(eventClass, selector) else {
        throw RPCMethodError.internalError("XCDeviceEvent is not available")
    }
    let factory = unsafeBitCast(method_getImplementation(method), to: DeviceEventFactory.self)
    guard let event = factory(eventClass, selector, page, usage, duration) else {
        throw RPCMethodError.internalError("Failed to create HID event (page \(page), usage \(usage))")
    }
    typealias PerformDeviceEvent = @convention(c) (AnyObject, Selector, AnyObject, UnsafeMutablePointer<AnyObject?>?) -> Bool
    let performSelector = NSSelectorFromString("performDeviceEvent:error:")
    guard let performMethod = class_getInstanceMethod(XCUIDevice.self, performSelector) else {
        throw RPCMethodError.internalError("performDeviceEvent is not available")
    }
    let perform = unsafeBitCast(method_getImplementation(performMethod), to: PerformDeviceEvent.self)
    var error: AnyObject?
    if !perform(XCUIDevice.shared, performSelector, event, &error) {
        throw RPCMethodError.internalError("Failed to send HID event (page \(page), usage \(usage)): \(String(describing: error))")
    }
}
