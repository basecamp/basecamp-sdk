/**
 * `CommentsService.expandMentions` / `createWithMentions` — posting a comment
 * that mentions people, by id.
 *
 * A mention is a `<bc-attachment>` carrying the person's `attachable_sgid`
 * (see `./mentions.ts`), and only a people read can supply one that BC3 will
 * honour. {@link CommentsService.expandMentions} is the read half alone — it
 * resolves people and returns the expanded content, writing nothing, which is
 * what makes it usable for a Campfire line as well;
 * {@link CommentsService.createWithMentions} composes it with the generated
 * comment write. Neither makes a wire request of its own or mints an operation
 * identity, so hooks see `GetPerson` and `CreateComment` under their own names
 * (SPEC §18 rule 3).
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
 * Go wraps with `%w`, which adds the context AND keeps every wrapped type
 * reachable through `errors.As`. JavaScript has no such operator: a new error
 * carries the context but not the original's class, and the original carries
 * its class but not the context. Identity wins, because it is the half a caller
 * can act on — `PeopleConfirmationRequiredError` carries a `people` list that a
 * flattened copy would drop, and a transport rejection is matched on its own
 * shape.
 *
 * So the context is added only in the case where nothing is lost by minting a
 * new error: the plain `BasecampError` the people read actually produces, whose
 * every field is copied across. A subclass or a non-`BasecampError` is re-raised
 * untouched, and `expandMentions` documents that.
 */
function mentionLookupFailed(personId: number, err: unknown): unknown {
  // `constructor`, not `instanceof`: a subclass passes `instanceof` and would
  // be flattened into its base by the copy below.
  if (!(err instanceof BasecampError) || err.constructor !== BasecampError) return err;
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
   *   own error. A plain `BasecampError` from that read is re-raised with the
   *   mention it was resolving named in its message; a subclass or a transport
   *   rejection is re-raised untouched, so its class and payload survive.
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
      // The Go reference validates here too — inside the loop, not ahead of it
      // (go/pkg/basecamp/comments.go, ExpandMentions) — so a bad id after a good
      // one costs the good one's read before it raises. Kept identical
      // deliberately; the invariant that matters is unaffected either way, since
      // every read happens before the write and a refused expansion posts
      // nothing.
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
    // The TYPE, not just the truthiness, and of the value that actually goes
    // out. `!content` is false for `{}` and for `[]`, so a caller in plain
    // JavaScript could pass an object, have it sail past the guard, and reach
    // the mention walk — which does string work and throws a raw TypeError, or,
    // with no mentions requested, posts a body the reference cannot produce:
    // Go's signature takes a string, so neither shape exists there. Checking a
    // coerced copy would be the same defect one step later, the guard answering
    // a question about a value that never leaves the method.
    if (typeof content !== "string") {
      throw Errors.usage("comment content must be a string");
    }
    if (content === "") throw Errors.usage("comment content is required");
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
