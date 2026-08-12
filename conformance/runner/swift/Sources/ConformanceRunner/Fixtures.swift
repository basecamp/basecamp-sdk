import Foundation

/// Arbitrary JSON value that preserves 64-bit integer precision.
///
/// Fixture bodies carry IDs beyond 2^53 (integer-precision.json), so numbers
/// are decoded as `Int64` first and only fall back to `Double` when the value
/// is not an integer. Re-encoding an `.int` therefore round-trips the exact
/// digits to the wire.
indirect enum JSON: Codable, Equatable, Sendable {
    case null
    case bool(Bool)
    case int(Int64)
    case double(Double)
    case string(String)
    case array([JSON])
    case object([String: JSON])

    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if container.decodeNil() {
            self = .null
        } else if let b = try? container.decode(Bool.self) {
            self = .bool(b)
        } else if let i = try? container.decode(Int64.self) {
            self = .int(i)
        } else if let d = try? container.decode(Double.self) {
            self = .double(d)
        } else if let s = try? container.decode(String.self) {
            self = .string(s)
        } else if let a = try? container.decode([JSON].self) {
            self = .array(a)
        } else if let o = try? container.decode([String: JSON].self) {
            self = .object(o)
        } else {
            throw DecodingError.dataCorruptedError(
                in: container, debugDescription: "Unsupported JSON value")
        }
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        switch self {
        case .null: try container.encodeNil()
        case .bool(let b): try container.encode(b)
        case .int(let i): try container.encode(i)
        case .double(let d): try container.encode(d)
        case .string(let s): try container.encode(s)
        case .array(let a): try container.encode(a)
        case .object(let o): try container.encode(o)
        }
    }

    // MARK: - Accessors

    var intValue: Int64? {
        switch self {
        case .int(let i): i
        case .double(let d): d == d.rounded() ? Int64(exactly: d.rounded()) : nil
        default: nil
        }
    }

    var stringValue: String? {
        if case .string(let s) = self { return s }
        return nil
    }

    var boolValue: Bool? {
        if case .bool(let b) = self { return b }
        return nil
    }

    var arrayValue: [JSON]? {
        if case .array(let a) = self { return a }
        return nil
    }

    var objectValue: [String: JSON]? {
        if case .object(let o) = self { return o }
        return nil
    }

    /// Display form used in failure messages.
    var display: String {
        switch self {
        case .null: "null"
        case .bool(let b): String(b)
        case .int(let i): String(i)
        case .double(let d): String(d)
        case .string(let s): "\"\(s)\""
        case .array, .object:
            (try? String(data: JSONEncoder().encode(self), encoding: .utf8) ?? "?") ?? "?"
        }
    }

    /// Serializes this value to JSON `Data`.
    func serialized() throws -> Data {
        // JSONEncoder refuses top-level fragments on older platforms; wrap and
        // slice is unnecessary on modern macOS — encode directly.
        try JSONEncoder().encode(self)
    }

    /// Parses raw data into a JSON value, or nil when not valid JSON.
    static func parse(_ data: Data) -> JSON? {
        try? JSONDecoder().decode(JSON.self, from: data)
    }

    /// Navigates a dot-separated key path through nested objects.
    func navigate(_ path: String) -> JSON? {
        var current = self
        for key in path.split(separator: ".") {
            guard let next = current.objectValue?[String(key)] else { return nil }
            current = next
        }
        return current
    }
}

// MARK: - Fixture models

/// One conformance test case, matching conformance/tests.schema.json.
struct TestCase: Decodable {
    let name: String
    let operation: String
    private let method: String?
    private let path: String?
    let pathParams: [String: JSON]?
    let queryParams: [String: JSON]?
    let requestBody: [String: JSON]?
    private let mockResponses: [MockResponse]?
    private let assertions: [Assertion]?
    private let tags: [String]?
    let configOverrides: ConfigOverrides?
    private let mode: String?

    var fixtureMethod: String { method ?? "" }
    var fixturePath: String { path ?? "" }
    var responses: [MockResponse] { mockResponses ?? [] }
    var allAssertions: [Assertion] { assertions ?? [] }
    var allTags: [String] { tags ?? [] }
    /// Whether the fixture stated a queue at all. An EMPTY queue is a
    /// deliberate declaration (the HTTPS-enforcement case makes no request);
    /// an absent key is a malformed fixture, and collapsing the two lets one
    /// through as a test that exercises nothing.
    var declaresMockResponses: Bool { mockResponses != nil }
    /// Live tests are TS-only (canonical wire-capturer); other runners filter
    /// them out at load time.
    var isMock: Bool { (mode ?? "mock") == "mock" }
}

struct ConfigOverrides: Decodable {
    let baseUrl: String?
    let maxPages: Int?
    let maxItems: Int?
    /// Pins the list operation to a single page (SPEC section 8).
    let page: Int?
    /// Overrides the client-wide retry cap as a TOTAL attempt count (SPEC
    /// section 2). Swift exposes no numeric cap by design, so the runner maps
    /// this onto `enableRetry` — see Runner.swift.
    let maxRetries: Int?
}

struct MockResponse: Decodable, Sendable {
    let status: Int?
    /// Raw flag, kept rather than folded into `isNetworkError`: the schema pins
    /// the literal `true`, so `networkError: false` is not legal. Collapsing it
    /// to "not a network error" lets a `status` + `networkError: false` entry
    /// slip past the exactly-one-of backstop and be served as a plain success.
    let networkError: Bool?
    private let headers: [String: String]?
    let body: JSON?
    private let delay: Int?

    var isNetworkError: Bool { networkError == true }
    var allHeaders: [String: String] { headers ?? [:] }
    var delayMs: Int { delay ?? 0 }
}

struct Assertion: Decodable {
    let type: String
    /// An explicit `expected: null` is a real expectation (the field must be
    /// absent), distinct from an omitted key (the assertion is malformed). The
    /// synthesized `decodeIfPresent` collapses both to nil, which turns the
    /// former into "assertion missing expected value" — a false FAIL.
    let expected: JSON?
    private let min: Double?
    private let max: Double?
    private let path: String?
    /// Request index for per-request assertions (0-based; negative = from end).
    private let index: Int?

    private enum CodingKeys: String, CodingKey {
        case type, expected, min, max, path, index
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        type = try container.decode(String.self, forKey: .type)
        expected = container.contains(.expected)
            ? try container.decode(JSON.self, forKey: .expected)
            : nil
        min = try container.decodeIfPresent(Double.self, forKey: .min)
        max = try container.decodeIfPresent(Double.self, forKey: .max)
        path = try container.decodeIfPresent(String.self, forKey: .path)
        index = try container.decodeIfPresent(Int.self, forKey: .index)
    }

    var maxValue: Double { max ?? 0 }
    var fieldPath: String { path ?? "" }
    var requestIndex: Int { index ?? 0 }
    /// Raw minimum for `delayBetweenRequests`. `checkDelayGaps` applies the
    /// default itself so no call site can gate the assertion away — a `min` of
    /// zero silently disabled the whole check in two other runners.
    var minDelayMs: Double? { min }
    /// Raw index for `delayBetweenRequests`, which must tell "gap 0" from
    /// "every gap". Distinct from `requestIndex`, whose 0 default is correct
    /// for the per-request assertions.
    var gapIndex: Int? { index }
}
