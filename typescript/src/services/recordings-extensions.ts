/**
 * `RecordingsService.summarize` — a compact projection of one recording,
 * resolved from the pointer an account event feed row or a webhook carries
 * (bucket id, recording id, and the event type or recording type) through the
 * typed read that type names.
 *
 * It exists for consumers that must decide something about a recording without
 * paying for its full payload: an agent connector's admission step, an MCP tool
 * answering "what is this?".
 *
 * The SDK has no untyped recording read (BC3 has no such route), so the type is
 * the routing key: `comment.created` reads a comment, `card.created` reads a
 * card, and so on — one typed read per type. Chat lines are the exception,
 * because their read needs the Campfire id and the pointer does not carry it;
 * `summarize` discovers the Campfire first (see {@link RecordingsService.summarize}
 * and the discovery notes further down).
 *
 * This is hand-written composition over the generated services (SPEC §18,
 * Appendix F). It makes no wire request of its own, and it mints no operation
 * identity: hooks see the constituent reads under their own names (§18 rule 3).
 * It lives in `src/services/*-extensions.ts` and is wired in `client.ts`, the
 * placement §18 rule 5 designates for TypeScript.
 */

import type { BasecampHooks } from "../hooks.js";
import { BasecampError, Errors, isBasecampError, truncateErrorMessage } from "../errors.js";
import type { BasecampErrorOptions, ErrorCode } from "../errors.js";
import { RecordingsService as GeneratedRecordingsService } from "../generated/services/recordings.js";
import type { RawClient } from "./base.js";
import type { components } from "../generated/schema.js";
import type { Person } from "../generated/services/people.js";
import type { CampfireLine } from "../generated/services/campfires.js";
import type { BasecampClient } from "../client.js";
import { mentionedPersonIds } from "./mentions.js";

// =============================================================================
// Public types
// =============================================================================

/** The nested identities the projection carries verbatim from the read. */
type RecordingParent = components["schemas"]["RecordingParent"];
type RecordingBucket = components["schemas"]["RecordingBucket"];

/**
 * The generated services `summarize` composes, as the client exposes them.
 *
 * Named rather than taken as the whole client so the composite's reach is
 * stated: every one of these is a public generated wire method, which is what
 * §18 rule 1 asks for.
 */
export type RecordingReadSources = Pick<
  BasecampClient,
  | "campfires"
  | "cardColumns"
  | "cards"
  | "cardSteps"
  | "cardTables"
  | "checkins"
  | "clientApprovals"
  | "clientCorrespondences"
  | "cloudFiles"
  | "comments"
  | "documents"
  | "forwards"
  | "googleDocuments"
  | "messageBoards"
  | "messages"
  | "projects"
  | "schedules"
  | "todolists"
  | "todos"
  | "todosets"
  | "uploads"
  | "vaults"
>;

/** Points at one recording the way an event feed row does. */
export interface RecordingRef {
  /**
   * The project the recording lives in. Required: it scopes the Campfire
   * discovery for chat lines, and the read is checked against it so a pointer
   * from one project can never resolve to a recording in another.
   */
  bucketId: number;

  /** The recording's id. */
  recordingId: number;

  /**
   * The account event feed type that named the recording —
   * `"comment.created"`, `"card.assignment_changed"`, `"chat.line.created"`.
   * The segment before the action names the recording type. Used when
   * `recordingType` is absent.
   */
  eventType?: string;

  /**
   * The recording's own type as BC3 spells it — `"Comment"`, `"Kanban::Card"`,
   * `"Chat::Lines::Text"`. When set it takes precedence over `eventType`, being
   * the more exact of the two.
   */
  recordingType?: string;
}

/**
 * The projection {@link RecordingsService.summarize} returns.
 *
 * Keys are the wire spelling, as everywhere else in this SDK: the generated
 * types are the API's own snake_case shapes, and a camelCase projection here
 * would be the one object in the surface that disagreed with them.
 *
 * Fields a type does not have are absent: a Comment has no `assignees`, a Vault
 * no `content` worth the name.
 */
export interface RecordingSummary {
  id: number;
  status: string;
  /** The recording type as BC3 spells it (`"Comment"`, `"Kanban::Card"`). */
  type: string;
  title: string;
  app_url: string;
  /**
   * The recording this one hangs off — the commented recording for a comment,
   * the Campfire for a chat line, the column for a card.
   */
  parent?: RecordingParent;
  bucket?: RecordingBucket;
  creator?: Person;
  /** Set for the assignable types (to-dos, cards, card steps). */
  assignees?: Person[];
  /**
   * The people `content` mentions, per `mentionedPersonIds`. Never absent, so a
   * JSON consumer reads `[]` rather than a missing key.
   */
  mentioned_person_ids: number[];
  /**
   * The recording's rich text, in full: the comment body, the message body, a
   * to-do's description, a card's content, the chat line.
   */
  content: string;
  updated_at: string;
  /**
   * The Campfire a chat line was found under — the reply destination for a chat
   * trigger. Absent for every other type.
   */
  campfire_id?: number;
}

// =============================================================================
// Errors
// =============================================================================

/**
 * The identities `summarize` fails with, beyond a read's own error.
 *
 * Matched on {@link RecordingSummaryError.kind}, never parsed out of a message:
 * `recording_unresolved` in particular is a conclusion the composite reached,
 * not a status any one request returned, and a consumer has to be able to tell
 * it from a read that failed.
 */
export type RecordingSummaryErrorKind =
  /**
   * The event type names no recording type — `boost.created`, whose recording
   * is the boost's target and whose type the feed row does not carry. A
   * consumer resolves those from its own record of what it posted, not through
   * `summarize`.
   */
  | "no_recording_type"
  /** Neither `eventType` nor `recordingType` names a type in the routing table. */
  | "unknown_recording_type"
  /**
   * A chat line was found under none of the Campfires the caller can currently
   * see in its bucket. Distinct from a failed read (any non-404 answer is
   * raised as itself) and from `campfire_discovery_incomplete`: every candidate
   * answered 404. It is not distinct from lost visibility — BC3 answers 404 for
   * a Campfire the caller may not see, too — so a consumer marks the record
   * blocked and retries on its own schedule; see
   * {@link UnresolvedRecordingError.staleCampfireIds}.
   */
  | "recording_unresolved"
  /**
   * Discovery could not be carried to a conclusion — the Campfire listing
   * overflowed its cap, or a bucket has more visible Campfires than the
   * candidate budget. Candidates were left unsearched, so nothing can be
   * reported absent.
   */
  | "campfire_discovery_incomplete"
  /** The recording the read returned lives in a different bucket from the pointer's. */
  | "bucket_mismatch";

/**
 * The base class for every failure that is the composite's own conclusion
 * rather than one read's HTTP answer.
 *
 * It is a {@link BasecampError} so an existing `catch` keeps working, and it
 * carries a `kind` so the conclusions stay distinguishable from each other and
 * from the read errors whose `code` they share. Every one of them is
 * **statusless**: no single request returned a status that describes the
 * conclusion, so a consumer seeing `code: "not_found"` with no `httpStatus`
 * knows it is looking at a composite's verdict and not at a 404 off the wire.
 */
export class RecordingSummaryError extends BasecampError {
  readonly kind: RecordingSummaryErrorKind;

  constructor(
    kind: RecordingSummaryErrorKind,
    code: ErrorCode,
    message: string,
    options?: BasecampErrorOptions,
  ) {
    super(code, message, { ...options, retryable: false });
    this.name = "RecordingSummaryError";
    this.kind = kind;
  }
}

/**
 * A {@link RecordingRef} `summarize` cannot route, raised before any request.
 *
 * `usage`: nothing was asked of the API — the pointer names no read.
 */
export class RecordingRoutingError extends RecordingSummaryError {
  readonly ref: RecordingRef;

  constructor(ref: RecordingRef, kind: "no_recording_type" | "unknown_recording_type") {
    // Raw, and the fallback is on exactly "", as Go's RecordingRoutingError
    // reads them: routing trims before matching, but the message reports what
    // the caller actually passed.
    const key =
      ref.recordingType !== undefined && ref.recordingType !== ""
        ? ref.recordingType
        : (ref.eventType ?? "");
    const reason =
      kind === "no_recording_type"
        ? "event type names no recording type"
        : "no typed read for recording type";
    super(kind, "usage", `${reason}: ${JSON.stringify(key)}`, {
      hint:
        kind === "no_recording_type"
          ? "resolve the recording from your own record of what the event referred to"
          : "pass a recording type summarizableRecordingTypes() lists, or an event type summarizableEventTypes() lists",
    });
    this.name = "RecordingRoutingError";
    this.ref = ref;
  }
}

/** A chat line found under no visible Campfire. */
export class UnresolvedRecordingError extends RecordingSummaryError {
  readonly bucketId: number;
  readonly recordingId: number;
  /** The candidates tried, in order; empty when the bucket has no visible Campfire at all. */
  readonly campfireIds: number[];
  /**
   * Whether the cached discovery sources were re-read before concluding. False
   * when every source had been read within the last
   * {@link CAMPFIRE_INDEX_MIN_REFRESH_MS}, so a Campfire created in that window
   * was not seen: the conclusion stands on data up to that old, and a retry
   * after the floor sees the current sources.
   */
  readonly refreshed: boolean;
  /**
   * Candidates from the cache that the refreshed sources no longer list —
   * Campfires the caller could see when the cache filled and cannot now. Set
   * only when {@link refreshed}.
   */
  readonly staleCampfireIds: number[];

