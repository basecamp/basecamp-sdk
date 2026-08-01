/// The `delayBetweenRequests` assertion contract, kept in its own SDK-free
/// target so its bounds branches are unit-testable (`ConformanceSupportTests`).
/// An executable target carrying `@main` cannot host XCTest cleanly, and these
/// branches never execute against a fixture that passes.

/// Validates one assertion against the recorded request times, returning `nil`
/// when it holds and a failure message otherwise.
///
/// Gap i is the interval between request i and request i+1, so N requests yield
/// N-1 gaps. The contract in conformance/schema.json:
///
/// - A NAMED index selects exactly that gap, bounds-checked unconditionally. A
///   gap the run never produced is a failure, not a silent pass — the whole
///   point of a timing pin is to catch a dropped backoff, and a dropped backoff
///   is precisely what removes the gap.
/// - An OMITTED index requires the minimum on EVERY gap. Zero gaps means
///   nothing was measured, so that fails too: an "every gap" rule with no gaps
///   left would otherwise wave through a run that dropped every retry.
/// - Negative indexes are rejected rather than wrapping to the end the way the
///   per-request assertions do. There is no sensible "last gap" when the point
///   of naming one is to pin a specific backoff.
///
/// The bounds test compares against the gap COUNT and never adds one to the
/// index: `index + 1 >= requestTimes.count` overflows for `Int.max`, which in
/// Swift traps and takes the whole runner down instead of failing the
/// assertion — the same shape that read out of bounds in Go and Kotlin.
public func checkDelayGaps(
    _ requestTimes: [UInt64],
    minDelayMs: Double?,
    index: Int?
) -> String? {
    // An absent or zero minimum still asserts that the gap EXISTS. The default
    // lands HERE rather than at the call site so a truthiness gate cannot
    // quietly reduce the assertion to nothing.
    let minimum = minDelayMs ?? 0
    let gaps = requestTimes.count - 1

    func shortfall(_ gap: Int) -> String? {
        // Saturating rather than wrapping: the capture clock is monotonic, so a
        // decreasing pair cannot happen, and if it ever did a 0ms gap fails a
        // positive minimum — the fail-closed direction. Unsigned subtraction
        // traps on underflow, which would crash the runner instead.
        let later = requestTimes[gap + 1]
        let earlier = requestTimes[gap]
        let delay = later >= earlier ? later - earlier : 0
        return Double(delay) < minimum
            ? "Expected delay >= \(formatMs(minimum))ms at gap \(gap), got \(delay)ms"
            : nil
    }

    if let index {
        if index < 0 {
            return "delayBetweenRequests gap index must be non-negative, got \(index)"
        }
        if index >= gaps {
            return "Expected a delay at gap \(index), but only \(requestTimes.count) request(s) were made"
        }
        return shortfall(index)
    }

    if gaps < 1 {
        return "Expected a delay between requests, but only \(requestTimes.count) request(s) were made"
    }
    for gap in 0..<gaps {
        if let failure = shortfall(gap) { return failure }
    }
    return nil
}

/// Renders an integral minimum without a trailing `.0`. The schema types `min`
/// as a number, but every fixture states whole milliseconds and "1000ms" reads
/// better than "1000.0ms" in a failure message.
private func formatMs(_ value: Double) -> String {
    value == value.rounded() && value.magnitude < 1e15 ? String(Int64(value)) : String(value)
}
