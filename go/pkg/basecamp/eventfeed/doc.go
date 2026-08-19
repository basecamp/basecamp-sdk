// Package eventfeed is the SPEC.md §23 Event Feed connector for Go: one
// blessed client for BC3's account-wide event feed, consumed as a serial
// stream of deduplicated events plus lifecycle observability.
//
// # Experimental
//
// This package is experimental and incomplete, and today it is FOUNDATIONS
// ONLY: there is no consumer entry point yet — no Connector, no constructor,
// no run loop — so nothing here can be pointed at the live API. What has
// landed is the material a run loop is assembled from: the event and filter
// models, the seam interfaces and their error taxonomies, the Action Cable
// frame codecs, deduplication, the backoff and repair-jitter envelopes, the
// checkpoint store, the cable-URL policy and its WebSocket transport, and the
// deterministic fakes the conformance harness drives.
//
// Two pieces are still to land, and the package is not usable until both do:
// the run loop that plays the protocol described below — the first mint
// through the catch-up walk, the entry boundary, the drain, streaming
// delivery, the repair poll, and the recovery matrix a poll's 410,
// 400-position, or 409 re-enters through — and the Layer-1 adapters over the
// generated CreateStreamTicket and PollEvents operations that back the mint
// and poll seams. Everything below this section documents the architecture
// those pieces implement, not behavior this package performs today, and the
// exported surface may still change as they land.
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
