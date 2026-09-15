import Foundation

/// Decodes the subset of Ruby's Marshal 4.8 format a SignedGlobalID payload
/// uses — nil, booleans, fixnums, strings (with their encoding ivars), symbols
/// and symbol links, arrays and hashes — into plain Swift values:
/// `[String: Any]`, `[Any]`, `String`, `Int`, `Bool`, `NSNull`. Anything else is
/// an error; the caller then treats the sgid as undecodable rather than guessing.
///
/// This exists only for ``Mentions``: an `attachable_sgid` carries a Marshal
/// envelope, and reading the person id out of it structurally is what keeps a
/// Person gid that merely *appears* inside some other value from counting as a
/// mention. It is not a general-purpose Marshal reader and should not become one.
enum RubyMarshal {
    enum Failure: Error, CustomStringConvertible {
        case malformed(String)

        var description: String {
            switch self {
            case .malformed(let why): "marshal: \(why)"
            }
        }
    }

    /// Bounds nesting in a payload; an envelope is two deep.
    static let maxDepth = 32

    /// Decodes exactly one value. Trailing bytes are corruption, not a second
    /// value: a Marshal dump is one value and nothing else.
    static func decode(_ data: [UInt8]) throws -> Any {
        var reader = Reader(data)
        let value = try reader.value(depth: 0)
        guard reader.position == data.count else {
            throw Failure.malformed("\(data.count - reader.position) trailing bytes")
        }
        return value
    }

    private struct Reader {
        private let data: [UInt8]
        private(set) var position = 0
        private var symbols: [String] = []

        init(_ data: [UInt8]) { self.data = data }

        private mutating func byte() throws -> UInt8 {
            guard position < data.count else { throw Failure.malformed("unexpected end of data") }
            defer { position += 1 }
            return data[position]
        }

        /// Takes the next `n` bytes. The bound is checked against what remains,
        /// never by adding `n` to the position: a hostile length near `Int.max`
        /// would overflow that addition and trap. Every length reaches here
        /// through ``count()``, which already refused anything past the
        /// remaining bytes.
        private mutating func bytes(_ n: Int) throws -> ArraySlice<UInt8> {
            guard n >= 0, n <= data.count - position else {
                throw Failure.malformed("unexpected end of data")
            }
            defer { position += n }
            return data[position..<(position + n)]
        }

        /// Reads Marshal's packed integer: 0 is 0; 1...4 and -1...-4 are a byte
        /// count for a little-endian value; anything else is the value itself
        /// offset by 5.
        private mutating func integer() throws -> Int {
            let lead = try byte()
            // The lead byte is a signed int8. Widen it into its signed meaning;
            // this never narrows, so there is no overflow to guard.
            var c = Int(lead)
            if c > 127 { c -= 256 }

            if c == 0 { return 0 }
            if c > 4 { return c - 5 }
            if c < -4 { return c + 5 }
            if c > 0 {
                var x = 0
                for (i, v) in try bytes(c).enumerated() { x |= Int(v) << (8 * i) }
                return x
            }
            var x = -1
            for (i, v) in try bytes(-c).enumerated() {
                x &= ~(0xff << (8 * i))
                x |= Int(v) << (8 * i)
            }
            return x
        }

        /// Reads a length or count — string and symbol bytes, array elements,
        /// hash pairs, ivar pairs — and refuses one that cannot be honest:
        /// negative, or more than the bytes left (every element takes at least
        /// one byte). Allocation follows what actually decodes, so a hostile
        /// count costs its own bytes to refuse, never the capacity it claims.
        private mutating func count() throws -> Int {
            let n = try integer()
            guard n >= 0, n <= data.count - position else {
                throw Failure.malformed("bad count \(n)")
            }
            return n
        }

        mutating func value(depth: Int) throws -> Any {
            guard depth <= RubyMarshal.maxDepth else { throw Failure.malformed("nesting too deep") }

            switch try byte() {
            case UInt8(ascii: "0"):
                return NSNull()
            case UInt8(ascii: "T"):
                return true
            case UInt8(ascii: "F"):
                return false
            case UInt8(ascii: "i"):
                return try integer()
            case UInt8(ascii: "\""):
                let n = try count()
                return String(decoding: try bytes(n), as: UTF8.self)
            case UInt8(ascii: ":"):
                let n = try count()
                let symbol = String(decoding: try bytes(n), as: UTF8.self)
                symbols.append(symbol)
                return symbol
            case UInt8(ascii: ";"):
                let index = try integer()
                guard index >= 0, index < symbols.count else {
                    throw Failure.malformed("bad symbol link")
                }
                return symbols[index]
            case UInt8(ascii: "I"):
                // An object followed by its instance variables — a String's
                // encoding. The ivars are read to advance past them and dropped.
                let inner = try value(depth: depth + 1)
                let pairs = try count()
                for _ in 0..<pairs {
                    _ = try value(depth: depth + 1)  // ivar name
                    _ = try value(depth: depth + 1)  // ivar value
                }
                return inner
            case UInt8(ascii: "["):
                let n = try count()
                var out: [Any] = []
                for _ in 0..<n { out.append(try value(depth: depth + 1)) }
                return out
            case UInt8(ascii: "{"):
                let n = try count()
                var out: [String: Any] = [:]
                for _ in 0..<n {
                    let key = try value(depth: depth + 1)
                    let element = try value(depth: depth + 1)
                    guard let key = key as? String else {
                        throw Failure.malformed("non-string hash key")
                    }
                    out[key] = element
                }
                return out
            case let other:
                throw Failure.malformed("unsupported type \"\(Character(Unicode.Scalar(other)))\"")
            }
        }
    }
}