  constructor(init: {
    bucketId: number;
    recordingId: number;
    campfireIds: number[];
    refreshed: boolean;
    staleCampfireIds: number[];
  }) {
    super(
      "recording_unresolved",
      "not_found",
      `chat line found under no visible campfire: line ${init.recordingId} in bucket ${init.bucketId} ` +
        `(tried ${init.campfireIds.length} campfires)`,
      {
        hint: "the line is under no Campfire this credential can currently see; retry once visibility may have changed",
      },
    );
    this.name = "UnresolvedRecordingError";
    this.bucketId = init.bucketId;
    this.recordingId = init.recordingId;
    this.campfireIds = init.campfireIds;
    this.refreshed = init.refreshed;
    this.staleCampfireIds = init.staleCampfireIds;
  }
}

/** Discovery stopped short of a conclusion, and why. */
export class CampfireDiscoveryIncompleteError extends RecordingSummaryError {
  readonly bucketId: number;
  readonly recordingId: number;
  readonly reason: string;

  constructor(init: { bucketId: number; recordingId: number; reason: string }) {
    super(
      "campfire_discovery_incomplete",
      "api_error",
      `campfire discovery incomplete: line ${init.recordingId} in bucket ${init.bucketId}: ${init.reason}`,
      { hint: "candidates were left unsearched, so the line cannot be reported absent" },
    );
    this.name = "CampfireDiscoveryIncompleteError";
    this.bucketId = init.bucketId;
    this.recordingId = init.recordingId;
    this.reason = init.reason;
  }
}

/**
 * A read whose bucket disagrees with the pointer.
 *
 * `api_error`, not `usage`: the disagreeing value came off the wire (SPEC §6,
 * "Statusless `api_error` for a malformed 2xx body" — classification is by
 * origin, not by value).
 */
export class BucketMismatchError extends RecordingSummaryError {
  readonly ref: RecordingRef;
  /** The bucket the read returned. */
  readonly bucketId: number;

  constructor(ref: RecordingRef, bucketId: number) {
    super(
      "bucket_mismatch",
      "api_error",
      `recording is not in the requested bucket: recording ${ref.recordingId} is in bucket ${bucketId}, not ${ref.bucketId}`,
    );
    this.name = "BucketMismatchError";
    this.ref = ref;
    this.bucketId = bucketId;
  }
}

// =============================================================================
// Routing
// =============================================================================

/** The routing key: which typed read serves a {@link RecordingRef}. */
type SummaryKind =
  | "comment"
  | "message"
  | "todo"
  | "card"
  | "chatLine"
  | "document"
  | "upload"
  | "scheduleEntry"
  | "question"
  | "questionAnswer"
  | "todolist"
  | "vault"
  | "forward"
  | "clientApproval"
  | "clientCorrespondence"
  | "googleDocument"
  | "cloudFile"
  | "cardStep"
  | "questionnaire"
  | "schedule"
  | "todoset"
  | "messageBoard"
  | "cardTable"
  | "cardColumn"
  | "inbox"
  | "campfire";

/**
 * Maps the subject of an account event feed type — everything before its final
 * `.` — to a read. This is the feed's catalog (bc3 `Event::EventType`) minus
 * `boost`, which names no recording type and is refused explicitly rather than
 * left to fall through as unknown.
 */
const EVENT_SUBJECTS = new Map<string, SummaryKind>([
  ["comment", "comment"],
  ["message", "message"],
  ["todo", "todo"],
  ["card", "card"],
  ["chat.line", "chatLine"],
]);

/**
 * Maps BC3's recording type strings to a read. It is the routing contract, and
 * it is a DELIBERATE set, not an exhaustive one: the recording types the
 * account event feed's trigger matrix names (comment, message, to-do, card,
 * chat line), plus the content and tool recordings a consumer reasoning about
 * those is likely to hold an id for. Chat lines are matched by prefix
 * (`Chat::Lines::Text`, `::RichText`, `::Code`, `::Upload`, `::Integration` all
 * read through the same route); everything else exactly.
 *
 * A type outside this set is `unknown_recording_type` by design, whether or not
 * the SDK has an id-only read for it — `Timesheet::Entry` and `Gauge::Needle`
 * do, and are not routed; `Client::Reply` and `Forward::Reply` cannot be, since
 * their reads need a parent id the pointer does not carry. Widening the set is
 * a product decision, not a gap: add the type here, its projection in
 * `readSummary`, and a case in `conformance/tests/recording_summary.json`, the
 * fixture every port implements.
 *
 * A `Map`, not an object literal: the key is an untrusted string off an event
 * feed row or a webhook, and `{}["constructor"]` is a hit. A routing table that
 * answers for every `Object.prototype` member would route those to something
 * that is not a read at all.
 */
const RECORDING_TYPES = new Map<string, SummaryKind>([
  ["Comment", "comment"],
  ["Message", "message"],
  ["Todo", "todo"],
  ["Kanban::Card", "card"],
  ["Document", "document"],
  ["Upload", "upload"],
  ["Schedule::Entry", "scheduleEntry"],
  ["Question", "question"],
  ["Question::Answer", "questionAnswer"],
  ["Todolist", "todolist"],
  ["Vault", "vault"],
  ["Inbox::Forward", "forward"],
  ["Client::Approval", "clientApproval"],
  ["Client::Correspondence", "clientCorrespondence"],
  ["GoogleDocument", "googleDocument"],
  ["CloudFile", "cloudFile"],
  ["Kanban::Step", "cardStep"],
  ["Questionnaire", "questionnaire"],
  ["Schedule", "schedule"],
  ["Todoset", "todoset"],
  ["Message::Board", "messageBoard"],
  ["Kanban::Board", "cardTable"],
  ["Kanban::Column", "cardColumn"],
  ["Inbox", "inbox"],
  ["Chat::Transcript", "campfire"],
]);

const CHAT_LINE_TYPE_PREFIX = "Chat::Lines::";

/**
 * The recording types `summarize` routes by `recordingType`, sorted, with the
 * `Chat::Lines` subtypes represented by their shared prefix
 * (`"Chat::Lines::*"`). The set is deliberate rather than exhaustive — see
 * {@link RECORDING_TYPES} — and any other type is `unknown_recording_type` by
 * design.
 */
export function summarizableRecordingTypes(): string[] {
  return [...RECORDING_TYPES.keys(), `${CHAT_LINE_TYPE_PREFIX}*`].sort();
}

/**
 * The account event feed subjects `summarize` routes by `eventType` — an event
 * type is `<subject>.<action>`, and any action on a listed subject routes to
 * that subject's read — sorted. `boost` is absent on purpose: it is
 * `no_recording_type`.
 */
export function summarizableEventTypes(): string[] {
  return [...EVENT_SUBJECTS.keys()].map((subject) => `${subject}.*`).sort();
}

/**
 * Whether a chat line subtype carries rich text — the two that declare
 * `rich_text_attribute :content` in BC3, and so the only two whose content can
 * hold a mention. A Text line's content is HTML-escaped on the way out
 * (`content_helper.rb`, `format_chat_line_with`), a Code line's is served
 * verbatim — a snippet that happens to contain a `bc-attachment` tag — and an
 * Upload line has no content.
 */
function chatLineIsRichText(lineType: string): boolean {
  return lineType === "Chat::Lines::RichText" || lineType === "Chat::Lines::Integration";
}

/** Picks the read for a ref. `recordingType` wins when set. */
function routeRecording(ref: RecordingRef): SummaryKind {
  const recordingType = ref.recordingType?.trim() ?? "";
  if (recordingType !== "") {
    if (recordingType.startsWith(CHAT_LINE_TYPE_PREFIX)) return "chatLine";
    const kind = RECORDING_TYPES.get(recordingType);
    if (kind !== undefined) return kind;
    throw new RecordingRoutingError(ref, "unknown_recording_type");
  }

  const eventType = ref.eventType?.trim() ?? "";
  if (eventType === "") throw new RecordingRoutingError(ref, "unknown_recording_type");

  // A feed type is "<subject>.<action>"; the subject names the recording type.
  // A string with no action is not a feed type and is not routed.
  const dot = eventType.lastIndexOf(".");
  if (dot <= 0 || dot === eventType.length - 1) {
    throw new RecordingRoutingError(ref, "unknown_recording_type");
  }
  const subject = eventType.slice(0, dot);
  if (subject === "boost") throw new RecordingRoutingError(ref, "no_recording_type");

  const kind = EVENT_SUBJECTS.get(subject);
  if (kind !== undefined) return kind;
  throw new RecordingRoutingError(ref, "unknown_recording_type");
}

// =============================================================================
// Campfire discovery
// =============================================================================

/**
 * How long a cached discovery source — a bucket's project dock, the account's
 * Campfire listing — is reused before it is read again.
 */
export const CAMPFIRE_INDEX_TTL_MS = 10 * 60 * 1000;

/**
 * Bounds the refresh-on-miss: a line found under no candidate re-reads the
 * cached sources, but not more often than this per source, so a run of
 * unresolvable lines cannot turn into a listing per line.
 * {@link UnresolvedRecordingError.refreshed} says whether the floor applied.
 */
const CAMPFIRE_INDEX_MIN_REFRESH_MS = 30 * 1000;

/**
 * Bounds how many Campfires one `summarize` call tries, across both sources and
 * the refresh. A project has one Campfire and a handful of pings; a bucket past
 * this bound is not a shape BC3 produces, and the call reports
 * `campfire_discovery_incomplete` rather than calling the rest absent.
 */
export const MAX_CAMPFIRE_CANDIDATES = 50;

/**
 * Caps the account-wide Campfire listing the fallback source reads. A listing
 * that overflows it is not cached and the call reports
 * `campfire_discovery_incomplete`: the dock covers every project, so the
 * listing only ever serves the leftover, and an account with more Campfires
 * than this should not pay a full walk per ten minutes for it.
 */
export const MAX_CAMPFIRE_LISTING = 1000;

