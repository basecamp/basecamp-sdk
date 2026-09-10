import Foundation

/// A `{{…}}` header value the runner does not define. Surfaced as an error
/// rather than served literally: a typo'd token on the wire would be an
/// unparseable header, which the SDK answers with its ordinary backoff — the
/// exact outcome the case exists to distinguish from.
public struct UnrecognisedHeaderToken: Error, CustomStringConvertible, Sendable {
    public let value: String
    public var description: String {
        "unrecognised header token \"\(value)\": only {{httpdate+Ns}} is defined (conformance/schema.json)"
    }
}

/// Substitutes the one token a fixture header value may carry,
/// `{{httpdate+Ns}}` (SPEC §19, conformance/schema.json), at the moment the
/// response is served. Every other value passes through untouched.
///
/// The token resolves to the IMF-fixdate of floor(now) + N + 1 seconds: the
/// first whole second strictly more than N seconds after the second the
/// response is served in. A compliant SPEC §6 parser sees a remainder in
/// (N − latency, N + 1] and, rounding up, computes at least N whole seconds, so
/// the fixture pairs it with a `delayBetweenRequests` floor of N × 1000 ms. It
/// exists because a static fixture has no clock: a literal past date pins only
/// the fall-through, and a far-future one is differently behaved per host.
public func resolveHeaderValue(_ value: String, now: Date) throws -> String {
    guard value.hasPrefix("{{"), value.hasSuffix("}}"), value.count >= 4 else { return value }
    let inner = value.dropFirst(2).dropLast(2)
    let prefix = "httpdate+"
    guard inner.hasPrefix(prefix), inner.hasSuffix("s") else { throw UnrecognisedHeaderToken(value: value) }
    let digits = inner.dropFirst(prefix.count).dropLast()
    guard !digits.isEmpty, digits.allSatisfy({ $0.isASCII && $0.isNumber }), let n = Int(digits) else {
        throw UnrecognisedHeaderToken(value: value)
    }
    let seconds = floor(now.timeIntervalSince1970) + Double(n) + 1
    let formatter = DateFormatter()
    formatter.locale = Locale(identifier: "en_US_POSIX")
    formatter.timeZone = TimeZone(identifier: "GMT")
    formatter.dateFormat = "EEE, dd MMM yyyy HH:mm:ss 'GMT'"
    return formatter.string(from: Date(timeIntervalSince1970: seconds))
}
