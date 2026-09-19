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
          --verbs <path>      Generated-verb declaration (default: ../spec/generated-verbs.json)
          --output <path>     Output directory (default: Sources/Basecamp/Generated)

        """)
    exit(1)
}

@MainActor
func run() throws {
    let args = CommandLine.arguments
    var openapiPath = "../openapi.json"
    var behaviorPath = "../behavior-model.json"
    var verbsPath = "../spec/generated-verbs.json"
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
        case "--verbs":
            i += 1
            guard i < args.count else { usage() }
            verbsPath = args[i]
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
    let resolvedVerbs = resolvePath(verbsPath)
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

    let (operations, schemas) = parseAllOperations(
        spec: spec, emittableVerbs: loadGeneratedVerbs(path: resolvedVerbs))
    let retryConfigs = try parseBehaviorModel(data: behaviorData)
    let services = groupOperations(operations, schemas: schemas)
    let (discoveredEntitySchemaNames, requestSchemaNames) = collectModelSchemas(
        operations: operations, schemas: schemas)
    let entitySchemaNames = Array(
        Set(discoveredEntitySchemaNames).union(typeAliases.keys.filter { schemas[$0] != nil })
    ).sorted()

    print("Parsed \(operations.count) operations into \(services.count) services")
    print("Found \(entitySchemaNames.count) entity schemas, \(requestSchemaNames.count) request schemas")
    print("Loaded \(retryConfigs.count) retry configurations")

    // MARK: - Render everything, then clean, then write

    // NOTHING IS DELETED UNTIL EVERY FILE'S CONTENT EXISTS.
    //
    // The invariant is not "parse before delete": parsing is only one thing that
    // can refuse, and any code path able to fail after a delete is a data-loss
    // path regardless of what it is called. This generator used to remove
    // Models/ and Services/ and then emit, so a failure anywhere in emission
    // destroyed the committed tree. Rendering into memory first retires the
    // class rather than hoisting one named failure — the same shape
    // rust/generator/src/main.rs uses and the Kotlin generator now uses.

    let modelsDir = resolvedOutput + "/Models"
    let servicesDir = resolvedOutput + "/Services"

    // path -> content, in emission order.
    var rendered: [(path: String, code: String)] = []

    var entityCount = 0
    for schemaName in entitySchemaNames {
        let code = emitEntityModel(schemaName: schemaName, schemas: schemas)
        if code.isEmpty { continue }
        let typeName = typeAliases[schemaName]?.name ?? schemaName
        rendered.append((modelsDir + "/\(typeName).swift", code))
        entityCount += 1
    }
    print("Generated \(entityCount) entity models")

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
        rendered.append((modelsDir + "/\(typeName).swift", code))
        requestCount += 1
    }
    print("Generated \(requestCount) request models")

    for (_, service) in services.sorted(by: { $0.key < $1.key }) {
        rendered.append((servicesDir + "/\(service.className).swift", emitService(service, schemas: schemas)))
        print("Generated \(service.className) (\(service.operations.count) operations)")
    }

    rendered.append((resolvedOutput + "/AccountClient+Services.swift",
                     emitAccountClientExtension(services: services)))
    print("Generated AccountClient+Services.swift")

    rendered.append((resolvedOutput + "/Metadata.swift", emitMetadata(configs: retryConfigs)))
    print("Generated Metadata.swift")

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
    rendered.append((resolvedOutput + "/QueryString.swift", queryStringHelper))

    // Everything is rendered. ONLY NOW is anything destroyed.
    for dir in [modelsDir, servicesDir] where fm.fileExists(atPath: dir) {
        try fm.removeItem(atPath: dir)
    }
    try fm.createDirectory(atPath: modelsDir, withIntermediateDirectories: true)
    try fm.createDirectory(atPath: servicesDir, withIntermediateDirectories: true)

    for (path, code) in rendered {
        try code.write(toFile: path, atomically: true, encoding: .utf8)
    }

    // MARK: - Summary

    let totalOps = services.values.reduce(0) { $0 + $1.operations.count }
    print("\nGenerated \(services.count) services with \(totalOps) operations total")
    print("Output directory: \(resolvedOutput)")
}

try run()
