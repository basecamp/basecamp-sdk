// Package eventfeed is the SPEC.md §23 Event Feed connector for Go: one
// blessed client for BC3's account-wide event feed, consumed as a serial
// stream of deduplicated events plus lifecycle observability.
//
// # Experimental
//
// This package is experimental and incomplete, and its exported surface may
// still change. The run loop has landed: Connector, Events, Close and Wait
// play the protocol described below — the first mint through the catch-up
// walk, the entry boundary, the drain, streaming delivery, the repair poll,
// and the recovery matrix a poll's 410, 400-position, or 409 re-enters
// through — alongside the event and filter models, the seam interfaces and
// their error taxonomies, the Action Cable frame codecs, deduplication, the
// backoff and repair-jitter envelopes, the checkpoint store, the cable-URL
// policy and its WebSocket transport, and the deterministic fakes the
// conformance harness drives.
//
// ONE piece is still to land, and it is what keeps the package unusable
// against the live API: the Layer-1 adapters over the generated
// CreateStreamTicket and PollEvents operations that back the TicketMinter and
// PollSource seams. Until they exist, the seams have no production
// implementation and a host must supply its own. Two obligations ride on
// those adapters rather than on anything here — zero egress to a foreign
// redirect target (conformance/event-feed/README.md's row-15 note), and the
// no-automatic-redirect-following rule §23 places on PollEvents.
//
// # Seams-first architecture
//
// The connector performs no wire I/O of its own. Every HTTP exchange reaches
// the wire through a seam backed by a generated operation: TicketMinter
// (CreateStreamTicket) and PollSource (PollEvents), each call one
// fully-governed generated call. Time flows through the injected Clock,
// persistence through CheckpointStore, and the WebSocket through
// CableTransport — whose dial of the mint's URL, verbatim, is the one
// sanctioned non-HTTP wire act the connector owns. CableTransport and Clock
// are product surface, not test hooks: they are the documented extension
// points for custom WebSocket stacks and embedded runtimes, and the reason
// the conformance harness is deterministic.
//
// # Delivery contract
//
// Feed rows are wake-up signals — enough to route, not enough to act.
// Feed payloads are never current resource state; consumers refetch the
// referenced recording through canonical resource APIs before acting. The
// push lane may delay, duplicate, or drop; the poll lane's repair bound is
// position-relative and best-effort. The delivery promise is §23's
// conjunctive save-ordering invariant, scoped to the ownership cut it
// defines, and nothing stronger — completeness-critical work corroborates
// against canonical resource APIs.
package eventfeed
