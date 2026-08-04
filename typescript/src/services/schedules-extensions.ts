import { SchedulesService as GeneratedSchedulesService } from "../generated/services/schedules.js";
import type {
  ReplaceEntryScheduleRequest,
  ScheduleEntry,
} from "../generated/services/schedules.js";
import {
  requireRecord,
  requiredWritableBoolean,
  requiredWritableString,
  writableBoolean,
  writableIdList,
  writableString,
} from "./merge-safe.js";

/** The deliberate-overwrite escape hatch named in this composite's error hints. */
const ESCAPE = "replaceEntry()";

/**
 * How the guards name this record in their messages. Spelled with a space
 * because {@link requireRecord} lowercases it into a sentence ("...where a
 * schedule entry object was expected"); "ScheduleEntry" would read as
 * "scheduleentry" there.
 */
const RECORD = "Schedule entry";

const guardOptions = { record: RECORD, escape: ESCAPE };

/**
 * Request parameters for `updateEntry`.
 *
 * **Addressedness is property presence, not value.** Every field is optional;
 * a key you leave off the object is not addressed, and a key you set is —
 * including `{ highlighted: false }`, `{ url: "" }` and
 * `{ participantIds: [] }`, each of which is a deliberate clear and reaches the
 * wire. That distinction is the whole point of the carve-out contract below,
 * so it is tested with `Object.prototype.hasOwnProperty.call`, never with a
 * truthiness test.
 *
 * The fields split in two, and the split is not cosmetic:
 *
 * - **full state** — `summary`, `startsAt`, `endsAt`, `description`, `allDay`.
 *   Always sent, seeded from the read-back when you do not address them,
 *   because BC3 rebuilds the entry from the submitted params and clears every
 *   writable field the body omits.
 * - **addressed-only** — `participantIds`, `url`, `highlighted`, `notify`.
 *   Sent only when you address them, and never seeded from the read-back. BC3
 *   preserves the first three on omission (`PRESERVED_ON_OMISSION`), so
 *   resending them is redundant at best and wrong if the read raced a
 *   concurrent change. `notify` is a directive rather than state: sending it
 *   makes BC3 recompute a drafted entry's subscriber list.
 */
export interface UpdateScheduleEntryRequest {
  /** Summary text. Omit to leave unchanged; `""` clears it. */
  summary?: string;
  /**
   * When the entry starts — a bare date (`"2026-06-05"`) for an all-day entry,
   * a timestamp otherwise. Omit to leave unchanged.
   */
  startsAt?: string;
  /** When the entry ends, in the same spelling as `startsAt`. Omit to leave unchanged. */
  endsAt?: string;
  /** Rich text description (HTML). Omit to leave unchanged; `""` clears it. */
  description?: string;
  /** Whether the entry occupies whole days. Omit to leave unchanged. */
  allDay?: boolean;
  /**
   * Replaces the entry's participants. Omit to leave them alone — BC3 preserves
   * them when the request does not address them — or pass `[]` to remove
   * everyone.
   *
   * Never resent on your behalf: the read-back's `participants` are people the
   * *reader* can see, and echoing them would rewrite the list from a stale view.
   */
  participantIds?: number[];
  /**
   * The entry's join link — a video-call URL or similar. Omit to leave it
   * alone; `""` clears it.
   *
   * Read back as `join_url`, never as `url`: an entry's `url` is its own
   * Basecamp API URL. The composite therefore never seeds this from the
   * response, which would write the API URL into the join link.
   */
  url?: string;
  /** Whether the entry is highlighted on the schedule. Omit to leave it alone; `false` removes it. */
  highlighted?: boolean;
  /**
   * Whether to notify the relevant people. A directive, not state: it is sent
   * only when you address it, and it is never read back.
   */
  notify?: boolean;
}

/**
 * A schedule entry's full writable state, handed to the `editEntry` callback.
 *
 * The five full-state members are seeded from the read-back and **always** PUT
 * back, so clearing one means setting it empty (`""`) — there is no third
 * state.
 *
 * The four carve-outs behave differently and deliberately: they are seeded for
 * *reading* (so you can inspect the current join link, highlight and
 * participants before deciding), but they reach the wire **only if you assign
 * to them**. Assignment is what marks them, not the value you assign — setting
 * `e.url = e.url` sends the join link, because intent is not recoverable from a
 * value comparison.
 */
