import Darwin
import Foundation
import os

struct IOHingeSetRequest: Codable {
    let angle: Double
}

private typealias CreateVendorDefinedEvent = @convention(c) (
    CFAllocator?, UInt64, UInt32, UInt32, UInt32, UnsafePointer<UInt8>, CFIndex, UInt32
) -> Unmanaged<CFTypeRef>?
private typealias CreateEventSystemClient = @convention(c) (CFAllocator?, Int32, CFDictionary?) -> Unmanaged<CFTypeRef>?
private typealias DispatchEvent = @convention(c) (CFTypeRef, CFTypeRef) -> Void

/// Sets the hinge angle of a foldable simulator (e.g. iPhone Duo) by dispatching the same
/// vendor-defined HID event DeviceHub sends: 0 = folded, 180 = fully open.
@MainActor
struct IOHingeSetMethodHandler: RPCMethodHandler {
    static let methodName = "device.io.hinge.set"

    private static let hingeUsagePage: UInt32 = 0xFF61
    private static let hingeUsage: UInt32 = 0x5B
    private static let adminClientType: Int32 = 1

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(IOHingeSetRequest.self, from: params)
        guard (0...180).contains(request.angle) else {
            throw RPCMethodError.invalidParams("Invalid angle \(request.angle), must be between 0 and 180")
        }

        logger.info("Setting hinge angle to \(request.angle)")
        try dispatchHingeEvent(angle: request.angle)
        return .object(["success": .bool(true)])
    }

    private func dispatchHingeEvent(angle: Double) throws {
        guard let iokit = dlopen("/System/Library/Frameworks/IOKit.framework/IOKit", RTLD_NOW),
              let createEventSym = dlsym(iokit, "IOHIDEventCreateVendorDefinedEvent"),
              let createClientSym = dlsym(iokit, "IOHIDEventSystemClientCreateWithType"),
              let dispatchSym = dlsym(iokit, "IOHIDEventSystemClientDispatchEvent") else {
            throw RPCMethodError.internalError("IOHIDEvent APIs are not available")
        }
        let createEvent = unsafeBitCast(createEventSym, to: CreateVendorDefinedEvent.self)
        let createClient = unsafeBitCast(createClientSym, to: CreateEventSystemClient.self)
        let dispatch = unsafeBitCast(dispatchSym, to: DispatchEvent.self)

        let payload = HingePayload.serialize(angle: angle)
        let event = payload.withUnsafeBufferPointer { bytes in
            createEvent(nil, mach_absolute_time(), Self.hingeUsagePage, Self.hingeUsage, 0,
                        bytes.baseAddress!, bytes.count, 0)
        }
        guard let event = event?.takeRetainedValue(),
              let client = createClient(nil, Self.adminClientType, nil)?.takeRetainedValue() else {
            throw RPCMethodError.internalError("Failed to create hinge HID event")
        }
        dispatch(client, event)
    }
}

/// OSSerializeBinary dictionary: {value, provider, type, source}, as captured from DeviceHub.
private enum HingePayload {
    private static let signature: UInt32 = 0x0000_00D3
    private static let endOfCollection: UInt32 = 0x8000_0000
    private static let dictionaryType: UInt32 = 0x01
    private static let numberType: UInt32 = 0x04
    private static let symbolType: UInt32 = 0x08
    private static let stringType: UInt32 = 0x09
    private static let numberBits: UInt32 = 63

    static func serialize(angle: Double) -> [UInt8] {
        var out: [UInt8] = []
        append(&out, signature)
        append(&out, endOfCollection | dictionaryType << 24 | 4)
        appendSymbol(&out, "value")
        append(&out, numberType << 24 | numberBits)
        append(&out, angle.bitPattern)
        appendSymbol(&out, "provider")
        appendString(&out, "com.apple.Virtualization.VirtualMachines")
        appendSymbol(&out, "type")
        appendString(&out, "range")
        appendSymbol(&out, "source")
        appendString(&out, "hinge-slider-control", isLast: true)
        return out
    }

    private static func appendSymbol(_ out: inout [UInt8], _ key: String) {
        let bytes = Array(key.utf8) + [0]
        append(&out, symbolType << 24 | UInt32(bytes.count))
        appendPadded(&out, bytes)
    }

    private static func appendString(_ out: inout [UInt8], _ value: String, isLast: Bool = false) {
        let bytes = Array(value.utf8)
        append(&out, (isLast ? endOfCollection : 0) | stringType << 24 | UInt32(bytes.count))
        appendPadded(&out, bytes)
    }

    private static func appendPadded(_ out: inout [UInt8], _ bytes: [UInt8]) {
        out += bytes
        out += [UInt8](repeating: 0, count: (4 - bytes.count % 4) % 4)
    }

    private static func append<T: FixedWidthInteger>(_ out: inout [UInt8], _ value: T) {
        withUnsafeBytes(of: value.littleEndian) { out += $0 }
    }
}