/**
 * Bounds each discovery cache's entry count. The dock cache holds one snapshot
 * per bucket consulted within the TTL: a connector listening across every
 * project an agent can see touches hundreds of buckets, not thousands, and a
 * snapshot is a handful of ids, so 1024 is generous headroom, and the bound
 * exists so that a process alive for weeks can never grow past it whatever it
 * sees. When the bound is reached the oldest-fetched entries go first,
 * deterministically.
 */
const CAMPFIRE_INDEX_MAX_ITEMS = 1024;

/** What a cache read hands back. */
interface TtlHit<V> {
  value: V;
  /**
   * When the snapshot was fetched, on the cache's monotonic clock. It is what
   * lets a caller tell a snapshot it already consulted from a newer one,
   * whoever loaded it.
   */
  fetchedAt: number;
  /**
   * Whether the value predated the call, as opposed to being loaded during it
   * — by this caller or by one it waited on.
   */
  cached: boolean;
}

interface TtlEntry<V> {
  value: V;
  fetchedAt: number;
  /** Publication order: the tie-breaker when fetch times are equal. */
  seq: number;
}

/**
 * A per-key cache with single-flight loading: concurrent callers for one key
 * await the one load in progress rather than loading again, and a failed load
 * leaves the previous value in place and is not cached.
 *
 * The clock is `performance.now()`, which is monotonic: a wall-clock adjustment
 * cannot make a snapshot look newer than it is, or make the refresh floor
 * elapse early.
 *
 * Go's version additionally distinguishes a load that failed because the
 * *loading caller's* context was cancelled, so a live waiter can load for
 * itself. There is no per-call cancellation to attribute here — the services
 * take no signal — so a failed load's rejection is simply shared by everyone
 * waiting on it, which is the half of that rule that matters: N waiters never
 * re-run one failed load N times.
 */
class TtlCache<K, V> {
  readonly #entries = new Map<K, TtlEntry<V>>();
  readonly #inflight = new Map<K, Promise<TtlHit<V>>>();
  readonly #ttlMs: number;
  readonly #floorMs: number;
  readonly #maxItems: number;
  readonly #now: () => number;
  #seq = 0;

  constructor(ttlMs: number, floorMs: number, maxItems: number, now: () => number = () => performance.now()) {
    this.#ttlMs = ttlMs;
    this.#floorMs = floorMs;
    this.#maxItems = maxItems;
    this.#now = now;
  }

  /**
   * The cached value for `key` when one is within the TTL, without loading. It
   * lets a caller consult what a source already holds before deciding whether
   * to pay for a fetch of it.
   */
  peek(key: K): TtlHit<V> | undefined {
    const entry = this.#entries.get(key);
    if (entry === undefined || this.#now() - entry.fetchedAt >= this.#ttlMs) return undefined;
    return { value: entry.value, fetchedAt: entry.fetchedAt, cached: true };
  }

  /**
   * The value for `key`, loading it when absent or older than the TTL — or,
   * with `refresh` set, older than the floor.
   */
  async get(key: K, refresh: boolean, load: () => Promise<V>): Promise<TtlHit<V>> {
    const entry = this.#entries.get(key);
    if (entry !== undefined) {
      const age = this.#now() - entry.fetchedAt;
      if (age < this.#ttlMs && (!refresh || age < this.#floorMs)) {
        return { value: entry.value, fetchedAt: entry.fetchedAt, cached: true };
      }
    }

    const pending = this.#inflight.get(key);
    // The load this call waits on is this call's load: its value is handed over
    // as fresh, not as something that predated the call.
    //
    // A waiter shares the load's outcome unconditionally, success or failure,
    // and nothing here reads the rejection to decide otherwise. That is what
    // keeps one failed load from becoming one request per waiter, and it is
    // also why the trap Go's `callerDone` exists to avoid cannot be sprung
    // here: Go has to tell a load that failed on the LOADING caller's dead
    // context from one that failed on its own, so a still-live waiter can load
    // again, and it deliberately refuses to decide that from the error alone —
    // an `http.Client.Timeout` satisfies `errors.Is(err,
    // context.DeadlineExceeded)` with the owner still live. TypeScript has the
    // same ambiguity in a sharper form, since an aborted `fetch` rejects with
    // an `AbortError` whether a timeout or a caller fired the signal. There is
    // nothing to attribute: the services take no per-call signal, so no caller
    // can be dead while another is live, and no waiter ever re-runs a load.
    //
    // That is a property of the CALL SITES, not of the client — client.ts does
    // thread an AbortSignal through its middleware and retry loop. Giving
    // summarize() a per-call signal would make a load abortable by one caller
    // while another still waits, and this paragraph would stop being true: Go's
    // attribution would have to be ported with it, and it cannot be done from
    // the rejection, since an aborted fetch rejects with an AbortError
    // whichever signal fired.
    if (pending !== undefined) return pending;

    // The order of these three statements is the release guarantee, and it is
    // load-bearing. An `async` function never throws synchronously — a throw in
    // its body becomes a rejection — so invoking the wrapper always yields a
    // promise; `.finally` is attached to that promise; and only then is the key
    // marked in flight. There is therefore no instant at which the slot is held
    // without a release handler attached to the thing holding it, and every way
    // the wrapper can end — `load()` throwing synchronously, its promise
    // rejecting, `#publish` throwing — settles that promise and runs the
    // `finally`. Go reaches the same guarantee the other way round, with a
    // deferred publish that survives a panicking loader.
    //
    // The one exit this cannot cover is a `load()` that never settles, which
    // would hold the slot and park every later caller for the life of the
    // process. Go's waiters escape that through their own context; these have
    // no signal to escape with. What closes it is one layer down: every request
    // a loader makes carries the client's request timeout, so a load that is
    // going nowhere rejects rather than hanging.
    //
    // A loader must not re-enter this cache for its own key. It would not join
    // the load in progress — the slot is taken after `load()` is invoked, where
    // Go takes it before — and would start a second one. Neither loader here
    // does; both call a single generated read.
    const started = (async () => {
      const value = await load();
      return this.#publish(key, value);
    })().finally(() => {
      // A failed load leaves the previous entry in place and caches nothing; the
      // key is released either way so the next caller can load again.
      //
      // By IDENTITY, not by key. A blind `delete(key)` would drop whatever is
      // registered there, and if that were ever a later caller's live load, the
      // single-flight guarantee would go with it — two concurrent account-wide
      // listings under one key, which is process-wide, since that key is a
      // constant. The ordering above makes that unreachable today: a later
      // caller can only register after this handler has vacated the slot,
      // because until then it sees the pending promise and waits on it. The
      // guard makes the property local anyway, so a second vacate path added
      // later cannot quietly reintroduce it.
      if (this.#inflight.get(key) === started) this.#inflight.delete(key);
    });
    this.#inflight.set(key, started);
    return started;
  }

  #publish(key: K, value: V): TtlHit<V> {
    const fetchedAt = this.#now();
    this.#sweep();
    this.#makeRoom(key);
    this.#entries.set(key, { value, fetchedAt, seq: ++this.#seq });
    return { value, fetchedAt, cached: false };
  }

  /**
   * Drops every entry past its TTL. It runs at each publication — the one
   * moment the cache does work proportional to a miss anyway — so a long-lived
   * client that has seen many buckets keeps a snapshot for at most a TTL past
   * its last use plus the interval to the next load on any key, rather than for
   * its lifetime.
   */
  #sweep(): void {
    const now = this.#now();
    for (const [key, entry] of this.#entries) {
      if (now - entry.fetchedAt >= this.#ttlMs) this.#entries.delete(key);
    }
  }

  /**
   * Evicts the oldest-fetched entries until the one about to be stored for
   * `key` fits under the bound — oldest by fetch time, and by publication order
   * among equals, so the choice is total rather than whatever iteration order
   * happens to visit first. Oldest-first is also the right order: the entry
   * nearest its TTL is the one least worth keeping.
   */
  #makeRoom(key: K): void {
    if (this.#maxItems <= 0) return;
    if (this.#entries.has(key)) return; // an overwrite takes no new room
    while (this.#entries.size >= this.#maxItems) {
      let oldestKey: K | undefined;
      let oldest = Number.POSITIVE_INFINITY;
      let oldestSeq = Number.POSITIVE_INFINITY;
      for (const [candidate, entry] of this.#entries) {
        if (entry.fetchedAt < oldest || (entry.fetchedAt === oldest && entry.seq < oldestSeq)) {
          oldestKey = candidate;
          oldest = entry.fetchedAt;
          oldestSeq = entry.seq;
        }
      }
      if (oldestKey === undefined) return;
      this.#entries.delete(oldestKey);
    }
  }
}

/** One consultation of a discovery source. */
interface SourceRead {
  /** The candidate ids it holds for the bucket. */
  ids: number[];
  fetchedAt: number;
  /** Whether the snapshot predated the call. */
  cached: boolean;
}

/** The single key the account-wide listing cache is stored under. */
const LISTING_KEY = "campfires";

/**
 * The two discovery sources.
 *
 * A client is bound to one credential and one account, so entries are never
 * shared across authorization contexts and a bucket id is a sufficient key.
 * Expired snapshots are swept at each load and each cache is bounded
 * ({@link CAMPFIRE_INDEX_MAX_ITEMS}, oldest out first), so the index holds at
 * most the buckets consulted within the last TTL, and never more than the
 * bound, whatever the client has seen.
 */
class CampfireIndex {
  readonly #docks: TtlCache<number, number[]>;
  readonly #listings: TtlCache<string, Map<number, number[]>>;

  constructor(now?: () => number) {
    this.#docks = new TtlCache<number, number[]>(
      CAMPFIRE_INDEX_TTL_MS,
      CAMPFIRE_INDEX_MIN_REFRESH_MS,
      CAMPFIRE_INDEX_MAX_ITEMS,
      now,
    );
    this.#listings = new TtlCache<string, Map<number, number[]>>(
      CAMPFIRE_INDEX_TTL_MS,
      CAMPFIRE_INDEX_MIN_REFRESH_MS,
      CAMPFIRE_INDEX_MAX_ITEMS,
      now,
    );
  }

