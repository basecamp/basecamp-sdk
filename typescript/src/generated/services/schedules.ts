/**
 * Schedules service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** ScheduleEntry entity from the Basecamp API. */
export type ScheduleEntry = components["schemas"]["ScheduleEntry"];
/** Schedule entity from the Basecamp API. */
export type Schedule = components["schemas"]["Schedule"];

/**
 * Request parameters for replaceEntry.
 */
export interface ReplaceEntryScheduleRequest {
  /** Summary text */
  summary?: string;
  /** Starts at (RFC3339 (e.g., 2024-12-15T09:00:00Z)) */
  startsAt: string;
  /** Ends at (RFC3339 (e.g., 2024-12-15T09:00:00Z)) */
  endsAt: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Replaces the entry's participants.

Omitting this member preserves the current participants; sending an empty
array clears them. That guarantee is BC3-side and recent: until
basecamp/bc3#12425, `Schedules::EntriesController#update` called
`replace_participants` unconditionally, so any update omitting the key —
including the shape in BC3's own "Update a schedule entry" doc example —
silently removed every participant and notified each one. The controller
now guards on the request actually addressing participants. */
  participantIds?: number[];
  /** Whether the entry occupies whole days rather than a time range.

Not carved out, and the carve-out list is what makes that dangerous to
forget: `schedule_entries.all_day` is NOT NULL with a `false` default, so
omitting this member on a replace resets it — silently converting an
all-day entry into a midnight-to-midnight timed one. The SDK's merge-safe
update and edit resend it from the read-back for exactly this reason.

Sending an explicit null is worse than omitting it: the column rejects
NULL, so BC3 raises rather than falling back to the default. The same is
true of highlighted. */
  allDay?: boolean;
  /** Whether to send notifications to relevant people */
  notify?: boolean;
  /** The entry's join link — a video-call URL or similar, up to 2500
characters, validated as a URL when present.

Omitting this member preserves the current join link; sending an empty
string clears it. Read it back as `join_url`, never as `url`: the entry's
`url` is its own Basecamp API URL, written by a partial that renders
before this one, so BC3 emits the join link under a non-colliding key.
Echoing the response's `url` into this member would write the API URL into
the join link. */
  url?: string;
  /** Whether the entry is highlighted on the schedule.

Omitting this member preserves the current highlight; sending false
removes it. Preserved on omission because until basecamp/bc3#12502 the
field was writable but never returned, so no caller could resend it. */
  highlighted?: boolean;
}

/**
 * Request parameters for updateSettings.
 */
export interface UpdateSettingsScheduleRequest {
  /** Include due assignments */
  includeDueAssignments: boolean;
}

/**
 * Options for listEntries.
 */
export interface ListEntriesScheduleOptions extends PaginationOptions {
  /** Filter by status */
  status?: "active" | "archived" | "trashed";
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Request parameters for createEntry.
 */
export interface CreateEntryScheduleRequest {
  /** Summary text */
  summary: string;
  /** Starts at (RFC3339 (e.g., 2024-12-15T09:00:00Z)) */
  startsAt: string;
  /** Ends at (RFC3339 (e.g., 2024-12-15T09:00:00Z)) */
  endsAt: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Participant ids */
  participantIds?: number[];
  /** All day */
  allDay?: boolean;
  /** Whether to send notifications to relevant people */
  notify?: boolean;
  /** The entry's join link — a video-call URL or similar, up to 2500
characters, validated as a URL when present. A scheme-less value is
normalized to `https://`.

Spell it `url` on the way in and read it back as `join_url`: the response
key `url` is the entry's own Basecamp API URL, written by a partial that
renders before this field, so BC3 emits the join link under a
non-colliding name. Sending `join_url` instead is silently dropped by
strong parameters — the create succeeds with no join link.

Accepted on create since long before it was documented:
`Schedules::Entries::BaseController#base_schedule_entry_params` permits it
and `new_schedule_entry_params` passes it through unchanged for API
requests. Modeling it only on ReplaceScheduleEntry forced callers into a
three-request read-modify-write for a field the create already took — and
create is the notifying write, so participants learned about a video call
before its link existed. */
  url?: string;
  /** Whether the entry is highlighted on the schedule. Defaults to false.

Do not send an explicit null: `schedule_entries.highlighted` is NOT NULL,
so BC3 raises rather than falling back to the default. Omit it instead —
every SDK's request compactor already drops unset members. */
  highlighted?: boolean;
  /** Publication state at creation — `active|drafted`, defaulting to `active`
for an API create.

A top-level parameter, not part of the entry's attributes: `status` is a
Recording column, so `wrap_parameters` leaves it outside the
`schedule_entry` envelope and `Recording::StatusParam#status_param` reads
it directly. On create it accepts `drafted`, `active`, `archived` or
`trashed` and raises `ActionController::BadRequest` — a 400, not a 422 —
for anything else; the two documented values are the two worth sending.

Unlike messages and documents, schedule-entry drafts are not listed by
GetMyDrafts. */
  status?: string;
  /** Subscriptions */
  subscriptions?: number[];
  /** Visible to clients */
  visibleToClients?: boolean;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Schedules operations.
 */
export class SchedulesService extends BaseService {

