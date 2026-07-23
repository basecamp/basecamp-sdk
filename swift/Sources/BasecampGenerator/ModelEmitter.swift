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

    // Handle oneOf schemas (union types like TodolistOrGroup)
    if let oneOf = schema["oneOf"] as? [[String: Any]] {
        collected.insert(schemaRef)
        for variant in oneOf {
            if let props = variant["properties"] as? [String: Any] {
                for (_, propValue) in props {
                    guard let propSchema = propValue as? [String: Any] else { continue }
                    if let ref = propSchema["$ref"] as? String {
                        let refName = resolveRef(ref)
                        if !collected.contains(refName) {
                            collectEntitySchemas(from: refName, schemas: schemas, into: &collected)
                        }
                    }
                }
            }
        }
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

    // Handle oneOf schemas (union types)
    if let oneOf = schema["oneOf"] as? [[String: Any]] {
        var lines: [String] = []
        lines.append("// @generated from OpenAPI spec \u{2014} do not edit directly")
        lines.append("import Foundation")
        lines.append("")
        lines.append("public struct \(typeName): Codable, Sendable {")
        for variant in oneOf {
            if let props = variant["properties"] as? [String: Any] {
                for propName in props.keys.sorted() {
                    guard let propSchema = props[propName] as? [String: Any] else { continue }
                    let swiftType = schemaToSwiftType(propSchema)
                    let camelName = toCamelCase(propName)
                    lines.append("    public var \(camelName): \(swiftType)?")
                }
            }
        }
        lines.append("}")
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

    for propName in orderedProps {
        guard let propSchema = properties[propName] as? [String: Any] else { continue }
        let baseType = schemaToSwiftType(propSchema)
        let camelName = toCamelCase(propName)
        let required = requiredFields.contains(propName)
        let valueOptional = schemaIsNullable(propSchema) || !required
        let propType = baseType + (valueOptional ? "?" : "")

        // Required members are immutable (`let`, set at init); optional members
        // stay `var` so callers can mutate/omit them.
        lines.append("    public \(required ? "let" : "var") \(camelName): \(propType)")
        // Add system_label field after FlexibleInt id fields
        if baseType == "FlexibleInt" {
            lines.append("    /// Label for system actors (e.g. \"basecamp\"). Present when personable_type is \"LocalPerson\".")
            lines.append("    public var systemLabel: String?")
        }
    }

    if !requiredFields.isEmpty {
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

        for propName in orderedProps {
            let camelName = toCamelCase(propName)
            lines.append("        self.\(camelName) = \(camelName)")
        }
        lines.append("    }")
    }

    // Synthesized Codable treats an optional-typed property as decodeIfPresent
    // (missing OK) and omits nil on encode — which is wrong for a
    // required-and-nullable member. Emit explicit coding only for such structs
    // so every other model keeps its synthesized (unchanged) Codable.
    if hasRequiredNullable {
        lines.append(contentsOf: emitRequiredNullableCoding(orderedProps: orderedProps, properties: properties, requiredFields: requiredFields))
    }

    lines.append("}")
    lines.append("")
    return lines.joined(separator: "\n")
}

/// Emits explicit `CodingKeys` + `init(from:)` + `encode(to:)` for a struct
/// that has at least one required-and-nullable member. Semantics per member:
///   - required & nullable: `decode(T?.self)` (rejects a missing key, decodes
///     JSON null -> nil) and `encode(value)` (nil -> explicit `"key": null`).
///   - required & non-null: `decode(T.self)` / `encode(value)`.
///   - optional: `decodeIfPresent` / `encodeIfPresent` (missing OK, nil omitted).
private func emitRequiredNullableCoding(orderedProps: [String], properties: [String: Any], requiredFields: Set<String>) -> [String] {
    var lines: [String] = []

    // Emits `case camel` or `case camel = "snake"`, plus the synthetic
    // system_label key for FlexibleInt id fields.
    func codingKeyLines() -> [String] {
        var out: [String] = []
        for propName in orderedProps {
            guard let propSchema = properties[propName] as? [String: Any] else { continue }
            let camelName = toCamelCase(propName)
            out.append(camelName == propName ? "        case \(camelName)" : "        case \(camelName) = \"\(propName)\"")
            if schemaToSwiftType(propSchema) == "FlexibleInt" {
                out.append("        case systemLabel = \"system_label\"")
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
        if required && nullable {
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