  /**
   * The Campfire ids a bucket's project dock names. A bucket that is not a
   * project (a 404 on the project read) has none; any other failure of the read
   * is raised.
   */
  async dockCampfires(
    sources: RecordingReadSources,
    bucketId: number,
    refresh: boolean,
  ): Promise<SourceRead> {
    const hit = await this.#docks.get(bucketId, refresh, async () => {
      let project;
      try {
        project = await sources.projects.get(bucketId);
      } catch (err) {
        if (isNotFound(err)) return [];
        throw err;
      }
      // Two layers, as Go has two: its decoder refuses a dock id that is not a
      // number and fails the whole project read, and only then does it skip a
      // zero id. TypeScript decodes nothing, so the first layer is the explicit
      // refusal below — without it a malformed id would be dropped silently and
      // discovery could go on to report the line unresolved, a composite
      // verdict standing in for a failed read.
      return sourceItems(sourceObject(project, "the project read").dock, "the project dock")
        .filter((item) => recordingText(item.name, "a project dock item name") === "chat")
        .map((item) => numericId(item.id, "project dock item"))
        .filter((id) => id !== 0);
    });
    return { ids: hit.value, fetchedAt: hit.fetchedAt, cached: hit.cached };
  }

  /**
   * The Campfire ids the cached account-wide listing shows in a bucket, without
   * fetching: `undefined` when the listing is not cached or has expired.
   */
  cachedListedCampfires(bucketId: number): SourceRead | undefined {
    const hit = this.#listings.peek(LISTING_KEY);
    if (hit === undefined) return undefined;
    return { ids: [...(hit.value.get(bucketId) ?? [])], fetchedAt: hit.fetchedAt, cached: true };
  }

  /**
   * The Campfire ids the account-wide listing shows in a bucket. A listing that
   * overflows {@link MAX_CAMPFIRE_LISTING} is not cached and is reported as
   * incomplete discovery by the caller.
   */
  async listedCampfires(
    sources: RecordingReadSources,
    bucketId: number,
    refresh: boolean,
  ): Promise<SourceRead> {
    const hit = await this.#listings.get(LISTING_KEY, refresh, async () => {
      const list = await sources.campfires.list({ maxItems: MAX_CAMPFIRE_LISTING });
      if (list.meta.truncated) throw new CampfireListingOverflow();
      const byBucket = new Map<number, number[]>();
      for (const campfire of sourceItems(list, "the campfire listing")) {
        // The bucket OBJECT, not only its id: a string or an array here left
        // `?.id` undefined and the entry was dropped in silence, where Go's
        // decode of the same body fails the listing read. A JSON null is the
        // measured exception — the zero struct, whose zero bucket id the filter
        // below skips, exactly as Go's `c.Bucket == nil` does.
        const bucket = sourceObject(campfire.bucket, "campfire listing bucket");
        // Go filters this source on the bucket alone — a zero campfire id is a
        // candidate there, spends budget, and lands in the unresolved verdict's
        // campfireIds — so it is kept here too. Only a value Go's decoder would
        // have refused outright is refused.
        const campfireId = numericId(campfire.id, "campfire listing entry");
        // `wireInteger`, not `numericId`: Go's line here is `c.Bucket == nil ||
        // c.Bucket.ID == 0` and then `byBucket[c.Bucket.ID]` — a map key and a
        // comparand. Only `c.ID` below is ever put in a URL, so only it carries
        // the safe-integer clause. Routing this one through `numericId` failed
        // the whole listing over one entry in ANOTHER bucket, which is worse
        // than what happens now: such an entry is skipped, and the healthy ones
        // beside it are kept.
        const listedBucketId = wireInteger(bucket.id, "campfire listing bucket");
        if (listedBucketId === 0) continue;
        // Unreachable while `summarize` refuses a caller `bucketId` past 2^53:
        // every rounded value above that is itself above MAX_SAFE_INTEGER, so
        // it can never equal a safe id and the filter below already drops it.
        // Stated here anyway, because the property it rests on lives in another
        // method — and reverting this line is behaviour-neutral, which the
        // guard-checking harness records as such rather than as a gap. What
        // makes it worth stating: rounding is not injective, so two int64
        // buckets can land on one double, and a key built from one would offer
        // another project's Campfire as a candidate.
        if (!Number.isSafeInteger(listedBucketId)) continue;
        const ids = byBucket.get(listedBucketId);
        if (ids === undefined) byBucket.set(listedBucketId, [campfireId]);
        else ids.push(campfireId);
      }
      return byBucket;
    });
    return { ids: [...(hit.value.get(bucketId) ?? [])], fetchedAt: hit.fetchedAt, cached: hit.cached };
  }
}

/**
 * The load failure for a listing past its cap; `#resolveChatLine` turns it into
 * the typed {@link CampfireDiscoveryIncompleteError}. Module-private and never
 * a `BasecampError`, so it can never be mistaken for a read error a caller
 * could see or reach one uncaught.
 */
class CampfireListingOverflow extends Error {
  constructor() {
    // Go's sentinel carries the constant's NAME, not its value, and `reason`
    // is a public field a consumer or a shared fixture can match on.
    super("campfire listing exceeds MaxCampfireListing");
    this.name = "CampfireListingOverflow";
  }
}

/**
 * The id a discovery source carried, read the way Go's decoder reads it.
 *
 * Three outcomes, and the first version of this had two of them wrong.
 * `json.Unmarshal` into an `int64` treats an ABSENT key and a JSON `null` as
 * the zero value and succeeds — Go then skips a zero id at its own filter — so
 * neither is an error here either. A string, a fraction, or a number past
 * int64 fails Go's decode outright and takes the whole read with it, which is
 * the malformed-response shape SPEC §6 spells for a composite that cannot
 * proceed with a 2xx body: `api_error`, statusless, non-retryable.
 *
 * Getting the first two wrong turned "skip this dock entry" into a failure of
 * the entire summarize; getting the last wrong let a fractional id through to
 * a request for `/chats/1.5/lines/{id}`.
 *
 * TWO GAPS, both stated rather than papered over. Go refuses the LITERAL — its
 * decoder runs `strconv.ParseInt` over the raw token — so `1.0`, `1e2` and
 * `7.7e1` are decode errors there. By the time this runs, `JSON.parse` has
 * turned every one of those into the number 77, and the literal is gone: an
 * exponent or trailing-zero spelling of a whole number is accepted here and
 * refused there — `1.0` arrives as 1, `1e2` as 100, `7.7e1` as 77 — and this
 * layer, which receives an already-parsed value, cannot tell any of them from
 * the literal Go refuses. Not an impossibility, a placement: on the supported
 * Node floor a `JSON.parse` reviver can read `context.source` and see the
 * spelling (SPEC §19 says so for the large-integer case), so a decoder that
 * wanted these distinctions could have them. This composite does not reach for
 * one, because it would mean parsing every response body twice to refuse a
 * spelling no BC3 endpoint emits. And
 * the upper bound is 2^53, not int64, which is the same limit SPEC §19 waives for this SDK: an id
 * past it cannot be held without rounding it into a different id, so refusing
 * it is the honest answer even though Go decodes it.
 *
 * The int64 window's own two ends are not symmetric after rounding, and the
 * message says so rather than pretending otherwise: 9223372036854775808 rounds
 * to exactly 2^63 and is refused, while -9223372036854775809 — a decode error
 * in Go — rounds to exactly -2^63, a legal value, and nothing at this layer can
 * tell the two apart. One end is measurable after JSON.parse and the other is
 * not.
 */
/**
 * The body of a routed read, refused when it is not a recording object.
 *
 * Go decodes each read into a struct, so a body that is not a JSON object never
 * reaches the projection: it fails the read. TypeScript decodes nothing, and
 * the cost is not a cosmetic one — the arms dereference the body immediately
 * (`c.title`), so a bare `"hello"` projected every scalar to its default and,
 * worse, left `bucket` absent, which is how a recording in ANOTHER project came
 * back as a match instead of a `bucket_mismatch`. A malformed body decided a
 * verdict, which is the line at which this port replicates Go's decoder.
 *
 * `null` is the exception, and it is measured rather than assumed: `json.Unmarshal`
 * takes a JSON null into a struct as a no-op, so Go returns the ZERO recording
 * and no error. That is projected here as the zero recording too — and it is
 * also the shape that used to throw a raw TypeError out of the SDK, since the
 * arm dereferenced it before anything could check. An array is a decode error
 * there, so it is refused here.
 */
function recordingBody<T>(value: T): T {
  if (value === null) return {} as T;
  if (typeof value !== "object" || Array.isArray(value)) {
    throw Errors.apiError(
      truncateErrorMessage(`the recording read returned ${describeWireValue(value)} rather than a recording`),
      undefined,
      {
        retryable: false,
        hint: "the response is malformed; the recording it names cannot be summarized from it",
      },
    );
  }
  return value;
}

/**
 * A nested object a discovery source carries, refused when it is not one.
 *
 * The id sweep covered the id FIELD at each site and not the container around
 * it, and the container has the same two failure modes with worse consequences.
 * Measured through the generated structs: a dock or a listing entry that is not
 * an object fails Go's decode outright, and a JSON `null` is the zero struct —
 * whose empty name and zero ids each site already skips. TypeScript did neither:
 * a non-object was read for fields it does not have and dropped in silence, and
 * a `null` threw a raw TypeError out of the SDK on a body Go reads happily.
 */
function sourceObject(value: unknown, what: string, hint = DISCOVERY_HINT): Record<string, unknown> {
  if (value === null || value === undefined) return {};
  if (typeof value !== "object" || Array.isArray(value)) {
    throw Errors.apiError(
      truncateErrorMessage(`${what} is ${describeWireValue(value)} rather than an object`),
      undefined,
      { retryable: false, hint },
    );
  }
  return value as Record<string, unknown>;
}

