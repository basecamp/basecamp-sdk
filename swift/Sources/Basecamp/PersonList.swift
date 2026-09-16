import Foundation

/// A generated model the reference decodes as a person: its id is a required
/// ``FlexibleInt`` (`generated.Person.Id` is `types.FlexibleInt64`).
///
/// The generator emits the conformance. ``zero`` is Go's zero struct — id `0`,
/// every other required member at its zero value — which is what
/// `encoding/json` leaves for a `null` element of a `[]Person`.
protocol ZeroPerson: Decodable {
    static var zero: Self { get }
}

extension KeyedDecodingContainer {
    /// Reads a list of people the way the reference does: a `null` element is
    /// the zero person rather than a failed read, so a merge-safe write that
    /// reads the list back sends `0` for it, as Go's does.
    ///
    /// Only the element is lenient. An element that is not an object still
    /// fails, and so does an element whose id is an explicit `null`. The
    /// generator emits this for exactly the members whose element type is a
    /// person; a person shape whose id is a plain `int64` in Go keeps the strict
    /// synthesized decode.
    func decodePeople<P: ZeroPerson>(_ type: [P].Type, forKey key: Key) throws -> [P] {
        try decode([P?].self, forKey: key).map { $0 ?? P.zero }
    }

    /// ``decodePeople(_:forKey:)`` for an optional list: an absent key or a
    /// `null` list is `nil`.
    func decodePeopleIfPresent<P: ZeroPerson>(_ type: [P].Type, forKey key: Key) throws -> [P]? {
        try decodeIfPresent([P?].self, forKey: key)?.map { $0 ?? P.zero }
    }
}
