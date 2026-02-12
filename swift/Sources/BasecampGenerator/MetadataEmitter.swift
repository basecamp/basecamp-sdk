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
    lines.append("}")
    lines.append("")
    return lines.joined(separator: "\n")
}