/**
 * The hints these helpers carry. Two, because the same shape checks run on two
 * different reads: a Campfire discovery source, and the recording itself.
 *
 * The projection borrowed the discovery wording for a while, so a malformed
 * `assignees` on a to-do read advised the caller about Campfires. Fixing that in
 * the container helpers left it unfixed one helper along, in `wireInteger` and
 * `numericId`, whose recording-side callers went on saying it — which is why
 * this is a parameter on every one of them rather than a constant inside the
 * two that were noticed first.
 */
const DISCOVERY_HINT = "the discovery source's response is malformed; no Campfire can be read from it";
const RECORDING_HINT = "the response is malformed; the recording cannot be summarized from it";

/** {@link sourceItems}, for a list read off the recording rather than a source. */
function wireItems(value: unknown, what: string): Record<string, unknown>[] {
  return sourceItems(value, what, RECORDING_HINT);
}

/** The same, for a list of them: absent is empty, a non-array is a failed read. */
function sourceItems(value: unknown, what: string, hint = DISCOVERY_HINT): Record<string, unknown>[] {
  if (value === null || value === undefined) return [];
  if (!Array.isArray(value)) {
    throw Errors.apiError(
      truncateErrorMessage(`${what} is ${describeWireValue(value)} rather than a list`),
      undefined,
      { retryable: false, hint },
    );
  }
  return value.map((item) => sourceObject(item, `${what} entry`, hint));
}

/**
 * The whole number a wire field carries, or a failed read — the TYPE half only.
 *
 * Shared because Go's decoders agree on it everywhere: absent and JSON null are
 * the zero value, and anything that is not a whole number in an int64 fails the
 * read. What is NOT shared is what each site then does with the number, and a
 * helper that decided that too would be the tidy kind of wrong: a rule reading
 * like the reference's while applying one site's policy at another's. Each
 * caller spells its own value rule beside the Go line it mirrors.
 */
const INT64_LIMIT = 2 ** 63;

function wireInteger(value: unknown, what: string, hint = DISCOVERY_HINT): number {
  if (value === undefined || value === null) return 0;
  // The window is Go's int64, measured at both ends: 9223372036854775807 and
  // -9223372036854775808 decode, and one past either is a decode error. An
  // "any whole number" rule was wider than the reference at the top end, so a
  // bucket id of 1e20 — a body Go refuses — came back as a bucket_mismatch
  // naming a bucket that cannot exist.
  if (typeof value !== "number" || !Number.isInteger(value) || value >= INT64_LIMIT || value < -INT64_LIMIT) {
    throw Errors.apiError(
      truncateErrorMessage(`${what} has an id that is not a usable whole number (${describeIdValue(value)})`),
      undefined,
      { retryable: false, hint },
    );
  }
  return value;
}

function numericId(value: unknown, what: string, hint = DISCOVERY_HINT): number {
  const id = wireInteger(value, what, hint);
  if (!Number.isSafeInteger(id)) {
    throw Errors.apiError(
      truncateErrorMessage(`${what} has an id that is not a usable whole number (${describeIdValue(value)})`),
      undefined,
      { retryable: false, hint },
    );
  }
  return id;
}

/**
 * What a wire value IS, for a site that was not reading an id.
 *
 * `describeIdValue` speaks about ids — "out of safe integer range", "not whole"
 * — and reusing it for containers and text produced messages that send a reader
 * hunting for the wrong thing: "the project dock is out of safe integer range
 * rather than a list" for `{"dock": 5}`.
 */
function describeWireValue(value: unknown): string {
  if (value === null) return "null";
  if (Array.isArray(value)) return "an array";
  if (typeof value === "string") return JSON.stringify(value);
  if (typeof value === "object") return "an object";
  return `${typeof value} ${String(value)}`;
}

function describeIdValue(value: unknown): string {
  if (typeof value === "string") return JSON.stringify(value);
  // `typeof []` is "object", which names neither of the two shapes a caller
  // would have to tell apart to fix the response.
  if (Array.isArray(value)) return "an array";
  if (typeof value === "number") {
    // Two different complaints, and the first version made both of them the
    // second: 1.5 is perfectly in range and was carried faithfully — what is
    // wrong with it is that it is not whole. A value past 2^53 is the other
    // case, and there the number has already been rounded by JSON.parse, so
    // quoting it would report something the response never carried.
    if (Number.isInteger(value)) return "out of safe integer range";
    return `${value}, which is not whole`;
  }
  return typeof value;
}

/**
 * Whether an error is a read's own 404 — the one answer discovery treats as
 * "not here".
 *
 * The status is required, not just the code. Go reaches this through a
 * `*basecamp.Error` whose `CodeNotFound` can only have come from a 404
 * response; here `not_found` is also the code {@link UnresolvedRecordingError}
 * carries, deliberately statusless. Matching on the code alone would let a
 * composite's own verdict — thrown by a hook, a middleware, or a caller-supplied
 * read source — be read as "the line is not in this Campfire", and discovery
 * would move on and eventually report the line unresolved instead of surfacing
 * it. Every `not_found` the transport produces carries the status.
 */
function isNotFound(err: unknown): boolean {
  return isBasecampError(err) && err.code === "not_found" && err.httpStatus === 404;
}

/** What one candidate sweep found. */
interface ChatLineHit {
  line: CampfireLine;
  campfireId: number;
}

/** One `summarize` call's discovery state. */
class ChatLineSearch {
  /** Candidates that answered 404, in order. */
  readonly tried: number[] = [];
  /** Candidates still allowed. */
  budget = MAX_CAMPFIRE_CANDIDATES;
  /** Whether a candidate was left untried for want of budget. */
  skipped = false;

  constructor(
    private readonly sources: RecordingReadSources,
    private readonly lineId: number,
  ) {}

  /**
   * Reads the line under each candidate not yet tried. Returns the line and its
   * Campfire on a hit; on a miss returns `undefined` and records the candidates
   * in {@link tried}. Any answer but 404 is raised as is.
   */
  async try(candidates: readonly number[]): Promise<ChatLineHit | undefined> {
    for (const campfireId of candidates) {
      if (this.tried.includes(campfireId)) continue;
      if (this.budget <= 0) {
        this.skipped = true;
        return undefined;
      }
      this.budget--;
      try {
        const line = await this.sources.campfires.getLine(campfireId, this.lineId);
        return { line, campfireId };
      } catch (err) {
        if (isNotFound(err)) {
          this.tried.push(campfireId);
          continue;
        }
        throw err;
      }
    }
    return undefined;
  }
}

// =============================================================================
// Service
// =============================================================================

/**
 * `RecordingsService` with the hand-written `summarize` composite on top of the
 * generated surface (`list`, `trash`, ...).
 *
 * The composite reaches its sibling reads through the client that built it, so
 * every request is the same public generated method a caller would have made
 * and hooks see each under its own operation identity.
 */
export class RecordingsService extends GeneratedRecordingsService {
  readonly #sources?: () => RecordingReadSources;
  readonly #campfires: CampfireIndex;

  // Preserve BaseService's full positional signature, then append the read
  // sources — the shape uploads-extensions established, so an existing
  // `new RecordingsService(client, hooks, fetchPage, maxPages)` call keeps
  // working and `summarize()` explains itself when the sources were not wired.
  constructor(
    client: RawClient,
    hooks?: BasecampHooks,
    fetchPage?: (url: string) => Promise<Response>,
    maxPages?: number,
    authenticatedFetch?: (url: string, init: RequestInit) => Promise<Response>,
    baseUrl?: string,
    sources?: () => RecordingReadSources,
    now?: () => number,
  ) {
    super(client, hooks, fetchPage, maxPages, authenticatedFetch, baseUrl);
    this.#sources = sources;
    // The index lives on the service, which the client memoizes: the dock and
    // listing snapshots are shared by every summarize() through one client and
    // by nothing across two, which is the scope Go gives them on its Client.
    this.#campfires = new CampfireIndex(now);
  }

