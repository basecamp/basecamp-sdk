/**
 * MyNotes service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** MyNote entity from the Basecamp API. */
export type MyNote = components["schemas"]["MyNote"];

/**
 * Request parameters for updateMyNote.
 */
export interface UpdateMyNoteMyNoteRequest {
  /** Note */
  note: components["schemas"]["MyNoteAttributes"];
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for MyNotes operations.
 */
export class MyNotesService extends BaseService {

  /**
   * Get the authenticated user's note — a per-person notebook singleton at
   * @returns The MyNote
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.myNotes.getMyNote();
   * ```
   */
  async getMyNote(): Promise<MyNote> {
    const response = await this.request(
      {
        service: "MyNotes",
        operation: "GetMyNote",
        resourceType: "my_note",
        isMutation: false,
      },
      () =>
        this.client.GET("/my/notes.json", {
        })
    );
    return response;
  }

  /**
   * Replace the note's content, recording a new revision server-side.
   * @param req - My_note update parameters
   * @returns The MyNote
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.myNotes.updateMyNote({ note: { content: "Hello world" } });
   * ```
   */
  async updateMyNote(req: UpdateMyNoteMyNoteRequest): Promise<MyNote> {
    if (!req.note) {
      throw Errors.validation("Note is required");
    }
    const response = await this.request(
      {
        service: "MyNotes",
        operation: "UpdateMyNote",
        resourceType: "my_note",
        isMutation: true,
      },
      () =>
        this.client.PUT("/my/notes.json", {
          body: {
            note: req.note,
          },
        })
    );
    return response;
  }
}