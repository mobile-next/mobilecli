import Foundation
import XCTest
import os

@MainActor
protocol RPCMethodHandler {
    static var methodName: String { get }

    func execute(params: JSONValue?) async throws -> JSONValue
}

extension RPCMethodHandler {
    func decodeParams<T: Decodable>(_ type: T.Type, from params: JSONValue?) throws -> T {
        let data = if let params = params {
            try params.toData()
        } else {
            Data("{}".utf8)
        }
        do {
            return try JSONDecoder().decode(type, from: data)
        } catch let error as RPCMethodError {
            throw error
        } catch {
            throw RPCMethodError.invalidParams("Invalid parameters: \(error.localizedDescription)")
        }
    }
}

enum RPCMethodError: Error {
    case invalidParams(String)
    case internalError(String)
    case timeout(String)

    var jsonRPCError: JSONRPCError {
        switch self {
        case .invalidParams(let message):
            return .invalidParams(message: message)
        case .internalError(let message):
            return .internalError(message: message)
        case .timeout(let message):
            return .timeout(message: message)
        }
    }
}
