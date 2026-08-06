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

/// Whether a fixture states a path the SDK dials literally, rather than one to
/// be scoped to the account. Only the download flows do — their path already
/// carries an account segment.
///
/// The test is a leading numeric segment. No account-relative fixture path
/// begins with one; they all start with a resource name (`/projects.json`,
/// `/buckets/...`, `/my/...`). If one ever did, this fails closed: the runner
/// would demand the literal form, receive the scoped form, and say so.
public func fixtureIsAccountScoped(_ fixturePath: String) -> Bool {
    guard fixturePath.hasPrefix("/") else { return false }
    let rest = fixturePath.dropFirst()
    let digits = rest.prefix(while: \.isNumber)
    return !digits.isEmpty && rest.dropFirst(digits.count).hasPrefix("/")
}

/// Whether an observed request path matches a rendered fixture path.
///
/// The form is decided by the FIXTURE, not by accepting whichever the SDK
/// happened to send. Accepting either let an unscoped request through: a
/// regression that dropped the account prefix and asked for `/projects.json`
/// satisfied a fixture meaning `/999/projects.json`, and the transport serves
/// any URL, so nothing else noticed.
///
/// EXACT, never a suffix test: `/999/my/projects.json` ends with
/// `/projects.json`, so a suffix match would wave through an operation that
/// hit a neighbouring endpoint — the very thing this invariant exists to catch.
public func requestPathMatches(_ actual: String, fixturePath: String, accountID: String) -> Bool {
    fixtureIsAccountScoped(fixturePath)
        ? actual == fixturePath
        : actual == "/\(accountID)\(fixturePath)"
}

/// The expected request path for a fixture path, for failure messages.
public func expectedRequestPath(_ fixturePath: String, accountID: String) -> String {
    fixtureIsAccountScoped(fixturePath) ? fixturePath : "/\(accountID)\(fixturePath)"
}

/// Extracts the `rel="next"` target from a `Link` header value, or nil when the
/// header names no next page.
///
/// Deliberately tolerant of the surrounding syntax (multiple comma-separated
/// links, arbitrary parameter order and spacing) and strict about the target
/// itself, which is returned verbatim between the angle brackets.
///
/// The bracket extraction follows SPEC "Link Header Parsing Algorithm" rather
/// than testing `hasPrefix("<") && hasSuffix(">")`. This function decides which
/// requests the LINK and PATH invariants govern, so a parser here that is
/// stricter than the SDKs' does not merely fail to notice a bug — it INVENTS
/// one: a header the SDK reads correctly (`>x</projects.json?page=2>`) reads as
/// "no next link" here, the follow-up request loses its exemption from the
/// PATH invariant, and a correct SDK fails on a link it followed exactly as
/// the fixture asked. A parser that is more permissive is just as bad in the
/// other direction: an empty `<>` accepted as a target makes the LINK
/// invariant demand a fetch of "".
public func nextLinkTarget(_ headerValue: String) -> String? {
    for link in headerValue.split(separator: ",") {
        let parts = link.split(separator: ";")
        guard let head = parts.first?.trimmingCharacters(in: .whitespaces),
              let target = angleBracketedTarget(head),
              parts.dropFirst().contains(where: { isRelNext($0) })
        else { continue }
        return target
    }
    return nil
}

/// The leftmost non-empty `<…>` span in `part`, matching `/<([^>]+)>/`.
///
/// Mirrors `extractAngleBracketed` in each SDK: scan for `<`, then for the
/// first `>` AFTER it (never from position 0, which is both quadratic and
/// wrong when a `>` precedes the `<`), and skip an empty `<>` because `[^>]+`
/// requires at least one character.
func angleBracketedTarget(_ part: String) -> String? {
    var cursor = part.startIndex

    while let start = part[cursor...].firstIndex(of: "<") {
        let contentStart = part.index(after: start)
        guard let end = part[contentStart...].firstIndex(of: ">") else { return nil }

        if end > contentStart {
            return String(part[contentStart..<end])
        }
        cursor = contentStart
    }

    return nil
}

private func isRelNext(_ parameter: Substring) -> Bool {
    let cleaned = parameter
        .trimmingCharacters(in: .whitespaces)
        .replacingOccurrences(of: " ", with: "")
        .replacingOccurrences(of: "\"", with: "")
        .lowercased()
    return cleaned == "rel=next"
}