  /**
   * Resolves a recording pointer into a {@link RecordingSummary} through the
   * typed read its type names. See {@link RecordingRef} for the routing inputs.
   *
   * Errors: a routing failure (`no_recording_type` / `unknown_recording_type`)
   * before any request; the read's own {@link BasecampError} otherwise — a 404
   * is `not_found`, as from the typed read itself; for chat lines,
   * {@link UnresolvedRecordingError} when every visible Campfire answered 404,
   * which is distinct from a read that failed (any non-404 from a candidate is
   * raised as that error, and the loop stops there) and from
   * {@link CampfireDiscoveryIncompleteError} (candidates were left unsearched);
   * {@link BucketMismatchError} when the read returned a recording from another
   * bucket.
   *
   * @example
   * ```ts
   * const summary = await client.recordings.summarize({
   *   bucketId: 2085958499,
   *   recordingId: 1069479361,
   *   eventType: "comment.created",
   * });
   * summary.mentioned_person_ids; // [1049715915]
   * ```
   */
  async summarize(ref: RecordingRef): Promise<RecordingSummary> {
    // `isSafeInteger`, not `isInteger`, and for two different reasons — one per
    // id, both worth stating because a reader checking only the first would
    // think the second was along for the ride.
    //
    // `recordingId` is ADDRESSED: it is interpolated into every read this
    // composite makes, so a rounded value fetches a DIFFERENT recording and
    // reports it as the one asked for. That is the same rule `numericId`
    // applies to a Campfire id, at the other end of the same URL.
    //
    // `bucketId` is COMPARED — the listing's bucket filter and the cross-project
    // check both weigh it against a number that arrived through JSON.parse, and
    // two int64 buckets can round onto one double, which is how a recording from
    // another project passed as a match — AND addressed, since chat-line
    // discovery reads `/projects/{bucketId}`. An earlier version of this
    // paragraph split the two ids by those two words, one each, in a commit
    // whose whole point was that a reason had covered only half of a check. It
    // covered half again.
    //
    // Go accepts both — its ids are int64 — so this is a refusal the reference
    // does not make, and it is this port's, not a reading of SPEC waiver 1B.6:
    // the waiver is about decoding ids the API sends, and says nothing about
    // caller input. A `usage` error because the caller supplied the value, and
    // a refusal rather than a silent rounding because the alternative is
    // answering about a recording nobody asked for.
    if (
      !Number.isSafeInteger(ref.bucketId) ||
      ref.bucketId <= 0 ||
      !Number.isSafeInteger(ref.recordingId) ||
      ref.recordingId <= 0
    ) {
      throw Errors.usage("bucket id and recording id are required");
    }
    const kind = routeRecording(ref);
    const summary = await this.#readSummary(ref, kind);
    // A bucket the read did not identify is Go's zero value: there is nothing
    // to disagree with, so the pointer stands rather than the call failing.
    // The bucket itself, not only its id: a string or an array where the object
    // belongs leaves `?.id` undefined, and "no bucket identified" is exactly the
    // answer that lets a recording in another project pass as a match. Go fails
    // the read on that body (measured: a string and an array are both decode
    // errors into the bucket struct; a null is the zero value, hence absent).
    // `"bucket" in summary`, not `!== undefined`: the projection assigns this
    // key on truthiness, so `"bucket": 0`, `false` and `""` never became a key
    // at all and slipped past a guard that asked whether the key was undefined.
    // Each of them is a decode error in Go, and each read here as "no bucket
    // identified" — the answer that lets a recording in another project pass as
    // a match, arrived at through the one door the first guard left open.
    const readBucket = summary.bucket as unknown;
    if (readBucket !== undefined && (readBucket === null || typeof readBucket !== "object" || Array.isArray(readBucket))) {
      throw Errors.apiError(
        truncateErrorMessage(`the recording read identified its bucket as ${describeWireValue(readBucket)}`),
        undefined,
        {
          retryable: false,
          hint: "the response is malformed; whether the recording is in the requested project cannot be decided from it",
        },
      );
    }
    // And the id, through the same gate as the three discovery sites — this one
    // reached it last. It began as `!== undefined`, which read a malformed id as
    // "no bucket"; then `typeof`, which still compared a FRACTION and could
    // report the recording as living in bucket 1.5; then `isInteger`, which
    // stopped fabricating that verdict but still answered "no bucket
    // identified" for a string, a boolean, an array or an object — and that
    // answer is the one that lets a recording in ANOTHER project pass as a
    // match. Go refuses every one of those bodies outright, and `numericId`
    // says exactly that: null and absent are its zero, which is skipped;
    // anything else that is not a whole number in range fails the read.
    // Go's line is `summary.Bucket != nil && summary.Bucket.ID != 0 &&
    // summary.Bucket.ID != ref.BucketID` — the type half from its decoder, then
    // a value rule of exactly "not zero, not the one asked for". `wireInteger`,
    // not `numericId`: the safe-integer clause belongs to the sites that BUILD
    // A URL from the id, where a rounded value addresses a different Campfire.
    // Nothing is addressed here; the id is only compared.
    //
    // An argument for comparing rounded ids stood here until it was deleted.
    // Not softened and not quoted: it argued for the behaviour two lines below
    // it, and a reader weighing prose against code would have believed the
    // prose. What replaced it lives in `summarize`, which refuses a caller id
    // this runtime cannot hold exactly.
    const readBucketId = wireInteger(summary.bucket?.id, "the recording read's bucket", RECORDING_HINT);
    // `!Number.isSafeInteger` is a MISMATCH, not a pass — and, like the listing
    // filter, unreachable while `ref.bucketId` must itself be a safe integer,
    // since a rounded value above 2^53 cannot equal one below it. The fix for
    // the defect is the check in `summarize`, not this clause; this states the
    // same fact where the comparison happens, and a revert of it is
    // behaviour-neutral.
    //
    // The defect: this used to compare the rounded values, with a comment
    // arguing they "round identically, so the comparison still means
    // something". It does not. Rounding is not injective, and a recording from
    // bucket 2^53+1 came back as a MATCH for a request against 2^53 — the one
    // thing this check exists to prevent, argued for in prose.
    if (readBucketId !== 0 && (!Number.isSafeInteger(readBucketId) || readBucketId !== ref.bucketId)) {
      throw new BucketMismatchError(ref, readBucketId);
    }
    return summary;
  }