export interface ScheduleEntryFields {
  /** Summary text. Set `""` to clear — the entry then reads back as "Untitled". */
  summary: string;
  /** Start date or timestamp, exactly as the server spelled it. */
  startsAt: string;
  /** End date or timestamp, exactly as the server spelled it. */
  endsAt: string;
  /** Rich text description (HTML). Set `""` to clear. */
  description: string;
  /** Whether the entry occupies whole days. */
  allDay: boolean;
  /** Current participants, by person ID. Assign to replace them; `[]` removes everyone. */
  participantIds: number[];
  /** The current join link (the response's `join_url`), or `""`. Assign to change it. */
  url: string;
  /** Whether the entry is currently highlighted. Assign to change it. */
  highlighted: boolean;
  /**
   * Whether to notify the relevant people. Write-only — the API reports no such
   * field, so this reads back `undefined` until you assign to it.
   */
  notify?: boolean;
}

/** The carve-out members, keyed as they appear on {@link ScheduleEntryFields}. */
const CARVE_OUTS = ["participantIds", "url", "highlighted", "notify"] as const;
type CarveOut = (typeof CARVE_OUTS)[number];

/** The subset of a replace request the carve-outs contribute. */
type CarveOutRequest = Pick<ReplaceEntryScheduleRequest, CarveOut>;

/**
 * SchedulesService with merge-safe `updateEntry` and read-modify-write
 * `editEntry` on top of the generated surface (`getEntry`, `replaceEntry`,
 * `createEntry`, `listEntries`, ...).
 *
 * `PUT /{accountId}/schedule_entries/{entryId}` is a full replace:
 * `Schedules::EntriesController#update` rebuilds the recordable from the
 * submitted params, so a sparse PUT that omits `description` erases it and one
 * that omits `all_day` turns an all-day event into a midnight-to-midnight timed
 * one. Neither is a 422 — both are a `200` that quietly clears, which is what
 * these composites exist to prevent.
 *
 * Three writable fields are exempt. BC3 seeds `participant_ids`, `url` and
 * `highlighted` from the existing recordable when the request does not address
 * them, so the SDK's job for those is the opposite one: keep them **off** the
 * wire unless the caller asked for them.
 *
 * Both composites compose the public `getEntry` and `replaceEntry`, so hooks
 * observe the two wire operations under their native identities
 * (`GetScheduleEntry` then `ReplaceScheduleEntry`) rather than a synthetic
 * composite.
 *
 * **Non-recurring entries only.** `ensure_non_recurring_event` 302-redirects
 * both `show` and `update` for a recurring entry, and the SDK follows neither:
 * the GET surfaces as an unexpected body and the PUT as a redirect. Recurrence
 * itself (`recurrence_schedule`, `recurs_until`, `time_zone_name`) is unmodeled
 * — BC3 forces all three to nil for a non-recurring entry.
 */
export class SchedulesService extends GeneratedSchedulesService {
  /**
   * Sets the given fields on a schedule entry and preserves everything else:
   * GETs the current entry, overlays the fields you addressed, and PUTs the
   * full representation back.
   *
   * A key you leave off the request is not addressed. For the five full-state
   * fields that means "keep what the server has"; for the four carve-outs it
   * means the key never appears in the body, so BC3's own preservation applies.
   * An explicitly-passed `""`, `false` or `[]` is an address and clears.
   *
   * Not atomic: there is no conditional-update signal on this endpoint, so a
   * concurrent write between the GET and PUT is overwritten — last write wins
   * for the full-state fields. The window is one round-trip. Use `replaceEntry`
   * to overwrite deliberately.
   *
   * @param entryId - The entry ID
   * @param req - Fields to set; unaddressed fields are preserved
   * @returns The updated ScheduleEntry
   * @throws {BasecampError} If the request fails, or if the read-back is malformed
   *
   * @example
   * ```ts
   * // Rename without erasing the description, times or all-day flag.
   * await client.schedules.updateEntry(123, { summary: "Team Meeting & Kickoff" });
   *
   * // Drop the join link and the highlight; participants are left alone.
   * await client.schedules.updateEntry(123, { url: "", highlighted: false });
   * ```
   */
  async updateEntry(entryId: number, req: UpdateScheduleEntryRequest): Promise<ScheduleEntry> {
    const current = await this.currentState(entryId);
    const addressed = (key: keyof UpdateScheduleEntryRequest): boolean =>
      Object.prototype.hasOwnProperty.call(req, key);

    // Full state: always sent. `??` covers the one degenerate presence case —
    // a key set to `undefined` (or, from untyped JS, `null`). Neither can ride
    // the wire, so honouring it as an address would drop the key and hand BC3
    // the clear-by-omission this composite exists to prevent; the read-back
    // value is the answer instead.
    const state: ScheduleEntryState = {
      summary: addressed("summary") ? (req.summary ?? current.summary) : current.summary,
      startsAt: addressed("startsAt") ? (req.startsAt ?? current.startsAt) : current.startsAt,
      endsAt: addressed("endsAt") ? (req.endsAt ?? current.endsAt) : current.endsAt,
      description: addressed("description")
        ? (req.description ?? current.description)
        : current.description,
      allDay: addressed("allDay") ? (req.allDay ?? current.allDay) : current.allDay,
    };

    // Addressed-only: never seeded, so presence alone decides. `false`, `""`
    // and `[]` all pass, which is the point — they are clears, and BC3 would
    // preserve rather than clear if the key went missing.
    const carveOuts: CarveOutRequest = {};
    if (addressed("participantIds")) carveOuts.participantIds = req.participantIds;
    if (addressed("url")) carveOuts.url = req.url;
    if (addressed("highlighted")) carveOuts.highlighted = req.highlighted;
    if (addressed("notify")) carveOuts.notify = req.notify;

    return this.putEntry(entryId, state, carveOuts);
  }

