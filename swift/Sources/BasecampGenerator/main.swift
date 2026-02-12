import Foundation

// MARK: - CLI

func printError(_ message: String) {
    FileHandle.standardError.write(Data(message.utf8))
}

func usage() -> Never {
    printError("""
        Usage: BasecampGenerator [options]
          --openapi <path>    OpenAPI spec (default: ../openapi.json)
          --behavior <path>   Behavior model (default: ../behavior-model.json)
          --output <path>     Output directory (default: Sources/Basecamp/Generated)

        """)
    exit(1)
}

@MainActor
func run() throws {
    let args = CommandLine.arguments
    var openapiPath = "../openapi.json"
    var behaviorPath = "../behavior-model.json"
    var outputDir = "Sources/Basecamp/Generated"

    var i = 1
    while i < args.count {
        switch args[i] {
        case "--openapi":
            i += 1
            guard i < args.count else { usage() }
            openapiPath = args[i]
        case "--behavior":
            i += 1
            guard i < args.count else { usage() }
            behaviorPath = args[i]
        case "--output":
            i += 1
            guard i < args.count else { usage() }
            outputDir = args[i]
        default:
            printError("Unknown argument: \(args[i])\n")
            usage()
        }
        i += 1
    }

    let fm = FileManager.default

    func resolvePath(_ path: String) -> String {
        if path.hasPrefix("/") { return path }
        return fm.currentDirectoryPath + "/" + path
    }

    let resolvedOpenAPI = resolvePath(openapiPath)
    let resolvedBehavior = resolvePath(behaviorPath)
    let resolvedOutput = resolvePath(outputDir)

    // MARK: - Load inputs

    guard let openapiData = fm.contents(atPath: resolvedOpenAPI) else {
        printError("Error: OpenAPI file not found: \(resolvedOpenAPI)\n")
        exit(1)
    }

    guard let behaviorData = fm.contents(atPath: resolvedBehavior) else {
        printError("Error: Behavior model not found: \(resolvedBehavior)\n")
        exit(1)
    }

    guard let spec = try? JSONSerialization.jsonObject(with: openapiData) as? [String: Any] else {
        printError("Error: Failed to parse OpenAPI JSON\n")
        exit(1)
    }

    // MARK: - Parse

    let (operations, schemas) = parseAllOperations(spec: spec)
    let retryConfigs = try parseBehaviorModel(data: behaviorData)
    let services = groupOperations(operations, schemas: schemas)
    let (entitySchemaNames, requestSchemaNames) = collectModelSchemas(operations: operations, schemas: schemas)

    print("Parsed \(operations.count) operations into \(services.count) services")
    print("Found \(entitySchemaNames.count) entity schemas, \(requestSchemaNames.count) request schemas")
    print("Loaded \(retryConfigs.count) retry configurations")

    // MARK: - Clean + create output directories

    let modelsDir = resolvedOutput + "/Models"
    let servicesDir = resolvedOutput + "/Services"

    // Remove stale generated files before writing
    for dir in [modelsDir, servicesDir] {
        if fm.fileExists(atPath: dir) {
            try fm.removeItem(atPath: dir)
        }
    }

    try fm.createDirectory(atPath: modelsDir, withIntermediateDirectories: true)
    try fm.createDirectory(atPath: servicesDir, withIntermediateDirectories: true)

    // MARK: - Emit entity models

    var entityCount = 0
    for schemaName in entitySchemaNames {
        let code = emitEntityModel(schemaName: schemaName, schemas: schemas)
        if code.isEmpty { continue }

        let typeName = typeAliases[schemaName]?.name ?? schemaName
        let filePath = modelsDir + "/\(typeName).swift"
        try code.write(toFile: filePath, atomically: true, encoding: .utf8)
        entityCount += 1
    }
    print("Generated \(entityCount) entity models")

    // MARK: - Emit request models

    var requestCount = 0
    for schemaName in requestSchemaNames {
        let code = emitRequestModel(schemaName: schemaName, schemas: schemas)
        if code.isEmpty { continue }

        var typeName = schemaName
        if typeName.hasSuffix("Content") {
            typeName = String(typeName.dropLast("Content".count))
        }
        if !typeName.hasSuffix("Request") && !typeName.hasSuffix("Payload") {
            typeName += "Request"
        }

        let filePath = modelsDir + "/\(typeName).swift"
        try code.write(toFile: filePath, atomically: true, encoding: .utf8)
        requestCount += 1
    }
    print("Generated \(requestCount) request models")

    // MARK: - Emit service files

    for (_, service) in services.sorted(by: { $0.key < $1.key }) {
        let code = emitService(service, schemas: schemas)
        let filePath = servicesDir + "/\(service.className).swift"
        try code.write(toFile: filePath, atomically: true, encoding: .utf8)
        print("Generated \(service.className) (\(service.operations.count) operations)")
    }

    // MARK: - Emit AccountClient+Services.swift

    let extensionCode = emitAccountClientExtension(services: services)
    let extensionPath = resolvedOutput + "/AccountClient+Services.swift"
    try extensionCode.write(toFile: extensionPath, atomically: true, encoding: .utf8)
    print("Generated AccountClient+Services.swift")

    // MARK: - Emit Metadata.swift

    let metadataCode = emitMetadata(configs: retryConfigs)
    let metadataPath = resolvedOutput + "/Metadata.swift"
    try metadataCode.write(toFile: metadataPath, atomically: true, encoding: .utf8)
    print("Generated Metadata.swift")

    // MARK: - Emit queryString helper

    let queryStringHelper = """
    // @generated from OpenAPI spec \u{2014} do not edit directly
    import Foundation

    /// Builds a URL query string from an array of URLQueryItem.
    func queryString(_ items: [URLQueryItem]) -> String {
        guard !items.isEmpty else { return "" }
        var components = URLComponents()
        components.queryItems = items
        return "?" + (components.query ?? "")
    }
    """
    let queryStringPath = resolvedOutput + "/QueryString.swift"
    try queryStringHelper.write(toFile: queryStringPath, atomically: true, encoding: .utf8)

    // MARK: - Summary

    let totalOps = services.values.reduce(0) { $0 + $1.operations.count }
    print("\nGenerated \(services.count) services with \(totalOps) operations total")
    print("Output directory: \(resolvedOutput)")
}

try run()