  /** The sources, or a usage error naming how to obtain a wired service. */
  get #reads(): RecordingReadSources {
    const sources = this.#sources?.();
    if (sources === undefined) {
      throw Errors.usage(
        "recordings.summarize composes the other services' reads — obtain RecordingsService via createBasecampClient(...).recordings rather than instantiating it directly",
      );
    }
    return sources;
  }

  /** Performs the one typed read a kind names and projects it. */
  async #readSummary(ref: RecordingRef, kind: SummaryKind): Promise<RecordingSummary> {
    const reads = this.#reads;
    const id = ref.recordingId;

    switch (kind) {
      case "comment": {
        const c = recordingBody(await reads.comments.get(id));
        return projectRecording(c, c.title, c.content, { parent: c.parent, bucket: c.bucket, creator: c.creator });
      }
      case "message": {
        const m = recordingBody(await reads.messages.get(id));
        return projectRecording(m, firstNonEmpty("a message title", m.title, m.subject), m.content, {
          parent: m.parent,
          bucket: m.bucket,
          creator: m.creator,
        });
      }
      case "todo": {
        // A to-do's content is its plain title; the rich text — where mentions
        // live — is the description.
        const t = recordingBody(await reads.todos.get(id));
        return projectRecording(t, firstNonEmpty("a recording title", t.title, t.content), t.description, {
          parent: t.parent,
          bucket: t.bucket,
          creator: t.creator,
          assignees: t.assignees,
        });
      }
      case "card": {
        const c = recordingBody(await reads.cards.get(id));
        return projectRecording(c, c.title, firstNonEmpty("a recording rich text", c.content, c.description), {
          parent: c.parent,
          bucket: c.bucket,
          creator: c.creator,
          assignees: c.assignees,
        });
      }
      case "chatLine": {
        const { line: rawLine, campfireId } = await this.#resolveChatLine(ref.bucketId, id);
        const line = recordingBody(rawLine);
        const summary = projectRecording(line, line.title, line.content, {
          parent: line.parent,
          bucket: line.bucket,
          creator: line.creator,
        });
        if (!chatLineIsRichText(line.type)) {
          // A plain-text or code line's content is text BC3 never read as
          // markup, so a literal "<bc-attachment>" in it mentions nobody.
          summary.mentioned_person_ids = [];
        }
        // Omitted when zero, because Go's field is `omitempty` and a zero
        // marshals away there. A zero is reachable: the account listing keeps
        // an entry whose own id is null as a candidate (Go filters that source
        // on the bucket alone), so a Campfire 0 that answers 200 resolves the
        // line — and only then do the two SDKs' JSON differ by a key.
        if (campfireId !== 0) summary.campfire_id = campfireId;
        return summary;
      }
      case "document": {
        const d = recordingBody(await reads.documents.get(id));
        return projectRecording(d, d.title, d.content, { parent: d.parent, bucket: d.bucket, creator: d.creator });
      }
      case "upload": {
        const u = recordingBody(await reads.uploads.get(id));
        return projectRecording(u, firstNonEmpty("an upload title", u.title, u.filename), u.description, {
          parent: u.parent,
          bucket: u.bucket,
          creator: u.creator,
        });
      }
      case "scheduleEntry": {
        const e = recordingBody(await reads.schedules.getEntry(id));
        return projectRecording(e, firstNonEmpty("a recording title", e.title, e.summary), e.description, {
          parent: e.parent,
          bucket: e.bucket,
          creator: e.creator,
        });
      }
      case "question": {
        const q = recordingBody(await reads.checkins.getQuestion(id));
        return projectRecording(q, q.title, "", { parent: q.parent, bucket: q.bucket, creator: q.creator });
      }
      case "questionAnswer": {
        const a = recordingBody(await reads.checkins.getAnswer(id));
        return projectRecording(a, a.title, a.content, { parent: a.parent, bucket: a.bucket, creator: a.creator });
      }
      case "todolist": {
        const l = recordingBody(await reads.todolists.get(id));
        return projectRecording(l, firstNonEmpty("a recording title", l.title, l.name), l.description, {
          parent: l.parent,
          bucket: l.bucket,
          creator: l.creator,
        });
      }
      case "vault": {
        const v = recordingBody(await reads.vaults.get(id));
        return projectRecording(v, v.title, "", { parent: v.parent, bucket: v.bucket, creator: v.creator });
      }
      case "forward": {
        const f = recordingBody(await reads.forwards.get(id));
        return projectRecording(f, firstNonEmpty("a message title", f.title, f.subject), f.content, {
          parent: f.parent,
          bucket: f.bucket,
          creator: f.creator,
        });
      }
      case "clientApproval": {
        const a = recordingBody(await reads.clientApprovals.get(id));
        return projectRecording(a, firstNonEmpty("a message title", a.title, a.subject), a.content, {
          parent: a.parent,
          bucket: a.bucket,
          creator: a.creator,
        });
      }
      case "clientCorrespondence": {
        const c = recordingBody(await reads.clientCorrespondences.get(id));
        return projectRecording(c, firstNonEmpty("a message title", c.title, c.subject), c.content, {
          parent: c.parent,
          bucket: c.bucket,
          creator: c.creator,
        });
      }
      case "googleDocument": {
        const g = recordingBody(await reads.googleDocuments.googleDocument(id));
        return projectRecording(g, g.title, g.description, { parent: g.parent, bucket: g.bucket, creator: g.creator });
      }
      case "cloudFile": {
        const f = recordingBody(await reads.cloudFiles.cloudFile(id));
        return projectRecording(f, f.title, f.description, { parent: f.parent, bucket: f.bucket, creator: f.creator });
      }
      case "cardStep": {
        const s = recordingBody(await reads.cardSteps.get(id));
        return projectRecording(s, s.title, "", {
          parent: s.parent,
          bucket: s.bucket,
          creator: s.creator,
          assignees: s.assignees,
        });
      }
      case "questionnaire": {
        const q = recordingBody(await reads.checkins.getQuestionnaire(id));
        return projectRecording(q, firstNonEmpty("a recording title", q.title, q.name), "", { bucket: q.bucket, creator: q.creator });
      }
      case "schedule": {
        const s = recordingBody(await reads.schedules.get(id));
        return projectRecording(s, s.title, "", { bucket: s.bucket, creator: s.creator });
      }
      case "todoset": {
        const t = recordingBody(await reads.todosets.get(id));
        return projectRecording(t, firstNonEmpty("a recording title", t.title, t.name), "", { bucket: t.bucket, creator: t.creator });
      }
      case "messageBoard": {
        const b = recordingBody(await reads.messageBoards.get(id));
        return projectRecording(b, b.title, "", { bucket: b.bucket, creator: b.creator });
      }
      case "cardTable": {
        const t = recordingBody(await reads.cardTables.get(id));
        return projectRecording(t, t.title, "", { bucket: t.bucket, creator: t.creator });
      }
      case "cardColumn": {
        const c = recordingBody(await reads.cardColumns.get(id));
        return projectRecording(c, c.title, c.description, {
          parent: c.parent,
          bucket: c.bucket,
          creator: c.creator,
        });
      }
      case "inbox": {
        const i = recordingBody(await reads.forwards.getInbox(id));
        return projectRecording(i, i.title, "", { bucket: i.bucket, creator: i.creator });
      }
      case "campfire": {
        const c = recordingBody(await reads.campfires.get(id));
        return projectRecording(c, c.title, "", { bucket: c.bucket, creator: c.creator });
      }
      default:
        // Unreachable for a routed kind — the switch is exhaustive over
        // SummaryKind — and kept anyway, as Go keeps its trailing routing
        // error. Without it a value that reached here outside the type system
        // would fall out of the switch as `undefined` and be dereferenced as a
        // summary, turning a routing failure into a TypeError.
        throw new RecordingRoutingError(ref, "unknown_recording_type");
    }
  }

  /**
   * Finds the Campfire a line lives in and reads it.
   *
   * A `chat.line.created` row carries the line's id and bucket, not its
   * Campfire, and the line read is `/chats/{campfireId}/lines/{lineId}`.
   * Candidates come from two sources, tried in order and each cached ten
   * minutes:
   *
   * 1. The bucket's project dock, whose `chat` tool is the project's Campfire:
   *    one project read per bucket, and the answer for every line posted in a
   *    project. Cached per bucket.
   * 2. The account-wide Campfire listing (BC3 has no per-bucket one), filtered
   *    to the bucket, for buckets that are not projects or whose dock did not
   *    hold the line. Cached per account, so a burst of lines costs one
   *    listing.
   *
   * The loop tries the line under each candidate until one answers, within one
   * total budget of {@link MAX_CAMPFIRE_CANDIDATES} per call.
   *
   * Two failure shapes are kept apart on purpose. A candidate that answers
   * anything but 404 — 401, 403, 5xx, a network error — stops the loop and is
   * raised as that error: the read failed, and trying the next Campfire would
   * only hide it. A 404 means "not here", so the loop moves on. Only when every
   * candidate said "not here" is the line unresolved
   * ({@link UnresolvedRecordingError}) — and before concluding that, the cached
   * sources are refreshed (subject to the floor) so a Campfire created after
   * the cache filled is tried too. Discovery that could not be completed — a
   * listing cut off at its cap, a bucket with more candidates than the budget —
   * is {@link CampfireDiscoveryIncompleteError}, never "unresolved": nothing
   * unsearched is ever reported absent — which is why a spent budget is read
   * against WHICH sources it was spent before: one already consulted is simply
   * not re-read, while one never reached leaves candidates unsearched and makes
   * the verdict incomplete.
   *
   * What HTTP cannot tell apart: BC3 answers 404 both for a line that is not in
   * a Campfire and for a Campfire the caller may no longer see. "Unresolved"
   * therefore means "under no Campfire the caller can currently see", and the
   * error reports the cached candidates that the refreshed sources no longer
   * list ({@link UnresolvedRecordingError.staleCampfireIds}) so a consumer can
   * see when visibility, not existence, is what changed.
   */
  async #resolveChatLine(bucketId: number, lineId: number): Promise<ChatLineHit> {
    const reads = this.#reads;
    const index = this.#campfires;
    const search = new ChatLineSearch(reads, lineId);
    const incomplete = (reason: string): CampfireDiscoveryIncompleteError =>
      new CampfireDiscoveryIncompleteError({ bucketId, recordingId: lineId, reason });

    // Pass 1: what the sources already hold — the dock (read if it must be),
    // then the listing only if it is cached. A listing fetch is the expensive,
    // slow request, and it is not made until the dock — including its refresh —
    // has had its say, so a listing that is down or over its cap never stands
    // between a project's line and the one project read that finds it.
    let dock = await index.dockCampfires(reads, bucketId, false);
    const fromDock = await search.try(dock.ids);
    if (fromDock !== undefined) return fromDock;

    let listed = index.cachedListedCampfires(bucketId);
    const listCached = listed !== undefined;
    if (listed !== undefined) {
      const fromCachedListing = await search.try(listed.ids);
      if (fromCachedListing !== undefined) return fromCachedListing;
    }

    // Pass 2: re-read the dock if it was served from cache (the floor may
    // decline), then fetch or refresh the listing. Whatever comes back is the
    // current snapshot of that source, whoever loaded it — another caller may
    // have populated or refreshed it in the meantime — so it always replaces
    // the pass-1 one; "refreshed" is whether a source the conclusion had
    // consulted is now newer than when it was consulted.
    //
    // Not when the budget is already spent: a re-read could return no candidate
    // this call may try, so it would cost a request that cannot help — and a
    // failure on it would replace the deterministic "incomplete" verdict with a
    // transient error a consumer retries forever.
    let refreshed = false;
    // No budget left means no re-read: a source already consulted cannot hand
    // this call a candidate it may try, so its refresh is skipped and the
    // conclusion stands on what was seen (refreshed false). A source NEVER
    // consulted is different — candidates may exist there unsearched — so
    // running out of budget before reaching it makes the verdict incomplete.
    //
    // Concluding incomplete on a spent budget unconditionally would be the
    // wrong fix: it would call a search that did examine both sources
    // unfinished. Concluding nothing would be the other wrong fix, and is what
    // this replaced — it fetched a listing that could not help, and a failure
    // on that fetch replaced a deterministic "incomplete" with a transient
    // error a consumer retries forever.
    if (search.budget > 0 && dock.cached) {
      const again = await index.dockCampfires(reads, bucketId, true);
      if (again.fetchedAt > dock.fetchedAt || !again.cached) refreshed = true;
      dock = again;
      const fromRefreshedDock = await search.try(dock.ids);
      if (fromRefreshedDock !== undefined) return fromRefreshedDock;
    }

    if (search.budget <= 0) {
      if (!listCached) {
        throw incomplete(
          `the candidate budget of ${MAX_CAMPFIRE_CANDIDATES} was spent before the account listing was consulted`,
        );
      }
    } else {
      let again: SourceRead;
      try {
        again = await index.listedCampfires(reads, bucketId, listCached);
      } catch (err) {
        if (err instanceof CampfireListingOverflow) throw incomplete(err.message);
        throw err;
      }
      if (listed !== undefined && (again.fetchedAt > listed.fetchedAt || !again.cached)) {
        refreshed = true;
      }
      listed = again;
      const fromListing = await search.try(listed.ids);
      if (fromListing !== undefined) return fromListing;
    }

    if (search.skipped) {
      throw incomplete(`more than ${MAX_CAMPFIRE_CANDIDATES} visible campfires in the bucket`);
    }

    const listedIds = listed?.ids ?? [];
    const staleCampfireIds = refreshed
      ? search.tried.filter((id) => !dock.ids.includes(id) && !listedIds.includes(id))
      : [];
    throw new UnresolvedRecordingError({
      bucketId,
      recordingId: lineId,
      campfireIds: search.tried,
      refreshed,
      staleCampfireIds,
    });
  }
}

// =============================================================================
// Projection
// =============================================================================

/** The recording fields every routed read shares. */
interface SummarizableRecording {
  id: number;
  status: string;
  type: string;
  app_url: string;
  updated_at: string;
}

/** The nested identities a read may carry. */
interface SummarizableNested {
  parent?: RecordingParent;
  bucket?: RecordingBucket;
  creator?: Person;
  assignees?: Person[];
}

