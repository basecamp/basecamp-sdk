import Foundation
import XCTest

@testable import BasecampGenerator

/// Pins the one strict test for a deprecated alias component (#955): nothing
/// but a `$ref` whose `deprecated` is the JSON boolean `true`.
///
/// `JSONSerialization` hands `1` back as an `NSNumber` that `as? Bool` bridges
/// to `true`, so reading the flag from that tree let `"deprecated": 1` make an
/// alias every other generator declines. The refusals for aliases the generator
/// cannot place run against the real binary in
/// scripts/test-compiled-generator-refusal.
final class DeprecatedAliasSchemaTests: XCTestCase {
    private let spec = """
        {"components": {"schemas": {
          "Subtask": {"type": "object", "properties": {"id": {"type": "integer"}}},
          "CardStep": {"$ref": "#/components/schemas/Subtask", "deprecated": true, "x-deprecated-reason": "renamed"},
          "NumberOne": {"$ref": "#/components/schemas/Subtask", "deprecated": 1},
          "StringTrue": {"$ref": "#/components/schemas/Subtask", "deprecated": "true"},
          "WithType": {"$ref": "#/components/schemas/Subtask", "deprecated": true, "type": "object"},
          "Scalar": true
        }}}
        """

    private func load() throws -> (schemas: [String: Any], deprecated: Set<String>) {
        let data = Data(spec.utf8)
        let root = try XCTUnwrap(try JSONSerialization.jsonObject(with: data) as? [String: Any])
        let components = try XCTUnwrap(root["components"] as? [String: Any])
        let schemas = try XCTUnwrap(components["schemas"] as? [String: Any])
        return (schemas, strictlyDeprecatedComponents(openapiData: data))
    }

    func testOnlyTheJsonBooleanTrueCountsAsDeprecated() throws {
        let (_, deprecated) = try load()
        XCTAssertEqual(deprecated, ["CardStep", "WithType"])
    }

    func testOnlyAPureDeprecatedRefIsAnAlias() throws {
        let (schemas, deprecated) = try load()
        let aliases = deprecatedAliasSchemas(schemas: schemas, deprecated: deprecated)
        XCTAssertEqual(aliases.map(\.alias), ["CardStep"])
        XCTAssertEqual(aliases.map(\.target), ["Subtask"])
        XCTAssertFalse(isDeprecatedAlias("WithType", schemas: schemas, deprecated: deprecated))
    }

    func testAnObjectModelHasProperties() throws {
        let (schemas, _) = try load()
        XCTAssertTrue(isObjectModel("Subtask", schemas: schemas))
        XCTAssertFalse(isObjectModel("CardStep", schemas: schemas))
        XCTAssertFalse(isObjectModel("Missing", schemas: schemas))
    }
}
