/**
 * EventFeed service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Options for pollEvents.
 */
export interface PollEventsEventFeedOptions {
  /** Entry point: a decimal event id (start after it; `0` replays served
history back to the epoch), or the literal `now` (skip history). Mutually
exclusive with `position` in practice; omit both to enter at the present. */
  since?: string;
  /** Resume token from a previous page's `position`. Opaque and signed; never
constructed or parsed client-side. */
  position?: string;
  /** Comma-separated event types from the catalog (e.g. `message.created,comment.created`). */
  types?: string;
  /** Comma-separated bucket (project) ids, at most 100. */
  buckets?: string;
  /** Comma-separated creator person ids, at most 100. */
  creators?: string;
  /** Comma-separated effective-performer ids (the agent on a delegated action,
else the creator), at most 100. The literal `self` means the request's own
effective actor and is resolved server-side before filtering. */
  performers?: string;
  /** Comma-separated effective-performer ids to exclude, at most 100; `self`
as on `performers`. `exclude_performers=self` is the loop guard for an
agent that acts on what it hears. */
  excludePerformers?: string;
  /** Comma-separated actor kinds: `agent`, `person`, or both. A filter, not a
default — agent activity is real account activity. */
  actorTypes?: string;
}

/**
 * Options for pollInbox.
 */
export interface PollInboxEventFeedOptions {
  /** Entry point: `0` (earliest retained), `now` (present), or a decimal item
id to start after. */
  since?: string;
  /** Resume token from a previous inbox page's `position`. */
  position?: string;
  /** Comma-separated addressing reasons: `mentioned`, `assigned`, `subscribed`,
`watched`, `pinged`, `boosted`. */
  reasons?: string;
  /** Comma-separated event types, as a narrowing filter. */
  types?: string;
  /** Comma-separated bucket ids, as a narrowing filter (at most 100). */
  buckets?: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for EventFeed operations.
 */
export class EventFeedService extends BaseService {

  /**
   * Poll the account event feed for events after a position (oldest first, strict event-id order, up to 100 per page).
   * @param options - Optional query parameters
   * @returns The feed_event
   *
   * @example
   * ```ts
   * const result = await client.eventFeed.pollEvents();
   * ```
   */
  async pollEvents(options?: PollEventsEventFeedOptions): Promise<components["schemas"]["PollEventsResponseContent"]> {
    const response = await this.request(
      {
        service: "EventFeed",
        operation: "PollEvents",
        resourceType: "feed_event",
        isMutation: false,
      },
      () =>
        this.client.GET("/events.json", {
          params: {
            query: { since: options?.since, position: options?.position, types: options?.types, buckets: options?.buckets, creators: options?.creators, performers: options?.performers, "exclude_performers": options?.excludePerformers, "actor_types": options?.actorTypes },
          },
        })
    );
    return response;
  }

  /**
   * Mint a short-lived stream ticket and the exact WebSocket URL to open a live event stream with.
   * @returns The stream_ticket
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.eventFeed.createStreamTicket();
   * ```
   */
  async createStreamTicket(): Promise<components["schemas"]["CreateStreamTicketResponseContent"]> {
    const response = await this.request(
      {
        service: "EventFeed",
        operation: "CreateStreamTicket",
        resourceType: "stream_ticket",
        isMutation: true,
      },
      () =>
        this.client.POST("/events/stream_ticket.json", {
        })
    );
    return response;
  }

  /**
   * Poll the authenticated agent's inbox for the items that addressed it (oldest first, strict item-id order); people receive 403.
   * @param options - Optional query parameters
   * @returns The inbox_item
   *
   * @example
   * ```ts
   * const result = await client.eventFeed.pollInbox();
   * ```
   */
  async pollInbox(options?: PollInboxEventFeedOptions): Promise<components["schemas"]["PollInboxResponseContent"]> {
    const response = await this.request(
      {
        service: "EventFeed",
        operation: "PollInbox",
        resourceType: "inbox_item",
        isMutation: false,
      },
      () =>
        this.client.GET("/inbox.json", {
          params: {
            query: { since: options?.since, position: options?.position, reasons: options?.reasons, types: options?.types, buckets: options?.buckets },
          },
        })
    );
    return response;
  }
}