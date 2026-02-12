import Foundation

// MARK: - Service Emitter

/// Emits a complete service class file for a ServiceDefinition.
func emitService(_ service: ServiceDefinition, schemas: [String: Any]) -> String {
    var lines: [String] = []

    lines.append("// @generated from OpenAPI spec \u{2014} do not edit directly")
    lines.append("import Foundation")
    lines.append("")

    // Options structs for operations with optional query params or pagination
    let optionsStructs = emitOptionsStructs(service)
    if !optionsStructs.isEmpty {
        lines.append(contentsOf: optionsStructs)
        lines.append("")
    }

    // Service class
    lines.append("public final class \(service.className): BaseService, @unchecked Sendable {")

    let sortedOps = service.operations.sorted { $0.operationId < $1.operationId }
    for (i, op) in sortedOps.enumerated() {
        if i > 0 { lines.append("") }
        lines.append(contentsOf: emitMethod(op, serviceName: service.name, schemas: schemas))
    }

    lines.append("}")
    lines.append("")
    return lines.joined(separator: "\n")
}

// MARK: - Options Structs

private func emitOptionsStructs(_ service: ServiceDefinition) -> [String] {
    var lines: [String] = []
    var generated = Set<String>()

    let sortedOps = service.operations.sorted { $0.operationId < $1.operationId }
    for op in sortedOps {
        let optionalQueryParams = op.queryParams.filter { !$0.required }
        let needsOptions = !optionalQueryParams.isEmpty || (op.hasPagination && op.returnsArray)
        guard needsOptions else { continue }

        let structName = "\(capitalize(op.methodName))\(capitalize(singularize(service.name)))Options"
        guard !generated.contains(structName) else { continue }
        generated.insert(structName)

        lines.append("public struct \(structName): Sendable {")

        for param in optionalQueryParams {
            let camelName = toCamelCase(param.name)
            lines.append("    public var \(camelName): \(param.swiftType)?")
        }

        // Add maxItems for paginated operations
        if op.hasPagination && op.returnsArray {
            lines.append("    public var maxItems: Int?")
        }

        // Memberwise init
        var initParams: [String] = []
        for param in optionalQueryParams {
            let camelName = toCamelCase(param.name)
            initParams.append("\(camelName): \(param.swiftType)? = nil")
        }
        if op.hasPagination && op.returnsArray {
            initParams.append("maxItems: Int? = nil")
        }

        lines.append("")
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

        for param in optionalQueryParams {
            let camelName = toCamelCase(param.name)
            lines.append("        self.\(camelName) = \(camelName)")
        }
        if op.hasPagination && op.returnsArray {
            lines.append("        self.maxItems = maxItems")
        }
        lines.append("    }")
        lines.append("}")
        lines.append("")
    }

    return lines
}

// MARK: - Method Emission

