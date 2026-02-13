/// Loose-typed JSON for endpoints without stable typed models.
///
/// Useful when extending the SDK with custom services for internal
/// or experimental endpoints. Decodes any JSON value.
///
/// Numeric values are stored as `Double`, matching Foundation's
/// `JSONSerialization` behavior. This means integers beyond 2^53
/// may lose precision — use typed models for endpoints where large
/// integer IDs are critical.
///
/// ```swift
/// let result: JSONValue = try await request(info, method: "GET", path: "/internal/account.json")
/// if case .object(let dict) = result, case .string(let name) = dict["name"] {
///     print(name)
/// }
/// ```
public enum JSONValue: Decodable, Sendable, Equatable {
    case string(String)
    case number(Double)
    case bool(Bool)
    case object([String: JSONValue])
    case array([JSONValue])
    case null

    public init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if container.decodeNil() { self = .null }
        else if let b = try? container.decode(Bool.self) { self = .bool(b) }
        else if let n = try? container.decode(Double.self) { self = .number(n) }
        else if let s = try? container.decode(String.self) { self = .string(s) }
        else if let a = try? container.decode([JSONValue].self) { self = .array(a) }
        else if let o = try? container.decode([String: JSONValue].self) { self = .object(o) }
        else { throw DecodingError.dataCorruptedError(in: container, debugDescription: "Unsupported JSON type") }
    }
}
