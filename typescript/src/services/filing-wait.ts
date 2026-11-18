/**
 * Move or copy a recording and wait for Basecamp to finish it (SPEC §18,
 * Appendix F "Filing Waits"), ported from `go/pkg/basecamp/filing_wait.go`.
 *
 * The start is sent once and never resent, because each copy request makes
 * another copy. GetFiling is then polled with backoff until the filing
 * completes or fails, or the wait ends. Every request is a generated operation
 * under its own hook identity.
 */

import { BasecampError } from "../errors.js";
import type { components } from "../generated/schema.js";

export type Filing = components["schemas"]["Filing"];

/**
 * The kind of container a move or copy files a recording into. Every kind but
 * `todolist` is a tool on a project's dock, and its value is the tool's dock
 * name: the destination project's container is its enabled dock item of that
 * name. A to-do list is not on the dock; list the destination project's to-do
 * lists under its todoset.
 */
export type FilingContainer =
  | "kanban_board"
  | "message_board"
  | "vault"
  | "schedule"
  | "questionnaire"
  | "inbox"
  | "todoset"
  | "todolist";

const FILING_CONTAINERS: ReadonlyMap<string, FilingContainer> = new Map([
  ["Kanban::Card", "kanban_board"],
  ["Kanban::Column", "kanban_board"],
  ["Message", "message_board"],
  ["Document", "vault"],
  ["Upload", "vault"],
  ["CloudFile", "vault"],
  ["GoogleDocument", "vault"],
  ["Vault", "vault"],
  ["Schedule::Entry", "schedule"],
  ["Question", "questionnaire"],
  ["Inbox::Forward", "inbox"],
  ["Todolist", "todoset"],
  ["Todo", "todolist"],
]);

/**
 * The kind of container a move or copy files a recording into, from its
 * recordable type and its parent's type as the recording's own read reports
 * them (`parent.type`, `""` when it has no parent). A card lands in Triage, or
 * in Done if it is completed. Undefined for everything else: a recording that
 * moves only with its parent, such as a comment or the columns every card
 * table has, and a dock tool, such as a docked Vault, which is filed by
 * `destinationProjectId`.
 */
export function filingContainerFor(
  recordingType: string,
  parentType: string,
): FilingContainer | undefined {
  if (recordingType === "Todolist" && parentType === "Todolist") return "todolist";
  if (recordingType === "Vault" && parentType !== "Vault") return undefined;
  return FILING_CONTAINERS.get(recordingType);
}

/** Whether a destination of this kind can take `parentId`, a folder inside it. */
export function filingContainerTakesFolders(container: FilingContainer): boolean {
  return container === "vault";
}

/** Bounds how long `moveAndWait` and `copyAndWait` wait. */
export interface FilingWaitOptions {
  /** Ends the wait this long after the filing starts. Defaults to a minute. */
  timeoutMs?: number;
}

export type FilingErrorKind =
  | "filing_refused"
  | "filing_may_have_started"
  | "filing_failed"
  | "filing_unsettled";

/**
 * A move or copy that did not complete. `kind` says how far it got; match on
 * it or on the subclass. Only `FilingRefusedError` is safe to try again. Like
 * Go's, these are not `BasecampError`s: the failure that caused one is its
 * `cause`, so a retryable error inside one that may have filed cannot read as
 * permission to send the start again.
 */
export class FilingError extends Error {
  readonly kind: FilingErrorKind;

  constructor(kind: FilingErrorKind, message: string, cause?: unknown) {
    super(message, cause === undefined ? undefined : { cause });
    this.name = "FilingError";
    this.kind = kind;
  }
}

/** Basecamp refused to start the move or copy (a 4xx): nothing was filed. */
export class FilingRefusedError extends FilingError {
  declare readonly cause: BasecampError;

  constructor(cause: BasecampError) {
    super("filing_refused", `filing refused: ${cause.message}`, cause);
    this.name = "FilingRefusedError";
  }
}

