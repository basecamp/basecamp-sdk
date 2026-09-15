/**
 * `CommentsService.expandMentions` / `createWithMentions` — posting a comment
 * that mentions people, by id.
 *
 * A mention is a `<bc-attachment>` carrying the person's `attachable_sgid`
 * (see `./mentions.ts`), and only a people read can supply one that BC3 will
 * honour. These two compose that read with the generated comment write; they
 * make no wire request of their own and mint no operation identity, so hooks
 * see `GetPerson` and `CreateComment` under their own names (SPEC §18 rule 3).
 * They live in `src/services/*-extensions.ts` and are wired in `client.ts`, the
 * placement §18 rule 5 designates for TypeScript.
 */

import type { BasecampHooks } from "../hooks.js";
import { BasecampError, Errors } from "../errors.js";
import { CommentsService as GeneratedCommentsService } from "../generated/services/comments.js";
import type { Comment } from "../generated/services/comments.js";
import type { Person } from "../generated/services/people.js";
import type { PeopleService } from "../generated/services/people.js";
import type { RawClient } from "./base.js";
import { withMentions } from "./mentions.js";

/**
 * The generated read the mention composites need, as the client exposes it.
 *
 * Narrow on purpose: the one authoritative source of an `attachable_sgid` is
 * `people.get`, and nothing else here reaches the wire.
 */
export interface MentionPeopleSource {
  readonly people: Pick<PeopleService, "get">;
}

/**
 * Re-raises a person read's failure with the mention it was resolving.
 *
 * The classification is preserved rather than flattened into a generic error:
 * a caller matching `code === "not_found"` or `"forbidden"` on the expansion
 * still gets the answer the read gave, and `cause` keeps the original for a
 * stack.
 */
function mentionLookupFailed(personId: number, err: unknown): unknown {
  if (!(err instanceof BasecampError)) return err;
  return new BasecampError(err.code, `resolving mention for person ${personId}: ${err.message}`, {
    hint: err.hint,
    httpStatus: err.httpStatus,
    retryable: err.retryable,
    retryAfter: err.retryAfter,
    requestId: err.requestId,
    fieldErrors: err.fieldErrors,
    cause: err,
  });
}

/**
 * `CommentsService` with the hand-written mention composites on top of the
 * generated surface (`get`, `list`, `create`, `update`, ...).
 */
export class CommentsService extends GeneratedCommentsService {
  readonly #sources?: () => MentionPeopleSource;

  // Preserve BaseService's full positional signature, then append the people
  // source — the shape uploads-extensions established, so an existing
  // `new CommentsService(client, hooks, fetchPage, maxPages)` call keeps
  // working and the mention methods explain themselves when it was not wired.
  constructor(
    client: RawClient,
    hooks?: BasecampHooks,
    fetchPage?: (url: string) => Promise<Response>,
    maxPages?: number,
    authenticatedFetch?: (url: string, init: RequestInit) => Promise<Response>,
    baseUrl?: string,
    sources?: () => MentionPeopleSource,
  ) {
    super(client, hooks, fetchPage, maxPages, authenticatedFetch, baseUrl);
    this.#sources = sources;
  }

  /**
   * Content that mentions each of the given people, for posting as a comment —
   * or, since the markup is the same, as a rich-text Campfire line.
   *
   * Every requested id is read through `people.get` for its `attachable_sgid` —
   * one read per distinct id, always: an sgid already in the content is
   * unsigned and cannot prove the person is mentioned, so it never stands in
   * for the read — and the mentions are placed as `withMentions` places them,
   * which adds nothing for a person whose exact `attachable_sgid` the content
   * already carries. A person read that fails — an id that is not a person in
   * this account, a 403 — fails the expansion; nothing is posted on a partial
   * mention list.
   *
   * The rendered mentions round-trip: `mentionedPersonIds` on the returned
   * content reports every id passed here, and `recordings.summarize` reports
   * them on the comment once posted.
   *
   * @param content - The comment body, in HTML.
   * @param personIds - The people to mention. Order is preserved; repeats cost
   *   one read and produce one mention.
   * @returns The content with the mentions placed.
   * @throws {BasecampError} `usage` for a non-positive id, or the person read's
   *   own error, re-raised with the mention it was resolving.
   *
   * @example
   * ```ts
   * const body = await client.comments.expandMentions("<div>On it.</div>", [1049715915]);
   * ```
   */
  async expandMentions(content: string, personIds: readonly number[]): Promise<string> {
    if (personIds.length === 0) return content;

    const people: Person[] = [];
    const seen = new Set<number>();
    for (const id of personIds) {
      if (!Number.isInteger(id) || id <= 0) {
        throw Errors.usage(`invalid mention person id ${id}`);
      }
      if (seen.has(id)) continue;
      seen.add(id);
      try {
        people.push(await this.#reads.people.get(id));
      } catch (err) {
        throw mentionLookupFailed(id, err);
      }
    }
    return withMentions(content, people);
  }

  /**
   * Creates a comment on a recording whose content mentions the given people:
   * {@link expandMentions}, then `create`. The mention reads happen before the
   * write, so a failed lookup posts nothing.
   *
   * @param recordingId - The recording to comment on.
   * @param content - The comment body, in HTML.
   * @param personIds - The people to mention.
   * @returns The created Comment.
   * @throws {BasecampError} `usage` when the content is empty, or the person
   *   read's own error.
   *
   * @example
   * ```ts
   * await client.comments.createWithMentions(1069479351, "<div>On it.</div>", [1049715915]);
   * ```
   */
  async createWithMentions(
    recordingId: number,
    content: string,
    personIds: readonly number[] = [],
  ): Promise<Comment> {
    if (!content) throw Errors.usage("comment content is required");
    const expanded = await this.expandMentions(content, personIds);
    return this.create(recordingId, { content: expanded });
  }

  /** The people source, or a usage error naming how to obtain a wired service. */
  get #reads(): MentionPeopleSource {
    const sources = this.#sources?.();
    if (sources === undefined) {
      throw Errors.usage(
        "comments.expandMentions composes the people read — obtain CommentsService via createBasecampClient(...).comments rather than instantiating it directly",
      );
    }
    return sources;
  }
}
