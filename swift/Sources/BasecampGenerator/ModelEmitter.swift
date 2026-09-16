import Foundation

// MARK: - Model Emitter

/// Collects all schemas that need to be emitted as Swift structs.
func collectModelSchemas(operations: [ParsedOperation], schemas: [String: Any]) -> (entities: [String], requests: [String]) {
    var entityNames = Set<String>()
    var requestNames = Set<String>()

    for op in operations {
        // Collect response entity schemas
        if let responseRef = op.responseSchemaRef {
            collectEntitySchemas(from: responseRef, schemas: schemas, into: &entityNames)
        }

        // Collect request body schemas
        if let bodyRef = op.bodySchemaRef {
            requestNames.insert(bodyRef)
            // Also walk request schema properties for nested $ref types
            collectEntitySchemas(fromProperties: bodyRef, schemas: schemas, into: &entityNames)
        }
    }

    return (
        entities: entityNames.sorted(),
        requests: requestNames.sorted()
    )
}

/// Recursively collects entity schemas that need to be generated.
private func collectEntitySchemas(from schemaRef: String, schemas: [String: Any], into collected: inout Set<String>) {
    guard let schema = schemas[schemaRef] as? [String: Any] else { return }

    // If it's a $ref wrapper, follow it
    if let ref = schema["$ref"] as? String {
        let refName = resolveRef(ref)
        collectEntitySchemas(from: refName, schemas: schemas, into: &collected)
        return
    }

    // If it's an array, follow items
    if (schema["type"] as? String) == "array",
       let items = schema["items"] as? [String: Any],
       let ref = items["$ref"] as? String {
        let refName = resolveRef(ref)
        collectEntitySchemas(from: refName, schemas: schemas, into: &collected)
        return
    }

    // Handle additionalProperties-only schemas (maps like WebhookHeadersMap)
    if schema["additionalProperties"] != nil && schema["properties"] == nil {
        collected.insert(schemaRef)
        return
    }

    // String enum schema — collect it for enum generation
    if (schema["type"] as? String) == "string", schema["enum"] != nil {
        collected.insert(schemaRef)
        return
    }

    // It's an object schema — add it and walk its properties
    guard (schema["type"] as? String) == "object" || schema["properties"] != nil else { return }

    // Skip error response schemas
    if schemaRef.hasSuffix("ErrorResponseContent") { return }

    collected.insert(schemaRef)

    // Walk properties for nested $ref types
    if let properties = schema["properties"] as? [String: Any] {
        for (_, propValue) in properties {
            guard let propSchema = propValue as? [String: Any] else { continue }
            if let ref = propSchema["$ref"] as? String {
                let refName = resolveRef(ref)
                if !collected.contains(refName) {
                    collectEntitySchemas(from: refName, schemas: schemas, into: &collected)
                }
            } else if (propSchema["type"] as? String) == "array",
                      let items = propSchema["items"] as? [String: Any],
                      let ref = items["$ref"] as? String {
                let refName = resolveRef(ref)
                if !collected.contains(refName) {
                    collectEntitySchemas(from: refName, schemas: schemas, into: &collected)
                }
            } else if let anyOf = propSchema["anyOf"] as? [[String: Any]] {
                // Required-and-nullable references (anyOf: [$ref, {type: "null"}])
                // hide their $ref inside the union — traverse it too.
                for member in anyOf {
                    if let ref = member["$ref"] as? String {
                        let refName = resolveRef(ref)
                        if !collected.contains(refName) {
                            collectEntitySchemas(from: refName, schemas: schemas, into: &collected)
                        }
                    }
                }
            }
        }
    }
}

/// Walks a schema's properties for nested $ref types without adding the schema itself.
/// Used for request schemas whose nested types need to be generated as entity models.
private func collectEntitySchemas(fromProperties schemaRef: String, schemas: [String: Any], into collected: inout Set<String>) {
    guard let schema = schemas[schemaRef] as? [String: Any],
          let properties = schema["properties"] as? [String: Any] else { return }

    for (_, propValue) in properties {
        guard let propSchema = propValue as? [String: Any] else { continue }
        if let ref = propSchema["$ref"] as? String {
            let refName = resolveRef(ref)
            if !collected.contains(refName) {
                collectEntitySchemas(from: refName, schemas: schemas, into: &collected)
            }
        } else if (propSchema["type"] as? String) == "array",
                  let items = propSchema["items"] as? [String: Any],
                  let ref = items["$ref"] as? String {
            let refName = resolveRef(ref)
            if !collected.contains(refName) {
                collectEntitySchemas(from: refName, schemas: schemas, into: &collected)
            }
        }
    }
}