/**
 * The start failed without Basecamp refusing it, such as a 5xx or a dropped
 * connection. It may have been filed, so look before trying again.
 */
export class FilingMayHaveStartedError extends FilingError {
  constructor(cause: unknown) {
    super("filing_may_have_started", `filing may have started: ${String(cause)}`, cause);
    this.name = "FilingMayHaveStartedError";
  }
}

/** Basecamp started the move or copy and reported it failed. */
export class FilingFailedError extends FilingError {
  readonly filing: Filing;

  constructor(filing: Filing) {
    super("filing_failed", `filing ${filing.id} failed`);
    this.name = "FilingFailedError";
    this.filing = filing;
  }
}

/**
 * Basecamp started the move or copy and had not finished when the wait ended:
 * the timeout, or a GetFiling that failed (its `cause`). `filing` is its last
 * known state; poll `getFiling(filing.id)` rather than starting it again.
 */
export class FilingUnsettledError extends FilingError {
  readonly filing: Filing;

  constructor(filing: Filing, cause?: unknown) {
    super("filing_unsettled", `filing ${filing.id} is still ${filing.status}`, cause);
    this.name = "FilingUnsettledError";
    this.filing = filing;
  }
}

const DEFAULT_WAIT_MS = 60_000;
const FIRST_POLL_MS = 250;
const MAX_POLL_MS = 2_000;

function refusedBeforeFiling(err: unknown): err is BasecampError {
  if (!(err instanceof BasecampError)) return false;
  const status = err.httpStatus ?? 0;
  return (status >= 400 && status < 500) || err.code === "usage" || err.code === "rate_limit";
}

// The generated methods return the body undecoded, so a malformed one would
// otherwise replace the filing the caller needs to poll again.
function isFiling(value: unknown, filingId?: number): value is Filing {
  if (typeof value !== "object" || value === null) return false;
  const { id, status } = value as { id?: unknown; status?: unknown };
  return (
    typeof id === "number" &&
    Number.isInteger(id) &&
    typeof status === "string" &&
    (filingId === undefined || id === filingId)
  );
}

/** Starts a filing once and waits for it; the body of moveAndWait and copyAndWait. */
export async function fileAndWait(
  start: () => Promise<Filing>,
  getFiling: (id: number) => Promise<Filing>,
  opts?: FilingWaitOptions,
): Promise<Filing> {
  let filing: Filing;
  try {
    filing = await start();
  } catch (err) {
    throw refusedBeforeFiling(err) ? new FilingRefusedError(err) : new FilingMayHaveStartedError(err);
  }
  if (!isFiling(filing)) {
    throw new FilingMayHaveStartedError(new Error(`unreadable answer ${JSON.stringify(filing)}`));
  }

  const deadline = Date.now() + (opts?.timeoutMs && opts.timeoutMs > 0 ? opts.timeoutMs : DEFAULT_WAIT_MS);
  for (let delay = FIRST_POLL_MS; ; delay = Math.min(2 * delay, MAX_POLL_MS)) {
    if (filing.status === "completed" && filing.destination_recording) return filing;
    if (filing.status === "failed") throw new FilingFailedError(filing);

    const remaining = deadline - Date.now();
    if (remaining <= delay) {
      await new Promise((resolve) => setTimeout(resolve, Math.max(remaining, 0)));
      throw new FilingUnsettledError(filing, new Error("the wait for the filing ended"));
    }
    await new Promise((resolve) => setTimeout(resolve, delay));
    let latest: unknown;
    try {
      latest = await getFiling(filing.id);
    } catch (err) {
      throw new FilingUnsettledError(filing, err);
    }
    if (!isFiling(latest, filing.id)) {
      throw new FilingUnsettledError(filing, new Error(`unreadable answer ${JSON.stringify(latest)}`));
    }
    filing = latest;
  }
}
