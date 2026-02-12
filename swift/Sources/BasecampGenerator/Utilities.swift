import Foundation

// MARK: - String Utilities

/// Converts a snake_case string to camelCase.
func toCamelCase(_ str: String) -> String {
    var result = ""
    var capitalizeNext = false
    for ch in str {
        if ch == "_" {
            capitalizeNext = true
        } else if capitalizeNext {
            result.append(ch.uppercased().first!)
            capitalizeNext = false
        } else {
            result.append(ch)
        }
    }
    return result
}

/// Capitalizes the first character.
func capitalize(_ str: String) -> String {
    guard let first = str.first else { return str }
    return first.uppercased() + str.dropFirst()
}

/// Lowercases the first character.
func lowercaseFirst(_ str: String) -> String {
    guard let first = str.first else { return str }
    return first.lowercased() + str.dropFirst()
}

/// Naive singularization for service names.
func singularize(_ str: String) -> String {
    if str.hasSuffix("ies") { return String(str.dropLast(3)) + "y" }
    if str.hasSuffix("ses") { return String(str.dropLast(2)) }
    if str.hasSuffix("s") { return String(str.dropLast(1)) }
    return str
}

/// Converts PascalCase to kebab-case.
func toKebabCase(_ str: String) -> String {
    var result = ""
    for (i, ch) in str.enumerated() {
        if ch.isUppercase {
            if i > 0 {
                result.append("-")
            }
            result.append(ch.lowercased())
        } else {
            result.append(ch)
        }
    }
    return result
}

/// Converts a snake_case or camelCase name to a human-readable description.
func toHumanReadable(_ str: String) -> String {
    if str.hasSuffix("Id") {
        let base = String(str.dropLast(2))
        let spaced = base.replacingOccurrences(
            of: "([a-z])([A-Z])", with: "$1 $2",
            options: .regularExpression
        ).lowercased()
        return spaced + " ID"
    }
    return str
        .replacingOccurrences(of: "_", with: " ")
        .replacingOccurrences(
            of: "([a-z])([A-Z])", with: "$1 $2",
            options: .regularExpression
        )
        .lowercased()
}

/// Resolves a $ref string to the schema name (last path component).
func resolveRef(_ ref: String) -> String {
    ref.split(separator: "/").last.map(String.init) ?? ""
}

/// Strips `/{accountId}` prefix from an OpenAPI path.
func convertPath(_ path: String) -> String {
    if path.hasPrefix("/{accountId}") {
        return String(path.dropFirst("/{accountId}".count))
    }
    return path
}

/// Extracts the resource type from an operationId using verb patterns.
func extractResourceType(_ operationId: String) -> String {
    for (prefix, _) in verbPatterns {
        if operationId.hasPrefix(prefix) {
            let remainder = String(operationId.dropFirst(prefix.count))
            if remainder.isEmpty { return "resource" }
            // Convert PascalCase to snake_case
            var snakeCase = ""
            for (i, ch) in remainder.enumerated() {
                if ch.isUppercase && i > 0 {
                    snakeCase.append("_")
                }
                snakeCase.append(ch.lowercased())
            }
            // Naive de-pluralize: remove trailing 's' if not 'ss'
            if snakeCase.hasSuffix("s") && !snakeCase.hasSuffix("ss") {
                snakeCase = String(snakeCase.dropLast())
            }
            return snakeCase
        }
    }
    return "resource"
}

/// Converts an OpenAPI path to a Swift string interpolation.
///
/// `/{accountId}/buckets/{projectId}/todos/{todoId}.json`
/// → `"/buckets/\(projectId)/todos/\(todoId).json"`
func pathToSwiftInterpolation(_ path: String) -> String {
    // First strip the accountId prefix
    let stripped = convertPath(path)
    // Replace {paramName} with \(paramName)
    var result = stripped
    let regex = try! NSRegularExpression(pattern: "\\{([^}]+)\\}")
    let matches = regex.matches(in: stripped, range: NSRange(stripped.startIndex..., in: stripped))
    for match in matches.reversed() {
        let range = Range(match.range, in: stripped)!
        let paramRange = Range(match.range(at: 1), in: stripped)!
        let paramName = toCamelCase(String(stripped[paramRange]))
        result.replaceSubrange(range, with: "\\(\(paramName))")
    }
    return result
}