/// Emits a Swift Codable struct for an entity or supporting schema.
func emitEntityModel(schemaName: String, schemas: [String: Any]) -> String {
    guard let schema = schemas[schemaName] as? [String: Any] else { return "" }

    let typeName = typeAliases[schemaName]?.name ?? schemaName

    // Handle string enum schemas
    if (schema["type"] as? String) == "string",
       let enumValues = schema["enum"] as? [String] {
        var lines: [String] = []
        lines.append("// @generated from OpenAPI spec \u{2014} do not edit directly")
        lines.append("import Foundation")
        lines.append("")
        lines.append("public enum \(typeName): String, Codable, Sendable {")
        for value in enumValues {
            let caseName = value.prefix(1).lowercased() + value.dropFirst()
            lines.append("    case \(caseName) = \"\(value)\"")
        }
        lines.append("}")
        lines.append("")
        return lines.joined(separator: "\n")
    }

    // Handle additionalProperties-only schemas as typealiases
    if schema["additionalProperties"] != nil && schema["properties"] == nil {
        let valueSchema = schema["additionalProperties"] as? [String: Any] ?? ["type": "String"]
        let valueType = schemaToSwiftType(valueSchema)
        var lines: [String] = []
        lines.append("// @generated from OpenAPI spec \u{2014} do not edit directly")
        lines.append("import Foundation")
        lines.append("")
        lines.append("public typealias \(typeName) = [String: \(valueType)]")
        lines.append("")
        return lines.joined(separator: "\n")
    }

    guard let properties = schema["properties"] as? [String: Any] else { return "" }

    let requiredFields = Set(schema["required"] as? [String] ?? [])

    // Partition: required properties first (sorted), then optional (sorted)
    let requiredProps = properties.keys.filter { requiredFields.contains($0) }.sorted()
    let optionalProps = properties.keys.filter { !requiredFields.contains($0) }.sorted()
    let orderedProps = requiredProps + optionalProps

    var lines: [String] = []
    lines.append("// @generated from OpenAPI spec \u{2014} do not edit directly")
    lines.append("import Foundation")
    lines.append("")
    // Documentation-only deprecation (see #406): a `///` doc comment on the
    // whole struct, no `@available`, so the generated code does not warn on its
    // own references.
    if schema["deprecated"] as? Bool == true {
        lines += deprecationDocLines(reason: (schema["x-deprecated-reason"] as? String) ?? "deprecated", indent: "")
    }
    lines.append("public struct \(typeName): Codable, Sendable {")

    // Requiredness and nullability are independent axes:
    //   nullable (type: [..., "null"]) -> the Swift value type is optional (T?)
    //   required (in the schema's required set) -> presence: `let`, no init
    //     default, and — when also nullable — custom Codable that rejects a
    //     missing key and encodes nil as an explicit JSON null.
    let hasRequiredNullable = orderedProps.contains { propName in
        guard let ps = properties[propName] as? [String: Any] else { return false }
        return requiredFields.contains(propName) && schemaIsNullable(ps)
    }
    // A person (see `isPersonSchema`) reads an absent id as `0`, and a list of
    // people reads a `null` element as the zero person. Synthesized Codable can
    // do neither, so both take the explicit coding below.
    let isPerson = isPersonSchema(schemaName, schemas: schemas)
    let hasPersonList = orderedProps.contains { propName in
        guard let ps = properties[propName] as? [String: Any] else { return false }
        return personListElement(ps, schemas: schemas) != nil
    }

    for propName in orderedProps {
        guard let propSchema = properties[propName] as? [String: Any] else { continue }
        let baseType = schemaToSwiftType(propSchema)
        let camelName = toCamelCase(propName)
        let required = requiredFields.contains(propName)
        let valueOptional = schemaIsNullable(propSchema) || !required
        let propType = baseType + (valueOptional ? "?" : "")

        // Documentation-only deprecation (see #406): `///` doc comment on the
        // property, no `@available`.
        if propSchema["deprecated"] as? Bool == true {
            lines += deprecationDocLines(reason: (propSchema["x-deprecated-reason"] as? String) ?? "deprecated", indent: "    ")
        }
        // Required members are immutable (`let`, set at init); optional members
        // stay `var` so callers can mutate/omit them.
        lines.append("    public \(required ? "let" : "var") \(camelName): \(propType)")
        // Add system_label field after FlexibleInt id fields
        if baseType == "FlexibleInt" {
            lines.append("    /// Label for system actors (e.g. \"basecamp\"). Present when personable_type is \"LocalPerson\".")
            lines.append("    public var systemLabel: String?")
        }
    }

    // Emitted unconditionally, matching `emitRequestModel`. Swift's implicit
    // memberwise initializer is `internal`, so a struct without an explicit
    // `public init` is unconstructible outside the module — an all-optional
    // model would otherwise compile in-repo (every test source that imports
    // the SDK imports it as `@testable import Basecamp`, and none plain-imports
    // it) and be uncallable for a consumer that plain-`import`s the SDK (#735).
    // `Sources/BasecampPublicAPIConsumer` is the target that observes this from
    // outside.
    lines.append("")
    var initParams: [String] = []
    for propName in orderedProps {
        guard let propSchema = properties[propName] as? [String: Any] else { continue }
        let baseType = schemaToSwiftType(propSchema)
        let camelName = toCamelCase(propName)
        let required = requiredFields.contains(propName)
        let valueOptional = schemaIsNullable(propSchema) || !required
        let propType = baseType + (valueOptional ? "?" : "")
        // Required members take no default (caller must supply presence).
        initParams.append(required ? "\(camelName): \(propType)" : "\(camelName): \(propType) = nil")
    }

    if initParams.count <= 3 {
        lines.append("    public init(\(initParams.joined(separator: ", "))) {")
    } else {
        lines.append("    public init(")
        for (i, param) in initParams.enumerated() {
            let comma = i < initParams.count - 1 ? "," : ""
            lines.append("        \(param)\(comma)")
        }
        lines.append("    ) {")
    }

    // Same guard as the two loops above: a property whose schema is not a
    // dictionary declares no member and takes no parameter, so it must not be
    // assigned either. The three loops have to agree on which properties exist.
    for propName in orderedProps {
        guard properties[propName] is [String: Any] else { continue }
        let camelName = toCamelCase(propName)
        lines.append("        self.\(camelName) = \(camelName)")
    }
    lines.append("    }")

    // Synthesized Codable treats an optional-typed property as decodeIfPresent
    // (missing OK) and omits nil on encode — which is wrong for a
    // required-and-nullable member. It is also unable to default a person's
    // absent id or read a `null` element of a person list as the zero person.
    // Emit explicit coding only for such structs so every other model keeps its
    // synthesized (unchanged) Codable.
    if hasRequiredNullable || isPerson || hasPersonList {
        lines.append(contentsOf: emitRequiredNullableCoding(orderedProps: orderedProps, properties: properties, requiredFields: requiredFields, schemas: schemas, isPerson: isPerson))
    }

    lines.append("}")

    if isPerson {
        lines.append(contentsOf: emitZeroPerson(typeName: typeName, orderedProps: orderedProps, properties: properties, requiredFields: requiredFields))
    }

    lines.append("")
    return lines.joined(separator: "\n")
}