private func emitMethod(_ op: ParsedOperation, serviceName: String, schemas: [String: Any]) -> [String] {
    var lines: [String] = []
    let resourceName = singularize(serviceName)

    // Build method signature
    let (paramString, hasOptions, hasRequest, _) = buildSignature(op, resourceName: resourceName, serviceName: serviceName)
    let returnType = buildReturnType(op, serviceName: serviceName, schemas: schemas)

    // Method signature
    if returnType == "Void" {
        lines.append("    public func \(op.methodName)(\(paramString)) async throws {")
    } else {
        lines.append("    public func \(op.methodName)(\(paramString)) async throws -> \(returnType) {")
    }

    let isPaginated = op.hasPagination && op.returnsArray

    // Build query items for ops with query params
    let optionalQueryParams = op.queryParams.filter { !$0.required }
    let requiredQueryParams = op.queryParams.filter { $0.required }
    let hasQueryItems = !op.queryParams.isEmpty

    if hasQueryItems && !isPaginated {
        // Non-paginated ops: build URL query string inline
        lines.append("        var queryItems: [URLQueryItem] = []")
        for q in requiredQueryParams {
            let camelName = toCamelCase(q.name)
            if q.swiftType == "Int" {
                lines.append("        queryItems.append(URLQueryItem(name: \"\(q.name)\", value: String(\(camelName))))")
            } else {
                lines.append("        queryItems.append(URLQueryItem(name: \"\(q.name)\", value: \(camelName)))")
            }
        }
        for q in optionalQueryParams {
            let camelName = toCamelCase(q.name)
            lines.append("        if let \(camelName) = options?.\(camelName) {")
            if q.swiftType == "Int" {
                lines.append("            queryItems.append(URLQueryItem(name: \"\(q.name)\", value: String(\(camelName))))")
            } else if q.swiftType == "Bool" {
                lines.append("            queryItems.append(URLQueryItem(name: \"\(q.name)\", value: String(\(camelName))))")
            } else {
                lines.append("            queryItems.append(URLQueryItem(name: \"\(q.name)\", value: \(camelName)))")
            }
            lines.append("        }")
        }
    }

    if isPaginated && hasQueryItems {
        lines.append("        var queryItems: [URLQueryItem] = []")
        for q in requiredQueryParams {
            let camelName = toCamelCase(q.name)
            if q.swiftType == "Int" {
                lines.append("        queryItems.append(URLQueryItem(name: \"\(q.name)\", value: String(\(camelName))))")
            } else {
                lines.append("        queryItems.append(URLQueryItem(name: \"\(q.name)\", value: \(camelName)))")
            }
        }
        for q in optionalQueryParams {
            let camelName = toCamelCase(q.name)
            lines.append("        if let \(camelName) = options?.\(camelName) {")
            if q.swiftType == "Int" {
                lines.append("            queryItems.append(URLQueryItem(name: \"\(q.name)\", value: String(\(camelName))))")
            } else if q.swiftType == "Bool" {
                lines.append("            queryItems.append(URLQueryItem(name: \"\(q.name)\", value: String(\(camelName))))")
            } else {
                lines.append("            queryItems.append(URLQueryItem(name: \"\(q.name)\", value: \(camelName)))")
            }
            lines.append("        }")
        }
    }

    // Build OperationInfo
    let swiftPath = pathToSwiftInterpolation(op.path)
    let projectParam = op.pathParams.first { $0.name == "projectId" }
    let resourceParam = op.pathParams.first { $0.name != "projectId" && $0.name.hasSuffix("Id") }

    var opInfoParts: [String] = []
    opInfoParts.append("service: \"\(serviceName)\"")
    opInfoParts.append("operation: \"\(op.operationId)\"")
    opInfoParts.append("resourceType: \"\(op.resourceType)\"")
    opInfoParts.append("isMutation: \(op.isMutation)")
    if projectParam != nil {
        opInfoParts.append("projectId: projectId")
    }
    if let rp = resourceParam {
        opInfoParts.append("resourceId: \(rp.name)")
    }

    let opInfoStr = opInfoParts.joined(separator: ", ")

    if isPaginated {
        // requestPaginated call
        lines.append("        return try await requestPaginated(")
        lines.append("            OperationInfo(\(opInfoStr)),")
        lines.append("            path: \"\(swiftPath)\",")
        if hasQueryItems {
            lines.append("            queryItems: queryItems.isEmpty ? nil : queryItems,")
        }
        if hasOptions {
            lines.append("            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },")
        }
        lines.append("            retryConfig: Metadata.retryConfig(for: \"\(op.operationId)\")")
        lines.append("        )")
    } else if op.returnsVoid {
        // requestVoid call
        lines.append("        try await requestVoid(")
        lines.append("            OperationInfo(\(opInfoStr)),")
        lines.append("            method: \"\(op.httpMethod)\",")
        if hasQueryItems {
            lines.append("            path: \"\(swiftPath)\" + queryString(queryItems),")
        } else {
            lines.append("            path: \"\(swiftPath)\",")
        }
        if hasRequest {
            lines.append("            body: req,")
        }
        if op.bodyContentType == .octetStream {
            lines.append("            body: data,")
            lines.append("            contentType: contentType,")
        }
        lines.append("            retryConfig: Metadata.retryConfig(for: \"\(op.operationId)\")")
        lines.append("        )")
    } else {
        // request<T> call
        lines.append("        return try await request(")
        lines.append("            OperationInfo(\(opInfoStr)),")
        lines.append("            method: \"\(op.httpMethod)\",")
        if hasQueryItems {
            lines.append("            path: \"\(swiftPath)\" + queryString(queryItems),")
        } else {
            lines.append("            path: \"\(swiftPath)\",")
        }
        if hasRequest {
            lines.append("            body: req,")
        }
        if op.bodyContentType == .octetStream {
            lines.append("            body: data,")
            lines.append("            contentType: contentType,")
        }
        lines.append("            retryConfig: Metadata.retryConfig(for: \"\(op.operationId)\")")
        lines.append("        )")
    }

    lines.append("    }")
    return lines
}

