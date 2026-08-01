/// Resolves a per-request assertion index against the number of recorded
/// requests, returning `nil` when it names no request that was made.
///
/// 0-based, and NEGATIVE INDEXES WRAP: -1 is the last request, -2 the one
/// before it. That is the opposite of `checkDelayGaps`, which rejects a
/// negative index outright — the two contracts share a fixture key (`index`)
/// and are easy to conflate, so both are pinned by tests.
///
/// The difference is deliberate and stated in conformance/schema.json: "the
/// last request" is a meaningful thing to assert a header on, whereas naming a
/// gap exists to pin one specific backoff, and "the last backoff" would let a
/// dropped retry re-target the assertion at a gap that did survive.
///
/// Out of range is nil rather than a clamp, so the caller fails the assertion
/// instead of validating a different request than the fixture asked for.
///
/// Unlike the delay-gap bounds test, `count + index` needs no overflow guard:
/// `count` is a non-negative array length and the branch only runs for a
/// negative `index`, so the sum always moves toward zero. These tests are
/// characterization, not a regression proof — they pin a contract that was
/// already correct, next to one that was not.
public func resolveRequestIndex(_ index: Int, _ count: Int) -> Int? {
    let resolved = index < 0 ? count + index : index
    return (0..<count).contains(resolved) ? resolved : nil
}