/// Whether a schema is a person as the reference decodes one: an object whose
/// id is a required, non-null flexible integer (`generated.Person.Id` is
/// `types.FlexibleInt64`). Keyed on that marker, not on a name, so it reaches
/// exactly what Go's flexible decoder reaches — `UpcomingSchedulePerson`,
/// `OutOfOfficePerson` and the other person shapes whose id is a plain `int64`
/// in Go are not persons here, and keep their strict decode.
func isPersonSchema(_ schemaName: String, schemas: [String: Any]) -> Bool {
    guard let schema = schemas[schemaName] as? [String: Any],
          let properties = schema["properties"] as? [String: Any] else { return false }
    let required = Set(schema["required"] as? [String] ?? [])
    return properties.contains { name, value in
        guard let ps = value as? [String: Any] else { return false }
        return required.contains(name) && !schemaIsNullable(ps) && schemaToSwiftType(ps) == "FlexibleInt"
    }
}

/// The person type a property lists, when the property is an array of
/// non-null references to a person schema; `nil` otherwise.
func personListElement(_ propSchema: [String: Any], schemas: [String: Any]) -> String? {
    var base = propSchema
    if let types = propSchema["type"] as? [Any] {
        base["type"] = types.compactMap { $0 as? String }.first { $0 != "null" }
    }
    guard (base["type"] as? String) == "array",
          let items = base["items"] as? [String: Any],
          !schemaIsNullable(items),
          let ref = items["$ref"] as? String else { return nil }
    let name = resolveRef(ref)
    return isPersonSchema(name, schemas: schemas) ? name : nil
}