  /**
   * Get a single schedule entry by id.
   * @param entryId - The entry ID
   * @returns The ScheduleEntry
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.schedules.getEntry(123);
   * ```
   */
  async getEntry(entryId: number): Promise<ScheduleEntry> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "GetScheduleEntry",
        resourceType: "schedule_entry",
        isMutation: false,
        resourceId: entryId,
      },
      () =>
        this.client.GET("/schedule_entries/{entryId}", {
          params: {
            path: { entryId },
          },
        })
    );
    return response;
  }

  /**
   * Replace a schedule entry with a new complete representation.
   * @param entryId - The entry ID
   * @param req - Schedule_entry request parameters
   * @returns The ScheduleEntry
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.schedules.replaceEntry(123, { startsAt: "2025-06-01T09:00:00Z", endsAt: "2025-06-01T09:00:00Z" });
   * ```
   */
  async replaceEntry(entryId: number, req: ReplaceEntryScheduleRequest): Promise<ScheduleEntry> {
    if (!req.startsAt) {
      throw Errors.validation("Starts at is required");
    }
    if (!req.endsAt) {
      throw Errors.validation("Ends at is required");
    }
    const response = await this.request(
      {
        service: "Schedules",
        operation: "ReplaceScheduleEntry",
        resourceType: "schedule_entry",
        isMutation: true,
        resourceId: entryId,
      },
      () =>
        this.client.PUT("/schedule_entries/{entryId}", {
          params: {
            path: { entryId },
          },
          body: {
            summary: req.summary,
            starts_at: req.startsAt,
            ends_at: req.endsAt,
            description: req.description,
            participant_ids: req.participantIds,
            all_day: req.allDay,
            notify: req.notify,
            url: req.url,
            highlighted: req.highlighted,
          },
        })
    );
    return response;
  }

  /**
   * Get a specific occurrence of a recurring schedule entry
   * @param entryId - The entry ID
   * @param date - The date
   * @returns The ScheduleEntry
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.schedules.getEntryOccurrence(123, "example");
   * ```
   */
  async getEntryOccurrence(entryId: number, date: string): Promise<ScheduleEntry> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "GetScheduleEntryOccurrence",
        resourceType: "schedule_entry_occurrence",
        isMutation: false,
        resourceId: entryId,
      },
      () =>
        this.client.GET("/schedule_entries/{entryId}/occurrences/{date}", {
          params: {
            path: { entryId, date },
          },
        })
    );
    return response;
  }

  /**
   * Get a schedule
   * @param scheduleId - The schedule ID
   * @returns The Schedule
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.schedules.get(123);
   * ```
   */
  async get(scheduleId: number): Promise<Schedule> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "GetSchedule",
        resourceType: "schedule",
        isMutation: false,
        resourceId: scheduleId,
      },
      () =>
        this.client.GET("/schedules/{scheduleId}", {
          params: {
            path: { scheduleId },
          },
        })
    );
    return response;
  }

  /**
   * Update schedule settings
   * @param scheduleId - The schedule ID
   * @param req - Schedule_setting update parameters
   * @returns The Schedule
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.schedules.updateSettings(123, { includeDueAssignments: true });
   * ```
   */
  async updateSettings(scheduleId: number, req: UpdateSettingsScheduleRequest): Promise<Schedule> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "UpdateScheduleSettings",
        resourceType: "schedule_setting",
        isMutation: true,
        resourceId: scheduleId,
      },
      () =>
        this.client.PUT("/schedules/{scheduleId}", {
          params: {
            path: { scheduleId },
          },
          body: {
            include_due_assignments: req.includeDueAssignments,
          },
        })
    );
    return response;
  }

  /**
   * List entries on a schedule
   * @param scheduleId - The schedule ID
   * @param options - Optional query parameters
   * @returns All ScheduleEntry across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.schedules.listEntries(123);
   *
   * // With options
   * const filtered = await client.schedules.listEntries(123, { status: "active" });
   * ```
   */
  async listEntries(scheduleId: number, options?: ListEntriesScheduleOptions): Promise<ListResult<ScheduleEntry>> {
    return this.requestPaginated(
      {
        service: "Schedules",
        operation: "ListScheduleEntries",
        resourceType: "schedule_entrie",
        isMutation: false,
        resourceId: scheduleId,
      },
      () =>
        this.client.GET("/schedules/{scheduleId}/entries.json", {
          params: {
            path: { scheduleId },
            query: { status: options?.status, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Create a new schedule entry
   * @param scheduleId - The schedule ID
   * @param req - Schedule_entry creation parameters
   * @returns The ScheduleEntry
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.schedules.createEntry(123, { summary: "example", startsAt: "2025-06-01T09:00:00Z", endsAt: "2025-06-01T09:00:00Z" });
   * ```
   */
  async createEntry(scheduleId: number, req: CreateEntryScheduleRequest): Promise<ScheduleEntry> {
    if (!req.summary) {
      throw Errors.validation("Summary is required");
    }
    if (!req.startsAt) {
      throw Errors.validation("Starts at is required");
    }
    if (!req.endsAt) {
      throw Errors.validation("Ends at is required");
    }
    const response = await this.request(
      {
        service: "Schedules",
        operation: "CreateScheduleEntry",
        resourceType: "schedule_entry",
        isMutation: true,
        resourceId: scheduleId,
      },
      () =>
        this.client.POST("/schedules/{scheduleId}/entries.json", {
          params: {
            path: { scheduleId },
          },
          body: {
            summary: req.summary,
            starts_at: req.startsAt,
            ends_at: req.endsAt,
            description: req.description,
            participant_ids: req.participantIds,
            all_day: req.allDay,
            notify: req.notify,
            url: req.url,
            highlighted: req.highlighted,
            status: req.status,
            subscriptions: req.subscriptions,
            visible_to_clients: req.visibleToClients,
          },
        })
    );
    return response;
  }
}