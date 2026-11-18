import Foundation

enum JSONRPCId: Codable, Equatable {
    case string(String)
    case int(Int)

    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if let intValue = try? container.decode(Int.self) {
            self = .int(intValue)
        } else if let stringValue = try? container.decode(String.self) {
            self = .string(stringValue)
        } else {
            throw DecodingError.typeMismatch(
                JSONRPCId.self,
                DecodingError.Context(
                    codingPath: decoder.codingPath,
                    debugDescription: "Expected String or Int for JSON-RPC id"
                )
            )
        }
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        switch self {
        case .string(let value):
            try container.encode(value)
        case .int(let value):
            try container.encode(value)
        }
    }
}

struct JSONRPCRequest: Codable {
    let jsonrpc: String
    let method: String
    let params: JSONValue?
    let id: JSONRPCId?

    var isValid: Bool {
        jsonrpc == "2.0"
    }
}

struct JSONRPCResponse: Encodable {
    let jsonrpc: String = "2.0"
    let result: JSONValue?
    let error: JSONRPCError?
    let id: JSONRPCId?

    static func success(result: JSONValue?, id: JSONRPCId?) -> JSONRPCResponse {
        JSONRPCResponse(result: result, error: nil, id: id)
    }

    static func failure(error: JSONRPCError, id: JSONRPCId?) -> JSONRPCResponse {
        JSONRPCResponse(result: nil, error: error, id: id)
    }
}

struct JSONRPCError: Codable {
    let code: Int
    let message: String
    let data: JSONValue?

    init(code: Int, message: String, data: JSONValue? = nil) {
        self.code = code
        self.message = message
        self.data = data
    }

    static let parseError = JSONRPCError(code: -32700, message: "Parse error")
    static let invalidRequest = JSONRPCError(code: -32600, message: "Invalid Request")
    static let methodNotFound = JSONRPCError(code: -32601, message: "Method not found")
    static let invalidParams = JSONRPCError(code: -32602, message: "Invalid params")
    static let internalError = JSONRPCError(code: -32603, message: "Internal error")

    static func internalError(message: String) -> JSONRPCError {
        JSONRPCError(code: -32603, message: message)
    }

    static func invalidParams(message: String) -> JSONRPCError {
        JSONRPCError(code: -32602, message: message)
    }

    static func timeout(message: String) -> JSONRPCError {
        JSONRPCError(code: -32000, message: message)
    }

    static func preconditionFailed(message: String) -> JSONRPCError {
        JSONRPCError(code: -32001, message: message)
    }
}

enum JSONValue: Codable, Equatable {
    case null
    case bool(Bool)
    case int(Int)
    case double(Double)
    case string(String)
    case array([JSONValue])
    case object([String: JSONValue])

    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()

        if container.decodeNil() {
            self = .null
        } else if let bool = try? container.decode(Bool.self) {
            self = .bool(bool)
        } else if let int = try? container.decode(Int.self) {
            self = .int(int)
        } else if let double = try? container.decode(Double.self) {
            self = .double(double)
        } else if let string = try? container.decode(String.self) {
            self = .string(string)
        } else if let array = try? container.decode([JSONValue].self) {
            self = .array(array)
        } else if let object = try? container.decode([String: JSONValue].self) {
            self = .object(object)
        } else {
            throw DecodingError.typeMismatch(
                JSONValue.self,
                DecodingError.Context(
                    codingPath: decoder.codingPath,
                    debugDescription: "Unable to decode JSON value"
                )
            )
        }
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        switch self {
        case .null:
            try container.encodeNil()
        case .bool(let value):
            try container.encode(value)
        case .int(let value):
            try container.encode(value)
        case .double(let value):
            try container.encode(value)
        case .string(let value):
            try container.encode(value)
        case .array(let value):
            try container.encode(value)
        case .object(let value):
            try container.encode(value)
        }
    }

    func toData() throws -> Data {
        try JSONEncoder().encode(self)
    }

    static func from<T: Encodable>(_ value: T) throws -> JSONValue {
        let data = try JSONEncoder().encode(value)
        let object = try JSONSerialization.jsonObject(with: data, options: [.fragmentsAllowed])
        return try JSONValue(foundationObject: object)
    }

    /// Builds a JSONValue from the object tree JSONSerialization produces.
    ///
    /// Decoding through `JSONDecoder` instead would go through `init(from:)`, which finds a
    /// value's type by attempting every case in turn and throwing until one sticks. That is
    /// six thrown errors per object, and a UI dump is tens of thousands of values, which
    /// made serialising a heavy screen cost more than snapshotting it.
    private init(foundationObject object: Any) throws {
        switch object {
        case is NSNull:
            self = .null

        case let number as NSNumber:
            // NSNumber carries bools and numbers alike; the order matters, since a bool
            // answers to intValue too. This matches what the decoder used to produce.
            if CFGetTypeID(number) == CFBooleanGetTypeID() {
                self = .bool(number.boolValue)
            } else if CFNumberIsFloatType(number) {
                self = .double(number.doubleValue)
            } else {
                self = .int(number.intValue)
            }

        case let string as String:
            self = .string(string)

        case let array as [Any]:
            self = .array(try array.map { try JSONValue(foundationObject: $0) })

        case let dictionary as [String: Any]:
            self = .object(try dictionary.mapValues { try JSONValue(foundationObject: $0) })

        default:
            throw DecodingError.typeMismatch(
                JSONValue.self,
                DecodingError.Context(
                    codingPath: [],
                    debugDescription: "Unable to convert \(type(of: object)) to a JSON value"
                )
            )
        }
    }
}

extension JSONValue {
    subscript(key: String) -> JSONValue? {
        guard case .object(let dict) = self else { return nil }
        return dict[key]
    }

    subscript(index: Int) -> JSONValue? {
        guard case .array(let array) = self, index < array.count else { return nil }
        return array[index]
    }
}