// MARK: - Signature Building

private func buildSignature(
    _ op: ParsedOperation, resourceName: String, serviceName: String
) -> (paramString: String, hasOptions: Bool, hasRequest: Bool, optionsName: String) {
    var params: [String] = []
    var hasOptions = false
    var hasRequest = false

    let optionsName = "\(capitalize(op.methodName))\(capitalize(singularize(serviceName)))Options"
    let requestTypeName = requestTypeNameForSchema(op.bodySchemaRef)

    // Path params
    for p in op.pathParams {
        params.append("\(p.name): \(p.swiftType)")
    }

    // Body params (JSON)
    if op.bodySchemaRef != nil && !op.bodyProperties.isEmpty && op.bodyContentType == .json {
        params.append("req: \(requestTypeName)")
        hasRequest = true
    }

    // Binary upload
    if op.bodyContentType == .octetStream {
        params.append("data: Data")
        params.append("contentType: String")
    }

    // Required query params
    let requiredQueryParams = op.queryParams.filter { $0.required }
    for q in requiredQueryParams {
        let camelName = toCamelCase(q.name)
        params.append("\(camelName): \(q.swiftType)")
    }

    // Optional query params / pagination → options struct
    let optionalQueryParams = op.queryParams.filter { !$0.required }
    if !optionalQueryParams.isEmpty || (op.hasPagination && op.returnsArray) {
        params.append("options: \(optionsName)? = nil")
        hasOptions = true
    }

    return (params.joined(separator: ", "), hasOptions, hasRequest, optionsName)
}

/// Derives a clean request type name from a schema ref.
func requestTypeNameForSchema(_ schemaRef: String?) -> String {
    guard let ref = schemaRef else { return "Void" }
    var name = ref
    if name.hasSuffix("Content") {
        name = String(name.dropLast("Content".count))
    }
    if !name.hasSuffix("Request") && !name.hasSuffix("Payload") {
        name += "Request"
    }
    return name
}

private func buildReturnType(_ op: ParsedOperation, serviceName: String, schemas: [String: Any]) -> String {
    if op.returnsVoid { return "Void" }

    if let responseRef = op.responseSchemaRef {
        let entityName = getEntityTypeName(responseRef, schemas: schemas)
        if let name = entityName {
            if op.returnsArray && op.hasPagination { return "ListResult<\(name)>" }
            return op.returnsArray ? "[\(name)]" : name
        }

        // Fallback: resolve through the response content schema
        if let schema = schemas[responseRef] as? [String: Any] {
            if let ref = schema["$ref"] as? String {
                let refName = resolveRef(ref)
                if op.returnsArray && op.hasPagination { return "ListResult<\(refName)>" }
                return op.returnsArray ? "[\(refName)]" : refName
            }
            if (schema["type"] as? String) == "array",
               let items = schema["items"] as? [String: Any],
               let ref = items["$ref"] as? String {
                let refName = resolveRef(ref)
                if op.hasPagination { return "ListResult<\(refName)>" }
                return "[\(refName)]"
            }
        }

        // Last fallback: use schema name directly
        return responseRef
    }

    return "Void"
}

private func resolveReturnEntityType(_ op: ParsedOperation, serviceName: String, schemas: [String: Any]) -> String {
    if let responseRef = op.responseSchemaRef,
       let entityName = getEntityTypeName(responseRef, schemas: schemas) {
        return entityName
    }
    return "Any"
}
