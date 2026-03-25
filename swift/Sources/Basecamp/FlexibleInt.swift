import Foundation

/// An integer that decodes from either a JSON number or a JSON string.
///
/// The Basecamp API sometimes returns person IDs as strings (e.g. `"12345"`)
/// instead of numbers, and uses non-numeric sentinels like `"basecamp"` for
/// system-generated entities. `FlexibleInt` handles all three wire formats:
///
/// - JSON number `12345` → `value = 12345`
/// - JSON string `"12345"` → `value = 12345`
/// - JSON string `"basecamp"` → `value = 0` (non-numeric sentinel)
/// - JSON string `"9223372036854775808"` → throws (numeric overflow)
public struct FlexibleInt: Codable, Sendable, Hashable, CustomStringConvertible, ExpressibleByIntegerLiteral {
    public let value: Int

    public init(_ value: Int) {
        self.value = value
    }

    public init(integerLiteral value: Int) {
        self.value = value
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if let i = try? container.decode(Int.self) {
            value = i
        } else if let s = try? container.decode(String.self) {
            if let i = Int(s) {
                value = i
            } else if s.range(of: #"^-?\d+$"#, options: .regularExpression) != nil {
                // Valid integer form but didn't parse — overflow
                throw DecodingError.dataCorruptedError(
                    in: container,
                    debugDescription: "FlexibleInt: \"\(s)\" overflows Int"
                )
            } else {
                // Non-numeric sentinel (e.g. "basecamp")
                value = 0
            }
        } else {
            value = 0
        }
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        try container.encode(value)
    }

    public var description: String { "\(value)" }
}
