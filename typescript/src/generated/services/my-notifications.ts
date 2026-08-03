/**
 * MyNotifications service for the Basecamp API.
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

/** Notification entity from the Basecamp API. */
export type Notification = components["schemas"]["Notification"];

/**
 * Options for myNotifications.
 */
export interface MyNotificationsMyNotificationOptions {
  /** Page number for paginating through read items. Defaults to 1. This
operation is not auto-paginated in any SDK, so a page is returned as
asked for and later pages are not followed. */
  page?: number;
  /** Set to true to cap `bubble_ups` at 2 current bubble-ups and omit the
`scheduled_bubble_ups` key entirely. Defaults to false. Use the dedicated
bubble-ups endpoint (GetBubbleUps) to page through all current and
scheduled bubble-ups. */
  limitBubbleUps?: boolean;
}

/**
 * Options for bubbleUps.
 */
export interface BubbleUpsMyNotificationOptions extends PaginationOptions {
  /** Page number. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Request parameters for markAsRead.
 */
export interface MarkAsReadMyNotificationRequest {
  /** Array of readable_sgid values identifying the items to mark as read */
  readables: string[];
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for MyNotifications operations.
 */
export class MyNotificationsService extends BaseService {

  /**
   * Get the current user's notification inbox (the "Hey!" menu).
   * @param options - Optional query parameters
   * @returns The my_notification
   *
   * @example
   * ```ts
   * const result = await client.myNotifications.myNotifications();
   * ```
   */
  async myNotifications(options?: MyNotificationsMyNotificationOptions): Promise<components["schemas"]["GetMyNotificationsResponseContent"]> {
    const response = await this.request(
      {
        service: "MyNotifications",
        operation: "GetMyNotifications",
        resourceType: "my_notification",
        isMutation: false,
      },
      () =>
        this.client.GET("/my/readings.json", {
          params: {
            query: { page: options?.page, "limit_bubble_ups": options?.limitBubbleUps },
          },
        })
    );
    return response;
  }

  /**
   * Get the current user's current and scheduled bubble-ups (paginated, 50 per page).
   * @param options - Optional query parameters
   * @returns All Notification across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.myNotifications.bubbleUps();
   * ```
   */
  async bubbleUps(options?: BubbleUpsMyNotificationOptions): Promise<ListResult<Notification>> {
    return this.requestPaginated(
      {
        service: "MyNotifications",
        operation: "GetBubbleUps",
        resourceType: "bubble_up",
        isMutation: false,
      },
      () =>
        this.client.GET("/my/readings/bubble_ups.json", {
          params: {
            query: { page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Mark specified items as read
   * @param req - Resource request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.myNotifications.markAsRead({ readables: [1234] });
   * ```
   */
  async markAsRead(req: MarkAsReadMyNotificationRequest): Promise<void> {
    if (!req.readables) {
      throw Errors.validation("Readables is required");
    }
    await this.request(
      {
        service: "MyNotifications",
        operation: "MarkAsRead",
        resourceType: "resource",
        isMutation: true,
      },
      () =>
        this.client.PUT("/my/unreads.json", {
          body: {
            readables: req.readables,
          },
        })
    );
  }
}