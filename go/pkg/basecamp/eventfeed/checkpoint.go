package eventfeed

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

// CheckpointKey is the durable checkpoint identity — all four fields, always
// (SPEC.md §23 "Checkpoint Identity"). Server positions are bound to
// {account, filter set} but carry no consumer identity; two independent
// consumers in one account would otherwise share a lineage and silently skip
// each other's work. Origin is included because the SDK supports configurable
// base URLs.
type CheckpointKey struct {
	// Origin is the canonicalized API base origin (CanonicalOrigin).
	Origin string
	// AccountID is the account the feed belongs to.
	AccountID string
	// ConsumerNamespace names the consumer's checkpoint lineage. Required
	// whenever a store is configured: a configured store with an empty
	// namespace fails construction with a usage-coded error.
	ConsumerNamespace string
	// FilterKey is "srv1-" + the bare 16-hex server digest
	// (Filters.FilterKey).
	FilterKey string
}

// FlatKey renders the identity as the compact RFC 8259 JSON array of the four
// strings — e.g.
//
//	["https://3.basecampapi.com","5951425","openclaw","srv1-9f2ab04e5c11d3a7"]
//
// JSON escaping removes all delimiter ambiguity: no bespoke path-joining, no
// percent-encoding. This is the flat key the file store (and any store keyed
// by a single string) uses.
func (k CheckpointKey) FlatKey() string {
	var b strings.Builder
	b.WriteByte('[')
	writeJSONString(&b, k.Origin)
	b.WriteByte(',')
	writeJSONString(&b, k.AccountID)
	b.WriteByte(',')
	writeJSONString(&b, k.ConsumerNamespace)
	b.WriteByte(',')
	writeJSONString(&b, k.FilterKey)
	b.WriteByte(']')
	return b.String()
}

// checkIdentityText rejects an identity component that is not valid UTF-8.
// FlatKey (and the srv1 digest, and the subscription identifier) encode
// rune-wise through writeJSONString, which decodes every invalid byte to the
// SAME U+FFFD replacement rune — so the encoding is one-to-one over valid
// UTF-8 and many-to-one outside it. Two distinct consumer namespaces
// (single-byte 0xff and 0xfe, say) would render one identical key, and the
// two independent consumers behind them would load and overwrite a single
// checkpoint lineage, each skipping the other's events. The components come
// from construction inputs, so this fails closed at construction — a
// usage-coded error with zero wire attempts — rather than silently mangling
// an identity at save time.
func checkIdentityText(field, s string) error {
	if !utf8.ValidString(s) {
		return usageError(field + " must be valid UTF-8")
	}
	return nil
}

// CanonicalOrigin canonicalizes a configured base URL to its
// checkpoint-identity form: lowercase scheme and host; the default port
// omitted (":443" for https, ":80" for http); no path, query, fragment, or
// trailing slash — canonical form exactly `scheme "://" host
// [":" nondefault-port]`. Hosts are used as configured after lowercasing (no
// IDN/punycode transformation).
func CanonicalOrigin(origin string) (string, error) {
	u, err := url.Parse(origin)
	if err != nil {
		return "", fmt.Errorf("eventfeed: unparseable origin %q: %w", origin, err)
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	if scheme == "" || host == "" {
		return "", fmt.Errorf("eventfeed: origin %q must carry a scheme and host", origin)
	}
	if strings.Contains(host, ":") {
		// An IPv6 literal: url.URL.Hostname strips the brackets; the
		// canonical host form keeps them.
		host = "[" + host + "]"
	}
	port := u.Port()
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		return scheme + "://" + host + ":" + port, nil
	}
	return scheme + "://" + host, nil
}

// CheckpointStore is host-provided persistence for the durable feed position.
// The contract is tri-state (SPEC.md §23 "Seam Contracts"):
//
//	load → Loaded(position) | Missing | Failed(error)
//	save → Saved | Failed(error)
//
// In Go, Load expresses Loaded as (position, true, nil), Missing as
// ("", false, nil), and Failed as a non-nil error (the other returns are
// ignored); Save expresses Saved as nil and Failed as the error. Missing
// proceeds to a present-class entry — no stored cursor is not an error.
// Failed on load is Terminal(checkpoint_load) with zero wire attempts;
// collapsing it to Missing would silently start at the present and skip
// history. Failed on save is Observer.CheckpointSaveFailed with the feed
// continuing — subsequent saves are still attempted.
type CheckpointStore interface {
	// Load returns the stored position for key, whether one exists, and any
	// store failure. Load happens exactly once, on the first iteration,
	// before the first mint.
	Load(ctx context.Context, key CheckpointKey) (position string, ok bool, err error)
	// Save durably records position under key. Only poll-page acceptance
	// ever saves; live event ids never advance the durable position.
	Save(ctx context.Context, key CheckpointKey, position string) error
}

// checkpointKey is this run's durable identity — all four parts, always.
func (l *loop) checkpointKey() CheckpointKey {
	return CheckpointKey{
		Origin:            l.cfg.origin,
		AccountID:         l.cfg.accountID,
		ConsumerNamespace: l.cfg.consumerNamespace,
		FilterKey:         l.cfg.filters.FilterKey(),
	}
}

// loadCheckpoint runs the store's load exactly once, on the first iteration
// and BEFORE the first mint. Loaded seeds the in-memory position UNDER THE
// RESUME MODE (which is authoritative from then on); Missing proceeds to a
// present-class entry — no stored cursor is not an error; Failed is
// Terminal(checkpoint_load) with zero wire attempts, because collapsing it to
// Missing would silently start at the present and skip history. It runs
// whenever a store is configured, including under an explicit entry mode: the
// lineage's identity is settled before anything durable can move.
//
// Only `StartResume` is defined as "the stored position if any" (SPEC.md §23
// "Consumer Ergonomics"). `StartPresent`, `StartBeginning` and `StartAfter`
// promise `since=now`, `since=0` and `since=<id>`, and a caller pairing one
// with a store means it: seeding the position from the load would make every
// explicit mode behave as resume the moment the store had anything in it, so
// a checkpointed feed could never be deliberately replayed or reset. The load
// still HAPPENS under those modes — its failure edge and its lineage identity
// are not mode-dependent — the value is simply not what the entry is taken
// from. The store is written under the same key either way, so the run's
// first accepted page repoints the lineage.
func (l *loop) loadCheckpoint() *TerminalError {
	if l.cfg.store == nil {
		return nil
	}
	position, ok, err := l.cfg.store.Load(l.runCtx, l.checkpointKey())
	if err != nil {
		return &TerminalError{
			Reason: ReasonCheckpointLoad,
			Msg:    "the checkpoint store failed to load the stored position",
			Err:    err,
		}
	}
	if ok && l.cfg.start.kind == startResume {
		l.position = position
	}
	return nil
}

// saveCheckpoint write-throughs one accepted position. The connector's own
// position tracking is in-memory and authoritative for resume and repair
// within the run; the store is durability only, so a failed save is reported
// through the observer and the feed continues — subsequent saves are still
// attempted, and the live cursor is neither regressed nor blanked.
func (l *loop) saveCheckpoint(position string) {
	if l.cfg.store == nil {
		return
	}
	if err := l.cfg.store.Save(l.runCtx, l.checkpointKey(), position); err != nil {
		if l.cfg.observer.CheckpointSaveFailed != nil {
			l.cfg.observer.CheckpointSaveFailed(err)
		}
		return
	}
	if l.cfg.observer.Checkpoint != nil {
		l.cfg.observer.Checkpoint(position)
	}
}