  /**
   * Applies a read-modify-write callback to a schedule entry: GETs the current
   * entry, hands the callback its writable state, and PUTs the whole thing
   * back. If the callback throws (or rejects), the edit aborts and nothing is
   * written.
   *
   * The five full-state fields always ride back, so clearing one means setting
   * it empty and an untouched one keeps its current value. The four carve-outs
   * (`participantIds`, `url`, `highlighted`, `notify`) are seeded so you can
   * read them, but they reach the wire only if the callback **assigns** to
   * them. Assignment is tracked by invocation, not by comparing values:
   * `e.url = e.url` is a write, because a caller who reassigns the value the
   * read returned still means "write this", and no diff can recover that
   * intent.
   *
   * Not atomic: there is no conditional-update signal on this endpoint, so a
   * concurrent write between the GET and PUT is overwritten — last write wins
   * for the full-state fields. The window is one round-trip. Use `replaceEntry`
   * to overwrite deliberately.
   *
   * @param entryId - The entry ID
   * @param fn - Callback that mutates the entry's writable fields in place
   * @returns The updated ScheduleEntry
   * @throws {BasecampError} If the request fails, or if the read-back is malformed
   *
   * @example
   * ```ts
   * await client.schedules.editEntry(123, (e) => {
   *   e.summary = `🚨 ${e.summary}`;
   *   e.description = ""; // clearing = setting empty on a full object
   *   // participants, join link and highlight are untouched, so they stay off
   *   // the wire and BC3 keeps them.
   * });
   * ```
   */
  async editEntry(
    entryId: number,
    fn: (e: ScheduleEntryFields) => void | Promise<void>
  ): Promise<ScheduleEntry> {
    const fields = await this.currentFields(entryId);

    // Setter-invocation dirty tracking. A snapshot diff would be wrong, not
    // merely different: the callback that assigns exactly what the GET returned
    // has still asked for a write, and dropping it hands the field back to
    // BC3's preserve-on-omission.
    const touched = new Set<string>();
    const view = new Proxy(fields, {
      set(target, property, value): boolean {
        if (typeof property === "string") touched.add(property);
        return Reflect.set(target, property, value);
      },
    });

    await fn(view);

    const carveOuts: CarveOutRequest = {};
    if (touched.has("participantIds")) carveOuts.participantIds = fields.participantIds;
    if (touched.has("url")) carveOuts.url = fields.url;
    if (touched.has("highlighted")) carveOuts.highlighted = fields.highlighted;
    if (touched.has("notify")) carveOuts.notify = fields.notify;

    return this.putEntry(entryId, fields, carveOuts);
  }