/** Builds the projection, keeping only the keys the recording actually has. */
function projectRecording(
  recording: SummarizableRecording,
  title: string,
  content: string | undefined,
  nested: SummarizableNested,
): RecordingSummary {
  const body = recordingText(content, "a recording rich text");
  // The scalars default rather than carrying `undefined` through: they are
  // required on every routed shape, so a body omitting one is malformed, and
  // Go emits its zero value. Left undefined, JSON.stringify would drop the key
  // and a consumer would see a shape no other SDK produces. The three strings
  // default to ""; updated_at does not, because its Go type is not a string.
  const summary: RecordingSummary = {
    // `id` too, which the first pass at this missed even while the comment
    // above stated the rule: Go marshals a zero RecordingSummary as {"id":0,…}.
    // And through `wireInteger`, which the id sweep missed here while sweeping
    // the three discovery sites and every string field around it: this was the
    // one number field left on a bare `??`, so `{"id":"1"}` came back as the
    // STRING "1" in a field typed `number`, a shape no other SDK can produce.
    // `wireInteger`, not `numericId`: this id is emitted, never addressed.
    id: wireInteger(recording.id, "the recording", RECORDING_HINT),
    status: recordingText(recording.status, "a recording status"),
    type: recordingText(recording.type, "a recording type"),
    // The types that read `title` straight off the recording pass it through
    // undefined when the body omits it; the ones with a fallback already
    // resolve to "" through firstNonEmpty.
    title: recordingText(title, "a recording title"),
    app_url: recordingText(recording.app_url, "a recording app_url"),
    mentioned_person_ids: mentionedPersonIds(body),
    content: body,
    // Go's zero value here is a `time.Time`, not a string, and it marshals as
    // the zero instant rather than as "".
    updated_at: recordingInstant(recording.updated_at),
  };
  // Each nested identity through the same gate as the bucket, and for the same
  // reason the sweep gave when it stopped at the bucket: measured through
  // generated.Comment and generated.Todo, a number, a string, a boolean or an
  // array in any of these slots is a decode error, while null and {} are the
  // zero value. Two of the four had no rule at all, three lines from the one
  // that did — the sweep stopped one field short of where it said it stopped.
  const parent = nestedIdentity(nested.parent, "a recording parent");
  if (parent !== undefined) summary.parent = parent;
  // Assigned when PRESENT, not when truthy: a falsy non-object (0, false, "")
  // is a decode error in Go, and dropping the key here hid it from the
  // cross-project check, which then found nothing to disagree with.
  if (nested.bucket !== undefined && nested.bucket !== null) summary.bucket = nested.bucket;
  const creator = nestedIdentity(nested.creator, "a recording creator");
  if (creator !== undefined) summary.creator = creator;
  // `assignees` is a LIST of them: Go refuses a non-array and a non-object
  // element, and takes null as the nil slice. Go's own `omitempty` is why an
  // empty list is dropped rather than emitted.
  // `wireItems` does the whole job: absent and null are the empty list, a
  // non-array fails the read, and each element goes through the same object
  // check the other three identities use. An earlier spelling wrapped every
  // element in `nestedIdentity` as well and credited the elementwise refusal to
  // it — that call could refuse nothing, having been handed an object already.
  const assignees = wireItems(nested.assignees, "a recording assignees list") as NonNullable<
    typeof nested.assignees
  >;
  if (assignees.length > 0) summary.assignees = assignees;
  return summary;
}

function firstNonEmpty(what: string, ...values: (string | undefined)[]): string {
  // EVERY candidate, not just the ones reached. Go's decoder refuses a
  // wrong-typed field whether or not the projection goes on to use it, so
  // `{"title":"T","subject":42}` fails the read there while a short-circuit
  // here returned "T" and never looked at the subject. Validate first, choose
  // after.
  const texts = values.map((value) => recordingText(value, what));
  for (const value of texts) {
    // A JSON null reads as Go's zero value and falls THROUGH to the next
    // candidate. Treating null as present is the quiet version of this defect:
    // {"title":null,"subject":"S"} decodes in Go with an empty title and takes
    // the subject, while here null passed the test, was returned, and the
    // caller's `?? ""` turned it into "" — the fallback lost, and a message
    // summarized under no title at all.
    if (value !== "") return value;
  }
  return "";
}

/**
 * A string field a routed read carries, refused when it is not one.
 *
 * Go's generated models decode these as `string`: a JSON null is the zero value
 * and no error, and a number, a boolean, an object or an array fails the read.
 * TypeScript carried whatever arrived into the projection, so `content: 42`
 * reached `mentionedPersonIds`, whose first string operation threw a raw
 * TypeError out of `summarize` — the same untyped-throw boundary the body and
 * container checks close, one level further in.
 */
/** Go's zero `time.Time`, which is what it marshals for an absent instant. */
const ZERO_INSTANT = "0001-01-01T00:00:00Z";
// Measured against Go's decoder rather than read off RFC 3339, in three places
// where it is wider than the grammar suggests: the fraction may be separated by
// a COMMA as well as a dot (`…05,5Z` decodes, and re-marshals as `.5`); the
// offset's range is wider than a clock, hour 00 to 24 and minute 00 to 60, with
// 25 and 61 refused; and the HOUR is the one field Go parses at variable width
// (`stdHour` calls `getnum` with `fixed=false`), so `T3:04:05Z` decodes and
// re-marshals as `T03:04:05Z`. Measured alongside it: a single-digit month,
// day, minute, second or offset component is refused, so `\d{1,2}` belongs to
// the hour alone.
const RFC3339 = /^(\d{4})-(\d{2})-(\d{2})T(\d{1,2}):(\d{2}):(\d{2})(?:[.,]\d+)?(?:Z|[+-](\d{2}):(\d{2}))$/;

/**
 * The instant a routed read carries, refused when it is not one.
 *
 * `updated_at` is the exception among the projected scalars, and treating it as
 * one more string got it wrong in both directions. Its Go type is `time.Time`,
 * whose `UnmarshalJSON` is stricter than a string's: a JSON null is the zero
 * instant and no error, but `""`, `"not-a-date"`, `"2024-01-02 03:04:05"`,
 * `"2024-02-30T…"` and a lowercase `t` or `z` are all decode errors. The
 * previous spelling — the string rule, then `|| ZERO_INSTANT` — painted `""`
 * with the zero instant, so a body the reference REFUSES came back carrying a
 * plausible timestamp instead of a visibly empty one.
 *
 * What this does not model is the re-rendering: Go parses and re-marshals, so
 * `2024-01-02T03:04:05.000Z` comes back as `…:05Z` there and verbatim here, and
 * `T3:04:05Z` as `T03:04:05Z`. The conformance runner compares instants rather
 * than strings — but only for the forms its own regex admits, and these are
 * not all of them. Five accepted spellings render differently here than in Go:
 * `,5` (Go re-renders `.5`), `T3:` (`T03:`), `T3:` with an offset, `+24:00`
 * (which Go's ENCODER then refuses outright, its own offset range being
 * [0,23]), and `+00:60` (`+01:00`). The runner's `sameInstant` demands a
 * two-digit hour, a dot fraction and a parseable `Date`, so all five fall back
 * to exact string comparison, where this port's verbatim output and Go's
 * re-rendering differ. Nothing in the fixture carries one — checked, every
 * `updated_at` there is canonical — so the difference is invisible in practice
 * rather than by construction.
 *
 * Accepting them is still right: refusing a body the reference reads is the
 * worse error, and this composite reports what the API served. The leniency is
 * the stdlib's own — `format_rfc3339.go` disables strict parsing behind a
 * `case true:` and a TODO about a GODEBUG opt-out — so if that lands, these
 * become decode errors there and this rule follows the measurement again.
 */
function recordingInstant(value: unknown): string {
  if (value === undefined || value === null) return ZERO_INSTANT;
  const text = recordingText(value, "a recording updated_at");
  const parts = RFC3339.exec(text);
  if (parts !== null) {
    const year = Number(parts[1]);
    const month = Number(parts[2]);
    const day = Number(parts[3]);
    const hour = Number(parts[4]);
    const minute = Number(parts[5]);
    const second = Number(parts[6]);
    const offsetHour = parts[7] === undefined ? 0 : Number(parts[7]);
    const offsetMinute = parts[8] === undefined ? 0 : Number(parts[8]);
    // The shape is not enough: 2024-02-30 and 25:04 match it and Go refuses
    // them. A UTC round-trip is what rejects a date the calendar does not have
    // — with `setUTCFullYear` after it, because `Date.UTC(1, …)` means 1901,
    // not year 1, and the two-digit-year mapping made this refuse
    // "0001-01-01T00:00:00Z": THE ZERO INSTANT, the value Go marshals for an
    // absent time and the one this very function returns for a null. It was not
    // idempotent on its own output.
    const at = new Date(Date.UTC(2000, month - 1, day, hour, minute, second));
    at.setUTCFullYear(year);
    if (
      at.getUTCFullYear() === year &&
      at.getUTCMonth() === month - 1 &&
      at.getUTCDate() === day &&
      at.getUTCHours() === hour &&
      at.getUTCMinutes() === minute &&
      at.getUTCSeconds() === second &&
      offsetHour <= 24 &&
      offsetMinute <= 60
    ) {
      return text;
    }
  }
  throw Errors.apiError(
    truncateErrorMessage(`a recording updated_at is ${describeWireValue(value)} rather than an RFC 3339 instant`),
    undefined,
    { retryable: false, hint: "the response is malformed; the recording cannot be summarized from it" },
  );
}

/**
 * A nested identity the projection carries verbatim, or `undefined` for Go's
 * zero value. A shape Go's decoder refuses fails the read here too.
 */
function nestedIdentity<T>(value: T, what: string): T | undefined {
  if (value === undefined || value === null) return undefined;
  if (typeof value !== "object" || Array.isArray(value)) {
    throw Errors.apiError(
      truncateErrorMessage(`${what} is ${describeWireValue(value)} rather than an object`),
      undefined,
      { retryable: false, hint: "the response is malformed; the recording cannot be summarized from it" },
    );
  }
  return value;
}

function recordingText(value: unknown, what: string): string {
  if (value === undefined || value === null) return "";
  if (typeof value !== "string") {
    throw Errors.apiError(
      truncateErrorMessage(`${what} is ${describeWireValue(value)} rather than text`),
      undefined,
      { retryable: false, hint: "the response is malformed; the recording cannot be summarized from it" },
    );
  }
  return value;
}
