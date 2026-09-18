import Foundation

// MARK: - Path Item walking (basecamp-sdk#925)
//
// A Path Item Object is read by EXCLUSION. Its non-operation fields are a
// closed, spec-defined set and its extensions are `x-` prefixed, so every OTHER
// field is an operation. Enumerating the verbs instead is the defect this
// replaces: Smithy's `@http` trait takes the method as a free-form string it
// "will use literally and will perform no validation on", so a model author
// writing `method: "HEAD"` produced a valid model, a valid openapi.json, and no
// method on any client — the verb was not in the list, so the operation was
// stepped over in silence.

let nonOperationPathItemFields: Set<String> = ["summary", "description", "servers", "parameters"]

func failGeneration(_ message: String) -> Never {
    FileHandle.standardError.write(Data((message + "\n").utf8))
    exit(1)
}

/// The ordered HTTP methods the SDK generators emit — one declaration for all
/// six SDKs. See spec/generated-verbs.json for why it is a policy rather than
/// six per-language capabilities.
func loadGeneratedVerbs(path: String) -> [String] {
    guard let data = FileManager.default.contents(atPath: path) else {
        failGeneration("Error: generated-verb declaration not found: \(path)")
    }
    guard
        let declaration = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
        let verbs = declaration["verbs"] as? [String],
        !verbs.isEmpty,
        // A positive character class rather than a blankness predicate. An HTTP
        // method is a token, so `[a-z]+` says what a verb IS and is written the
        // same way in all six loaders; a "not blank" rule would keep diverging,
        // because every language defines whitespace differently.
        verbs.allSatisfy({ $0.allSatisfy { $0.isASCII && $0.isLowercase && $0.isLetter } })
    else {
        failGeneration(
            "Error: \(path) must declare a non-empty `verbs` array of lowercase ASCII method "
                + "names (/[a-z]+/)."
        )
    }
    return verbs
}

/// Every operation in one path item, as (verb, operation) pairs.
///
/// `emittable` bounds what the caller can RENDER, and is checked after
/// discovery: an operation on any other verb stops the run by name rather than
/// being dropped. Pass nil from a caller that is verb-agnostic.
///
/// Visit order follows `order` so emitted output stays byte-stable; a verb
/// outside it sorts deterministically to the end by name, which is ordering,
/// not membership.
func operationsOf(
    path: String, pathItem: Any, order: [String], emittable: [String]?
) -> [(String, [String: Any])] {
    guard let item = pathItem as? [String: Any] else {
        failGeneration("Error: openapi.json path \(path) is not a path item object.")
    }

    // A `$ref` path item points at operations this walk cannot see without
    // resolving the reference. Skipping it is the same silent under-count the
    // exclusion walk exists to prevent, so refuse instead.
    if item["$ref"] != nil {
        failGeneration(
            "Error: openapi.json path \(path) is a $ref; resolving a path-item reference is not "
                + "implemented, and skipping it would hide every operation behind it from the SDK."
        )
    }

    // OpenAPI 3.2's `additionalOperations` is a MAP of method to Operation, not
    // an operation. Read as one it carries no operationId, so a verb-agnostic
    // caller would drop every operation inside it without saying so. Refuse by
    // name until the walk learns the map shape.
    if item["additionalOperations"] != nil {
        failGeneration(
            "Error: openapi.json path \(path) declares `additionalOperations`, which OpenAPI 3.2 "
                + "defines as a map of method to Operation. This walk reads a path-item field as a "
                + "single operation, so it would drop every operation inside it. Teach the walk the "
                + "map shape, or take the field out of the spec."
        )
    }

    let rank = { (field: String) -> Int in order.firstIndex(of: field) ?? order.count }
    let fields = item.keys
        .filter { !nonOperationPathItemFields.contains($0) && !$0.hasPrefix("x-") }
        .sorted { rank($0) != rank($1) ? rank($0) < rank($1) : $0 < $1 }

    return fields.map { field in
        guard let operation = item[field] as? [String: Any] else {
            failGeneration(
                "Error: openapi.json path \(path) field \"\(field)\" is neither a known "
                    + "non-operation field nor an operation object. If a later OpenAPI version "
                    + "added it, add it to nonOperationPathItemFields with a reason."
            )
        }
        if let emittable, !emittable.contains(field) {
            let opId = operation["operationId"] as? String ?? "(no operationId)"
            failGeneration(
                "Error: openapi.json declares \(field.uppercased()) \(path) (\(opId)), and this "
                    + "generator emits only \(emittable.map { $0.uppercased() }.joined(separator: "/")). "
                    + "Generating the rest of the SDK without it would drop the operation from "
                    + "every client in silence, which is the failure basecamp-sdk#925 closed. Give "
                    + "the runtime a \(field) helper and add \"\(field)\" to "
                    + "spec/generated-verbs.json (read that file first — the other five SDKs need "
                    + "the same helper), or take the operation out of the Smithy model."
            )
        }
        return (field, operation)
    }
}