/// Emits the `ZeroPerson` conformance: the value a `null` list element decodes
/// to, which is Go's zero struct — every required member at its zero value.
private func emitZeroPerson(typeName: String, orderedProps: [String], properties: [String: Any], requiredFields: Set<String>) -> [String] {
    var args: [String] = []
    for propName in orderedProps where requiredFields.contains(propName) {
        guard let ps = properties[propName] as? [String: Any] else { continue }
        let zero: String
        if schemaIsNullable(ps) {
            zero = "nil"
        } else {
            switch schemaToSwiftType(ps) {
            case "FlexibleInt", "Int", "Int32", "Double": zero = "0"
            case "String": zero = "\"\""
            case "Bool": zero = "false"
            case let t where t.hasPrefix("["): zero = "[]"
            case let t:
                fatalError("\(typeName).\(propName): no zero value for \(t) — a person's required member must have one")
            }
        }
        args.append("\(toCamelCase(propName)): \(zero)")
    }
    return [
        "",
        "extension \(typeName): ZeroPerson {",
        "    static var zero: \(typeName) { \(typeName)(\(args.joined(separator: ", "))) }",
        "}",
    ]
}

/// Emits explicit `CodingKeys` + `init(from:)` + `encode(to:)` for a struct
/// that has at least one required-and-nullable member. Semantics per member:
///   - required & nullable: `decode(T?.self)` (rejects a missing key, decodes
///     JSON null -> nil) and `encode(value)` (nil -> explicit `"key": null`).
///   - required & non-null: `decode(T.self)` / `encode(value)`.
///   - optional: `decodeIfPresent` / `encodeIfPresent` (missing OK, nil omitted).
private func emitRequiredNullableCoding(orderedProps: [String], properties: [String: Any], requiredFields: Set<String>, schemas: [String: Any], isPerson: Bool) -> [String] {
    var lines: [String] = []

    // Emits bare `case camel` for every field. BaseService's decoder/encoder use
    // `.convertFromSnakeCase`/`.convertToSnakeCase`, which map the camelCase
    // CodingKey rawValue to/from the snake_case wire key. Emitting an explicit
    // `= "snake"` rawValue here would instead be matched against the *converted*
    // (camelCase) incoming key and fail with keyNotFound for any snake_case field
    // (e.g. `app_url`). Single-word fields already match under either scheme.
    func codingKeyLines() -> [String] {
        var out: [String] = []
        for propName in orderedProps {
            guard let propSchema = properties[propName] as? [String: Any] else { continue }
            let camelName = toCamelCase(propName)
            out.append("        case \(camelName)")
            if schemaToSwiftType(propSchema) == "FlexibleInt" {
                out.append("        case systemLabel")
            }
        }
        return out
    }

    lines.append("")
    lines.append("    enum CodingKeys: String, CodingKey {")
    lines.append(contentsOf: codingKeyLines())
    lines.append("    }")

    lines.append("")
    lines.append("    public init(from decoder: any Decoder) throws {")
    lines.append("        let container = try decoder.container(keyedBy: CodingKeys.self)")
    for propName in orderedProps {
        guard let propSchema = properties[propName] as? [String: Any] else { continue }
        let baseType = schemaToSwiftType(propSchema)
        let camelName = toCamelCase(propName)
        let required = requiredFields.contains(propName)
        let nullable = schemaIsNullable(propSchema)
        if let person = personListElement(propSchema, schemas: schemas) {
            // A `null` element is the zero person (`decodePeople`, `PersonList.swift`).
            if required && nullable {
                lines.append("        guard container.contains(.\(camelName)) else { throw DecodingError.keyNotFound(CodingKeys.\(camelName), DecodingError.Context(codingPath: container.codingPath, debugDescription: \"\(propName) is required\")) }")
                lines.append("        self.\(camelName) = try container.decodePeopleIfPresent([\(person)].self, forKey: .\(camelName))")
            } else if required {
                lines.append("        self.\(camelName) = try container.decodePeople([\(person)].self, forKey: .\(camelName))")
            } else {
                lines.append("        self.\(camelName) = try container.decodePeopleIfPresent([\(person)].self, forKey: .\(camelName))")
            }
        } else if isPerson && required && !nullable && baseType == "FlexibleInt" {
            // Absent is Go's zero value, `0`: `FlexibleInt64`'s reader only runs
            // for a key that is there. A present `null` still reaches the reader
            // and fails the read, as `ParseInt("")` fails it in the reference.
            lines.append("        self.\(camelName) = try container.contains(.\(camelName)) ? container.decode(FlexibleInt.self, forKey: .\(camelName)) : FlexibleInt(0)")
        } else if required && nullable {
            // `decode(T?.self)` requires the key present but accepts null.
            lines.append("        self.\(camelName) = try container.decode(\(baseType)?.self, forKey: .\(camelName))")
        } else if required {
            lines.append("        self.\(camelName) = try container.decode(\(baseType).self, forKey: .\(camelName))")
        } else {
            lines.append("        self.\(camelName) = try container.decodeIfPresent(\(baseType).self, forKey: .\(camelName))")
        }
        if baseType == "FlexibleInt" {
            lines.append("        self.systemLabel = try container.decodeIfPresent(String.self, forKey: .systemLabel)")
        }
    }
    lines.append("    }")

    lines.append("")
    lines.append("    public func encode(to encoder: any Encoder) throws {")
    lines.append("        var container = encoder.container(keyedBy: CodingKeys.self)")
    for propName in orderedProps {
        guard let propSchema = properties[propName] as? [String: Any] else { continue }
        let baseType = schemaToSwiftType(propSchema)
        let camelName = toCamelCase(propName)
        let required = requiredFields.contains(propName)
        // Required (incl. required-nullable) always encodes: nil -> explicit null.
        lines.append("        try container.\(required ? "encode" : "encodeIfPresent")(self.\(camelName), forKey: .\(camelName))")
        if baseType == "FlexibleInt" {
            lines.append("        try container.encodeIfPresent(self.systemLabel, forKey: .systemLabel)")
        }
    }
    lines.append("    }")

    return lines
}

