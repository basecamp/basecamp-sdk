import Foundation

/// Rendering a fixture's declared `path` template against its `pathParams`,
/// so the runner can hold an operation to the endpoint the fixture names.

/// The outcome of rendering a path template.
public enum RenderedPath: Equatable, Sendable {
    case rendered(String)
    /// A `{placeholder}` the params did not supply. Fail-closed: an
    /// unsubstituted template can never equal a real request path, but saying
    /// WHICH parameter is missing is the difference between a fixable fixture
    /// and a puzzling mismatch.
    case unsubstituted(String)
}

/// Substitutes `{name}` placeholders in a fixture path template.
///
/// Values arrive already stringified by the caller, because a path parameter
/// that is neither a string nor an integer is a fixture bug the dispatch
/// accessors reject first — this function only has to render what survived.
public func renderFixturePath(_ template: String, _ params: [String: String]) -> RenderedPath {
    var out = template
    for (name, value) in params {
        out = out.replacingOccurrences(of: "{\(name)}", with: value)
    }
    if let leftover = firstPlaceholder(in: out) {
        return .unsubstituted(leftover)
    }
    return .rendered(out)
}

/// Names the first `{...}` still present, or nil when the template is fully
/// rendered. Scans rather than using a regex so the target stays dependency-free.
private func firstPlaceholder(in path: String) -> String? {
    guard let open = path.firstIndex(of: "{") else { return nil }
    let afterOpen = path.index(after: open)
    guard let close = path[afterOpen...].firstIndex(of: "}") else { return nil }
    return String(path[afterOpen..<close])
}

/// Whether an observed request path matches a rendered fixture path.
///
/// The SDK prefixes the account id, and fixtures state the account-relative
/// path — except the download fixtures, whose path is already absolute because
/// the SDK dials a literal URL there. Both forms are accepted.
///
/// EXACT on both, never a suffix test: `/999/my/projects.json` ends with
/// `/projects.json`, so a suffix match would wave through an operation that
/// hit a neighbouring endpoint — the very thing this invariant exists to catch.
public func requestPathMatches(_ actual: String, fixturePath: String, accountID: String) -> Bool {
    actual == fixturePath || actual == "/\(accountID)\(fixturePath)"
}
