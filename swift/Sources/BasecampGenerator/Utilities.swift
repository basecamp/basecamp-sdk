import Foundation

// MARK: - Query Item Emission

/// Emits the `queryItems.append(...)` line(s) for a single query parameter.
///
/// Array-typed params (e.g. `bucket_ids[]`) expand into one `URLQueryItem` per
/// element, keyed by the raw bracketed wire name, so URLComponents serializes
/// `bucket_ids[]=1&bucket_ids[]=2`. Scalars emit a single item; Int/Bool values
/// are stringified.
func queryItemAppendLines(_ q: QueryParam, accessor: String, indent: String) -> [String] {
    if q.swiftType.hasPrefix("[") {
        // URLQueryItem(value:) wants String?; stringify every non-String
        // element type ([Int], [Bool], [Double], …). Only [String] passes through.
        let elementValue = q.swiftType == "[String]" ? "item" : "String(item)"
        return [
            "\(indent)for item in \(accessor) {",
            "\(indent)    queryItems.append(URLQueryItem(name: \"\(q.wireName)\", value: \(elementValue)))",
            "\(indent)}",
        ]
    }
    if q.swiftType == "Int" || q.swiftType == "Bool" {
        return ["\(indent)queryItems.append(URLQueryItem(name: \"\(q.wireName)\", value: String(\(accessor))))"]
    }
    return ["\(indent)queryItems.append(URLQueryItem(name: \"\(q.wireName)\", value: \(accessor)))"]
}

// MARK: - Deprecation Doc Comments

/// Renders a deprecation `reason` as one or more `///` doc-comment lines (#406).
///
/// A reason can be sourced from a multi-line OpenAPI description; interpolating
/// it into a single `///` line would leave any continuation on an un-commented
/// source line and break compilation. Splitting on line boundaries and
/// prefixing every line with `///` keeps the emitted Swift valid for any reason.
/// `leader` precedes the first reason line (e.g. `"Deprecated: "` or
/// `"- Parameter foo: Deprecated: "`); continuation lines carry no leader.
func deprecationDocLines(reason: String, indent: String, leader: String = "Deprecated: ") -> [String] {
    let reasonLines = reason.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
    let first = reasonLines.first ?? ""
    var out = ["\(indent)/// \(leader)\(first)"]
    for line in reasonLines.dropFirst() {
        out.append(line.isEmpty ? "\(indent)///" : "\(indent)/// \(line)")
    }
    return out
}

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

/// Singularization for service/type names (PascalCase or lowercase).
func singularize(_ str: String) -> String {
    if str.hasSuffix("sses") { return String(str.dropLast(2)) }  // glasses → glass
    if str.hasSuffix("ss") { return str }                         // progress, access → unchanged
    if str.hasSuffix("ies") { return String(str.dropLast(3)) + "y" }
    if str.hasSuffix("ses") { return String(str.dropLast(2)) }
    if str.hasSuffix("s") { return String(str.dropLast(1)) }
    return str
}

/// Singularizes a snake_case string by singularizing only the last segment.
///
/// `"schedule_entries"` → `"schedule_entry"`, `"client_replies"` → `"client_reply"`
func singularizeSnakeCase(_ str: String) -> String {
    guard let lastUnderscore = str.lastIndex(of: "_") else {
        return singularize(str)
    }
    let prefix = str[str.startIndex...lastUnderscore]
    let suffix = singularize(String(str[str.index(after: lastUnderscore)...]))
    return prefix + suffix
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

private let resourceTypeOverrides: [String: String] = [
    "SpotlightRecording": "recording",
    "UnspotlightRecording": "recording",
    "UpdateHillChartSettings": "hill_chart",
    // The whole family reports "bookmark"; the inferred "my_bookmark" would
    // split the list operation into its own telemetry category.
    "ListMyBookmarks": "bookmark",
    // Creates and returns a Todo; the inferred "todoset_todo" would split
    // loose-to-do operations into their own telemetry category.
    "CreateTodosetTodo": "todo",
    // "Destroy" is not a verb pattern, so inference falls through to the generic
    // "resource" and would split this delete away from the get/update siblings
    // that report "timesheet_entry".
    "DestroyTimesheetEntry": "timesheet_entry",
    // The whole family reports "project" (as the Go wrapper does); the inferred
    // "recent_project" would split the list operation into its own telemetry
    // category, and "Record" is not a verb pattern, so the visit would fall
    // through to the generic "resource".
    "ListRecentProjects": "project",
    "RecordProjectVisit": "project",
]

/// Extracts the resource type from an operationId using verb patterns.
func extractResourceType(_ operationId: String) -> String {
    if let override = resourceTypeOverrides[operationId] {
        return override
    }
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
            return singularizeSnakeCase(snakeCase)
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