/// Emits a Swift Codable struct for a request body.
func emitRequestModel(schemaName: String, schemas: [String: Any]) -> String {
    guard let schema = schemas[schemaName] as? [String: Any],
          let properties = schema["properties"] as? [String: Any] else {
        return ""
    }

    let requiredFields = Set(schema["required"] as? [String] ?? [])

    // Derive a clean type name: "CreateTodoRequestContent" → "CreateTodoRequest"
    var typeName = schemaName
    if typeName.hasSuffix("Content") {
        typeName = String(typeName.dropLast("Content".count))
    }
    // For schemas that are already named "...Request" (like CreatePersonRequest), keep as-is
    if !typeName.hasSuffix("Request") && !typeName.hasSuffix("Payload") {
        typeName += "Request"
    }

    var lines: [String] = []
    lines.append("// @generated from OpenAPI spec \u{2014} do not edit directly")
    lines.append("import Foundation")
    lines.append("")
    lines.append("public struct \(typeName): Codable, Sendable {")

    // Properties: required use `let`, optional use `var`
    let sortedProps = properties.keys.sorted()
    for propName in sortedProps {
        guard let propSchema = properties[propName] as? [String: Any] else { continue }
        let swiftType = schemaToSwiftType(propSchema)
        let camelName = toCamelCase(propName)
        let isRequired = requiredFields.contains(propName)

        if isRequired {
            lines.append("    public let \(camelName): \(swiftType)")
        } else {
            lines.append("    public var \(camelName): \(swiftType)?")
        }
    }

    // Memberwise init
    lines.append("")
    var initParams: [String] = []
    for propName in sortedProps {
        guard let propSchema = properties[propName] as? [String: Any] else { continue }
        let swiftType = schemaToSwiftType(propSchema)
        let camelName = toCamelCase(propName)
        let isRequired = requiredFields.contains(propName)
        if isRequired {
            initParams.append("\(camelName): \(swiftType)")
        } else {
            initParams.append("\(camelName): \(swiftType)? = nil")
        }
    }

    if initParams.count <= 3 {
        lines.append("    public init(\(initParams.joined(separator: ", "))) {")
    } else {
        lines.append("    public init(")
        for (i, param) in initParams.enumerated() {
            let comma = i < initParams.count - 1 ? "," : ""
            lines.append("        \(param)\(comma)")
        }
        lines.append("    ) {")
    }

    for propName in sortedProps {
        let camelName = toCamelCase(propName)
        lines.append("        self.\(camelName) = \(camelName)")
    }
    lines.append("    }")

    lines.append("}")
    lines.append("")
    return lines.joined(separator: "\n")
}