  /**
   * Fetches the entry and derives the full-state half of its writable state.
   *
   * Every value here is resent in the full-replace PUT, so every value is
   * validated before it is read. `?? ""` coalesces only `null` and `undefined`,
   * leaving corruption wide open: `false`, `0`, `[]`, `{}`, `42`, `true` and
   * friends would all be forwarded **verbatim** and written to the entry on a
   * call that never mentioned the field. Nothing below this rejects them —
   * `schema.d.ts` is erased at build time, so `ScheduleEntry` is a compile-time
   * claim about runtime data, which puts this composite with Python and Ruby
   * rather than with Go and Swift. See `merge-safe.ts` and #576.
   *
   * The guards differ per field because the spec models the fields
   * differently:
   *
   * - `summary` is `@required` and `Schedule::Entry#summary` is
   *   `super.presence || "Untitled"`, so blank is impossible from a healthy
   *   server and absent/null/blank is malformed.
   * - `starts_at`/`ends_at` are `@required` and `NOT NULL`. The value is a bare
   *   date for an all-day entry and a timestamp otherwise; it round-trips
   *   verbatim rather than through a `Date`, which would normalise the two
   *   spellings into one and rewrite the entry's shape.
   * - `all_day` is `@required`, `NOT NULL DEFAULT false`. It needs the boolean
   *   guard specifically because a truthiness test would read the legitimate
   *   `false` as missing — and defaulting a missing value to `false` converts
   *   an all-day event into a midnight-to-midnight timed one.
   * - `description` is optional and nullable both ways, so absent or null is a
   *   genuinely empty description.
   */
  private async currentState(entryId: number): Promise<ScheduleEntryState> {
    const current = await this.readEntry(entryId);
    return {
      summary: requiredWritableString(current, "summary", guardOptions),
      startsAt: requiredWritableString(current, "starts_at", guardOptions),
      endsAt: requiredWritableString(current, "ends_at", guardOptions),
      description: writableString(current, "description", guardOptions),
      allDay: requiredWritableBoolean(current, "all_day", guardOptions),
    };
  }

  /**
   * The full-state fields plus the carve-out seeds the `editEntry` callback can
   * read.
   *
   * `url` is seeded from the response's **`join_url`**, not its `url`: the
   * latter is the entry's own Basecamp API URL, written by
   * `recordings/_recording` before the entry partial renders. Echoing it into
   * the request's `url` would write the API URL into the join link.
   *
   * `updateEntry` deliberately does not come through here. It never seeds a
   * carve-out, so validating `participants` for it would refuse a write that
   * does not depend on them.
   */
  private async currentFields(entryId: number): Promise<ScheduleEntryFields> {
    const current = await this.readEntry(entryId);
    return {
      summary: requiredWritableString(current, "summary", guardOptions),
      startsAt: requiredWritableString(current, "starts_at", guardOptions),
      endsAt: requiredWritableString(current, "ends_at", guardOptions),
      description: writableString(current, "description", guardOptions),
      allDay: requiredWritableBoolean(current, "all_day", guardOptions),
      participantIds: writableIdList(current, "participants", guardOptions),
      url: writableString(current, "join_url", guardOptions),
      highlighted: writableBoolean(current, "highlighted", guardOptions),
    };
  }

  /**
   * GETs the entry and checks the envelope before any field is read.
   *
   * A successful GET can still return a scalar, an array or null; reading a
   * property off `null` throws a raw `TypeError` instead of the documented
   * statusless `api_error`, and reading one off an array or string silently
   * yields `undefined`, which the field guards would then take at face value.
   */
  private async readEntry(entryId: number): Promise<Record<string, unknown>> {
    return requireRecord(await this.getEntry(entryId), {
      record: RECORD,
      operation: "GetScheduleEntry",
      escape: ESCAPE,
    });
  }

  /**
   * PUTs the full writable state via `replaceEntry`, plus whichever carve-outs
   * the caller addressed.
   *
   * The five full-state fields are always named, empties included: on a
   * full-replace endpoint `""` is how a clear is expressed — never JSON null
   * (SPEC §18 body compaction), and never by omission, which would leave the
   * field to BC3's clear-by-default and read as an accident rather than an
   * intent. The carve-outs are the mirror image: an unaddressed one is absent
   * from `carveOuts`, so the generated request builder leaves it `undefined`
   * and it never reaches the wire.
   */
  private putEntry(
    entryId: number,
    state: ScheduleEntryState,
    carveOuts: CarveOutRequest
  ): Promise<ScheduleEntry> {
    return this.replaceEntry(entryId, {
      summary: state.summary,
      startsAt: state.startsAt,
      endsAt: state.endsAt,
      description: state.description,
      allDay: state.allDay,
      ...carveOuts,
    });
  }
}

/**
 * The always-sent half of a schedule entry's writable state.
 *
 * Internal: `editEntry` hands callers the wider {@link ScheduleEntryFields},
 * and `updateEntry` takes {@link UpdateScheduleEntryRequest}.
 */
interface ScheduleEntryState {
  summary: string;
  startsAt: string;
  endsAt: string;
  description: string;
  allDay: boolean;
}
