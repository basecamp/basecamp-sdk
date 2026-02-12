import Foundation

// MARK: - AccountClient Extension Emitter

/// Emits `AccountClient+Services.swift` with lazy service accessors.
func emitAccountClientExtension(services: [String: ServiceDefinition]) -> String {
    var lines: [String] = []

    lines.append("// @generated from OpenAPI spec \u{2014} do not edit directly")
    lines.append("import Foundation")
    lines.append("")
    lines.append("extension AccountClient {")

    // Sort services alphabetically for deterministic output
    for name in services.keys.sorted() {
        let service = services[name]!
        let propName = lowercaseFirst(name)
        lines.append("    public var \(propName): \(service.className) { service(\"\(propName)\") { \(service.className)(accountClient: self) } }")
    }

    lines.append("}")
    lines.append("")
    return lines.joined(separator: "\n")
}
