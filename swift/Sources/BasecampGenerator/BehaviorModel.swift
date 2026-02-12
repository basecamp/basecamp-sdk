import Foundation

// MARK: - Behavior Model

struct BehaviorRetryConfig {
    let operationId: String
    let maxAttempts: Int
    let baseDelayMs: Int
    let backoff: String
    let retryOn: [Int]
}

/// Parses retry configurations from behavior-model.json.
func parseBehaviorModel(data: Data) throws -> [BehaviorRetryConfig] {
    guard let json = try JSONSerialization.jsonObject(with: data) as? [String: Any],
          let operations = json["operations"] as? [String: Any] else {
        return []
    }

    var configs: [BehaviorRetryConfig] = []

    for (operationId, value) in operations {
        guard let opDict = value as? [String: Any],
              let retry = opDict["retry"] as? [String: Any] else { continue }

        let maxAttempts = retry["max"] as? Int ?? 3
        let baseDelayMs = retry["base_delay_ms"] as? Int ?? 1000
        let retryOn = retry["retry_on"] as? [Int] ?? [429, 503]

        let allowedBackoffs: Set<String> = ["exponential", "linear", "constant"]
        let rawBackoff = retry["backoff"] as? String ?? "exponential"
        let backoff: String
        if allowedBackoffs.contains(rawBackoff) {
            backoff = rawBackoff
        } else {
            printError("Warning: unknown backoff '\(rawBackoff)' for \(operationId), defaulting to exponential\n")
            backoff = "exponential"
        }

        configs.append(BehaviorRetryConfig(
            operationId: operationId,
            maxAttempts: maxAttempts,
            baseDelayMs: baseDelayMs,
            backoff: backoff,
            retryOn: retryOn
        ))
    }

    return configs.sorted { $0.operationId < $1.operationId }
}
