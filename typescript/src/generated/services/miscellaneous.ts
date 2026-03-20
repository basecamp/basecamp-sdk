/**
 * Miscellaneous service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Request parameters for updateMyProfile.
 */
export interface UpdateMyProfileMiscellaneouRequest {
  /** Display name */
  name?: string;
  /** Email address */
  emailAddress?: string;
  /** Title */
  title?: string;
  /** Bio */
  bio?: string;
  /** Location */
  location?: string;
  /** Time zone name */
  timeZoneName?: string;
  /** First week day */
  firstWeekDay?: number;
  /** Time format */
  timeFormat?: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Miscellaneous operations.
 */
export class MiscellaneousService extends BaseService {

  /**
   * Update the current authenticated user's profile
   * @param req - My_profile update parameters
   * @returns void
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * await client.miscellaneous.updateMyProfile({ });
   * ```
   */
  async updateMyProfile(req: UpdateMyProfileMiscellaneouRequest): Promise<void> {
    await this.request(
      {
        service: "Miscellaneous",
        operation: "UpdateMyProfile",
        resourceType: "my_profile",
        isMutation: true,
      },
      () =>
        this.client.PUT("/my/profile.json", {
          body: {
            name: req.name,
            email_address: req.emailAddress,
            title: req.title,
            bio: req.bio,
            location: req.location,
            time_zone_name: req.timeZoneName,
            first_week_day: req.firstWeekDay,
            time_format: req.timeFormat,
          },
        })
    );
  }
}