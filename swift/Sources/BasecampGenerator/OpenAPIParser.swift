import Foundation

// MARK: - Parsed Operation

struct ParsedOperation {
    let operationId: String
    let methodName: String
    let httpMethod: String
    let path: String
    let description: String
    let tag: String
    let pathParams: [PathParam]
    let queryParams: [QueryParam]
    let bodySchemaRef: String?
    let bodyProperties: [BodyProperty]
    let bodyRequired: Bool
    let bodyContentType: BodyContentType?
    let responseSchemaRef: String?
    let returnsArray: Bool
    let returnsVoid: Bool
    let isMutation: Bool
    let resourceType: String
    let hasPagination: Bool
}

struct PathParam {
    let name: String
    let swiftType: String
    let description: String?
}

struct QueryParam {
    let name: String
    let swiftType: String
    let required: Bool
    let description: String?
}

struct BodyProperty {
    let name: String
    let swiftType: String
    let required: Bool
    let description: String?
    let isRef: Bool
}

enum BodyContentType {
    case json
    case octetStream
}

// MARK: - Parser

/// Parses all operations from the OpenAPI spec.
func parseAllOperations(spec: [String: Any]) -> (operations: [ParsedOperation], schemas: [String: Any]) {
    let paths = spec["paths"] as? [String: Any] ?? [:]
    let components = spec["components"] as? [String: Any] ?? [:]
    let schemas = components["schemas"] as? [String: Any] ?? [:]

    var operations: [ParsedOperation] = []

    for path in paths.keys.sorted() {
        let pathItem = paths[path]!
        guard let pathDict = pathItem as? [String: Any] else { continue }

        for method in ["get", "post", "put", "patch", "delete"] {
            guard let operation = pathDict[method] as? [String: Any] else { continue }
            guard operation["operationId"] is String else { continue }

            let parsed = parseOperation(
                path: path, method: method, operation: operation, schemas: schemas
            )
            operations.append(parsed)
        }
    }

    return (operations, schemas)
}

/// Parses a single operation.
func parseOperation(
    path: String, method: String, operation: [String: Any], schemas: [String: Any]
) -> ParsedOperation {
    let operationId = operation["operationId"] as! String
    let httpMethod = method.uppercased()
    let methodName = extractMethodName(operationId)
    let desc = (operation["description"] as? String)
        ?? (operation["summary"] as? String)
        ?? "\(methodName) operation"
    let tags = operation["tags"] as? [String] ?? []
    let tag = tags.first ?? "Untagged"

    // Path parameters
    let parameters = operation["parameters"] as? [[String: Any]] ?? []
    let pathParams: [PathParam] = parameters
        .filter { ($0["in"] as? String) == "path" && ($0["name"] as? String) != "accountId" }
        .map { param in
            let name = param["name"] as! String
            let schema = param["schema"] as? [String: Any] ?? [:]
            let type = (schema["type"] as? String) == "integer" ? "Int" : "String"
            return PathParam(
                name: toCamelCase(name),
                swiftType: type,
                description: param["description"] as? String
            )
        }

    // Query parameters
    let queryParams: [QueryParam] = parameters
        .filter { ($0["in"] as? String) == "query" }
        .map { param in
            let name = param["name"] as! String
            let schema = param["schema"] as? [String: Any] ?? [:]
            let type = mapQueryParamType(schema)
            return QueryParam(
                name: name,
                swiftType: type,
                required: param["required"] as? Bool ?? false,
                description: param["description"] as? String
            )
        }

    // Request body
    var bodySchemaRef: String?
    var bodyProperties: [BodyProperty] = []
    var bodyRequired = false
    var bodyContentType: BodyContentType?

    if let requestBody = operation["requestBody"] as? [String: Any],
       let content = requestBody["content"] as? [String: Any] {
        bodyRequired = requestBody["required"] as? Bool ?? false

        if let jsonContent = content["application/json"] as? [String: Any],
           let schema = jsonContent["schema"] as? [String: Any] {
            bodyContentType = .json
            if let ref = schema["$ref"] as? String {
                bodySchemaRef = resolveRef(ref)
                bodyProperties = extractBodyProperties(schemaName: bodySchemaRef!, schemas: schemas)
            }
        } else if content["application/octet-stream"] != nil {
            bodyContentType = .octetStream
        }
    }

    // Response
    var responseSchemaRef: String?
    var returnsArray = false
    let responses = operation["responses"] as? [String: Any] ?? [:]
    let successResponse = (responses["200"] ?? responses["201"]) as? [String: Any]

    if let content = successResponse?["content"] as? [String: Any],
       let jsonContent = content["application/json"] as? [String: Any],
       let schema = jsonContent["schema"] as? [String: Any] {
        if let ref = schema["$ref"] as? String {
            responseSchemaRef = resolveRef(ref)
            // Check if referenced schema is an array type
            if let resolved = schemas[responseSchemaRef!] as? [String: Any],
               (resolved["type"] as? String) == "array" {
                returnsArray = true
            }
        }
        if (schema["type"] as? String) == "array" {
            returnsArray = true
        }
    }

    let returnsVoid = isVoidResponse(responses)
    let isMutation = httpMethod != "GET"
    let resourceType = extractResourceType(operationId)
    let hasPagination = operation["x-basecamp-pagination"] != nil

    return ParsedOperation(
        operationId: operationId,
        methodName: methodName,
        httpMethod: httpMethod,
        path: path,
        description: desc,
        tag: tag,
        pathParams: pathParams,
        queryParams: queryParams,
        bodySchemaRef: bodySchemaRef,
        bodyProperties: bodyProperties,
        bodyRequired: bodyRequired,
        bodyContentType: bodyContentType,
        responseSchemaRef: responseSchemaRef,
        returnsArray: returnsArray,
        returnsVoid: returnsVoid,
        isMutation: isMutation,
        resourceType: resourceType,
        hasPagination: hasPagination
    )
}

// MARK: - Helpers

private func isVoidResponse(_ responses: [String: Any]) -> Bool {
    let successResponse = (responses["200"] ?? responses["201"] ?? responses["204"]) as? [String: Any]
    guard let resp = successResponse else { return true }
    guard let content = resp["content"] as? [String: Any] else { return true }
    return content["application/json"] == nil
}

private func extractBodyProperties(schemaName: String, schemas: [String: Any]) -> [BodyProperty] {
    guard let schema = schemas[schemaName] as? [String: Any] else { return [] }
    guard let properties = schema["properties"] as? [String: Any] else { return [] }
    let requiredFields = schema["required"] as? [String] ?? []

    return properties.keys.sorted().compactMap { propName in
        guard let propSchema = properties[propName] as? [String: Any] else { return nil }
        let swiftType = schemaToSwiftType(propSchema)
        let isRef = propSchema["$ref"] != nil
            || ((propSchema["type"] as? String) == "array" && (propSchema["items"] as? [String: Any])?["$ref"] != nil)
        return BodyProperty(
            name: propName,
            swiftType: swiftType,
            required: requiredFields.contains(propName),
            description: propSchema["description"] as? String,
            isRef: isRef
        )
    }
}

private func mapQueryParamType(_ schema: [String: Any]) -> String {
    let type = schema["type"] as? String ?? "string"
    switch type {
    case "integer": return "Int"
    case "boolean": return "Bool"
    case "number": return "Double"
    default: return "String"
    }
}
