import Foundation

// MARK: - Metadata Emitter

/// Emits `Metadata.swift` with per-operation retry configurations.
func emitMetadata(configs: [BehaviorRetryConfig]) -> String {
    var lines: [String] = []

    lines.append("// @generated from behavior-model.json \u{2014} do not edit directly")
    lines.append("import Foundation")
    lines.append("")
    lines.append("enum Metadata {")
    lines.append("    private static let configs: [String: RetryConfig] = [")

    for config in configs {
        let retryOnStr = config.retryOn.sorted().map(String.init).joined(separator: ", ")
        lines.append("        \"\(config.operationId)\": RetryConfig(maxAttempts: \(config.maxAttempts), baseDelayMs: \(config.baseDelayMs), backoff: .\(config.backoff), retryOn: [\(retryOnStr)]),")
    }

    lines.append("    ]")
    lines.append("")
    lines.append("    static func retryConfig(for operationId: String) -> RetryConfig? {")
    lines.append("        configs[operationId]")
    lines.append("    }")
    lines.append("")

    // Operations whose effects are idempotent per behavior-model.json. These
    // POSTs stay retry-eligible even though POST is not naturally idempotent;
    // everything else keys retry off the HTTP method alone (see HTTPClient).
    let idempotentIds = configs.filter { $0.idempotent }.map { $0.operationId }.sorted()
    lines.append("    private static let idempotentOperations: Set<String> = [")
    for operationId in idempotentIds {
        lines.append("        \"\(operationId)\",")
    }
    lines.append("    ]")
    lines.append("")
    lines.append("    static func isIdempotent(for operationId: String) -> Bool {")
    lines.append("        idempotentOperations.contains(operationId)")
    lines.append("    }")
    lines.append("}")
    lines.append("")
    return lines.joined(separator: "\n")
}
