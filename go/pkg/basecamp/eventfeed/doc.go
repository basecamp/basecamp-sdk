// Package eventfeed is the SPEC.md §23 Event Feed connector for Go: one
// blessed client for BC3's account-wide event feed, consumed as a serial
// stream of deduplicated events plus lifecycle observability.
//
// # Experimental
//
// This package is experimental and incomplete. The connector run loop, the
// WebSocket transport, and the Layer-1 adapters over the generated
// CreateStreamTicket and PollEvents operations have not landed yet. Until
// they do, the package exposes the connector's pure kernel — the Event wire
// shape, filter validation, the srv1 filter digest and checkpoint identity,
// the two-lane retry timing, the delivered-id LRU, the terminal taxonomy,
// and the seam interfaces — with no I/O of any kind. The exported surface may
// still change as those layers land.
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
