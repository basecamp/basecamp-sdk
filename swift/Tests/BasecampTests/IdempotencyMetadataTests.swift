import XCTest

@testable import BasecampGenerator

/// Verifies the generator carries the `idempotent` flag from behavior-model.json
/// into `Metadata.swift` (#411). Covers the parser (flag present/absent/false)
/// and the emitter (deterministically sorted set containing exactly the true ops).
final class IdempotencyMetadataTests: XCTestCase {
    private func parse(_ json: String) throws -> [BehaviorRetryConfig] {
        try parseBehaviorModel(data: Data(json.utf8))
    }

    /// A fixture with three ops: one explicitly idempotent, one explicitly not,
    /// one omitting the flag entirely. Only the `retry` block is required for an
    /// op to be parsed at all.
    private let fixture = """
    {
      "operations": {
        "CompleteTodo": {
          "idempotent": true,
          "retry": { "max": 3, "base_delay_ms": 1000, "backoff": "exponential", "retry_on": [429, 503] }
        },
        "CreateProject": {
          "idempotent": false,
          "retry": { "max": 3, "base_delay_ms": 1000, "backoff": "exponential", "retry_on": [429, 503] }
        },
        "GetProject": {
          "retry": { "max": 3, "base_delay_ms": 1000, "backoff": "exponential", "retry_on": [429, 503] }
        }
      }
    }
    """

    func testParsesIdempotentFlag() throws {
        let byId = Dictionary(uniqueKeysWithValues: try parse(fixture).map { ($0.operationId, $0) })

        XCTAssertEqual(byId["CompleteTodo"]?.idempotent, true, "explicit idempotent:true parses true")
        XCTAssertEqual(byId["CreateProject"]?.idempotent, false, "explicit idempotent:false parses false")
        XCTAssertEqual(byId["GetProject"]?.idempotent, false, "absent flag defaults to false")
    }

    func testEmitsSortedIdempotentSetWithExactlyTrueOps() throws {
        // Feed the emitter unsorted input to prove it sorts, and include two
        // idempotent ops so ordering is observable.
        let configs = [
            BehaviorRetryConfig(operationId: "SubscribeToCardColumn", maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503], idempotent: true),
            BehaviorRetryConfig(operationId: "CreateProject", maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503], idempotent: false),
            BehaviorRetryConfig(operationId: "CompleteTodo", maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503], idempotent: true),
        ]

        let code = emitMetadata(configs: configs)

        // Exactly the true ops appear, and they are sorted deterministically.
        XCTAssertTrue(code.contains("private static let idempotentOperations: Set<String> = ["), "emits the set")
        XCTAssertTrue(code.contains("\"CompleteTodo\","), "includes idempotent CompleteTodo")
        XCTAssertTrue(code.contains("\"SubscribeToCardColumn\","), "includes idempotent SubscribeToCardColumn")
        XCTAssertFalse(code.contains("\"CreateProject\","), "excludes non-idempotent CreateProject from the set")

        // Sorted: CompleteTodo precedes SubscribeToCardColumn in the emitted set.
        guard let completeIdx = code.range(of: "\"CompleteTodo\","),
              let subscribeIdx = code.range(of: "\"SubscribeToCardColumn\",") else {
            return XCTFail("expected both idempotent ops in output")
        }
        XCTAssertTrue(completeIdx.lowerBound < subscribeIdx.lowerBound, "idempotentOperations must be sorted")

        XCTAssertTrue(code.contains("static func isIdempotent(for operationId: String) -> Bool"), "emits the lookup")
    }
}
