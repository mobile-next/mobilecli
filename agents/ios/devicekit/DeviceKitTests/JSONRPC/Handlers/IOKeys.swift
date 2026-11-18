import XCTest
import os

struct IOKeysCombo: Codable {
    let key: String
    let modifiers: [String]
}

struct IOKeysRequest: Codable {
    let keys: [IOKeysCombo]
}

private struct ResolvedKeyCombo {
    let key: String
    let flags: XCUIElement.KeyModifierFlags
    let preferTypeText: Bool
}

private let modifierFlagsByName: [String: XCUIElement.KeyModifierFlags] = [
    "command": .command,
    "control": .control,
    "option": .option,
    "shift": .shift,
    "fn": .function,
]

private let keyboardKeysByName: [String: XCUIKeyboardKey] = [
    "return": .return,
    "enter": .enter,
    "backspace": .delete,
    "delete": .delete,
    "forwarddelete": .forwardDelete,
    "tab": .tab,
    "space": .space,
    "escape": .escape,
    "up": .upArrow,
    "down": .downArrow,
    "left": .leftArrow,
    "right": .rightArrow,
    "home": .home,
    "end": .end,
    "pageup": .pageUp,
    "pagedown": .pageDown,
    "f1": .F1, "f2": .F2, "f3": .F3, "f4": .F4,
    "f5": .F5, "f6": .F6, "f7": .F7, "f8": .F8,
    "f9": .F9, "f10": .F10, "f11": .F11, "f12": .F12,
]

// HID "Keyboard Clear" (page 0x07, usage 0x9C) wipes the focused text field in one event.
// Sent as a device event, not through typeKey, which has no key for it.
private let hidClearKeyName = "clear"
private let hidKeyboardPage: UInt32 = 0x07
private let hidKeyboardClearUsage: UInt32 = 0x9C

// Named keys whose XCUIKeyboardKey rawValue is a real control character that
// typeText delivers correctly without a modifier. Every other named key (arrows,
// home/end, page nav, F-keys) carries a scalar that typeText would insert as a
// literal glyph, so it must be sent through typeKey instead.
private let typeTextKeyNames: Set<String> = [
    "return", "enter", "backspace", "delete", "forwarddelete",
    "tab", "space", "escape",
]

@MainActor
struct IOKeysMethodHandler: RPCMethodHandler {
    static let methodName = "device.io.keys"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(IOKeysRequest.self, from: params)
        guard !request.keys.isEmpty else {
            throw RPCMethodError.invalidParams("At least one key is required")
        }

        // resolve all combos upfront, so an invalid combo fails before any
        // key is pressed
        let resolvedCombos = try request.keys.map { combo -> ResolvedKeyCombo in
            if combo.key.lowercased() == hidClearKeyName && combo.modifiers.isEmpty {
                return ResolvedKeyCombo(key: hidClearKeyName, flags: [], preferTypeText: false)
            }
            let resolved = try resolveKey(combo.key)
            return ResolvedKeyCombo(
                key: resolved.value,
                flags: try resolveModifiers(combo.modifiers),
                preferTypeText: resolved.preferTypeText
            )
        }

        let start = Date()
        logger.info("[Start] Typing \(request.keys.count) keys")

        let app = RunningApp.getForegroundApp() ?? XCUIApplication(bundleIdentifier: RunningApp.springboardBundleId)
        for combo in resolvedCombos {
            // typeKey delivers modifier combos and navigation/function keys;
            // typeText handles literal characters and control-character keys,
            // which typeKey does not deliver reliably without a modifier.
            if combo.key == hidClearKeyName {
                try sendHIDEvent(page: hidKeyboardPage, usage: hidKeyboardClearUsage)
            } else if combo.flags.isEmpty && combo.preferTypeText {
                app.typeText(combo.key)
            } else {
                app.typeKey(combo.key, modifierFlags: combo.flags)
            }
        }

        let duration = Date().timeIntervalSince(start)
        logger.info("[Done] Typing keys took \(duration)")
        return .object(["success": .bool(true)])
    }

    private func resolveKey(_ key: String) throws -> (value: String, preferTypeText: Bool) {
        let lowered = key.lowercased()
        if let namedKey = keyboardKeysByName[lowered] {
            return (namedKey.rawValue, typeTextKeyNames.contains(lowered))
        }

        guard key.count == 1 else {
            throw RPCMethodError.invalidParams("Unsupported key: \(key)")
        }

        return (key, true)
    }

    private func resolveModifiers(_ modifiers: [String]) throws -> XCUIElement.KeyModifierFlags {
        var flags: XCUIElement.KeyModifierFlags = []
        for modifier in modifiers {
            guard let flag = modifierFlagsByName[modifier.lowercased()] else {
                throw RPCMethodError.invalidParams("Unsupported modifier: \(modifier)")
            }
            flags.insert(flag)
        }

        return flags
    }
}
