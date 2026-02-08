import type { WebhookEvent } from "./events.js";
import { verifyWebhookSignature } from "./verify.js";

/** Handler function for webhook events. */
export type WebhookEventHandler = (event: WebhookEvent) => void | Promise<void>;

/** Middleware function that wraps event processing. Call next() to continue the chain. */
export type WebhookMiddleware = (event: WebhookEvent, next: () => Promise<void>) => Promise<void>;

/** Header accessor: either a record of headers or a function that retrieves a header by name. */
export type HeaderAccessor =
  | Record<string, string | string[] | undefined>
  | ((name: string) => string | undefined);

export interface WebhookReceiverOptions {
  /** HMAC secret for signature verification. If unset, verification is skipped. */
  secret?: string;
  /** HTTP header containing the signature (default: "x-basecamp-signature"). */
  signatureHeader?: string;
  /** Number of recent event IDs to track for deduplication (default: 1000, 0 to disable). */
  dedupWindowSize?: number;
}

/** Error thrown when webhook signature verification fails. */
export class WebhookVerificationError extends Error {
  constructor(message = "invalid webhook signature") {
    super(message);
    this.name = "WebhookVerificationError";
  }
}

/**
 * Receives and routes webhook events from Basecamp.
 *
 * Framework-agnostic: works with raw body bytes and a header accessor.
 * Use adapters (e.g., createNodeHandler) for framework-specific integration.
 */
export class WebhookReceiver {
  private readonly secret?: string;
  private readonly signatureHeader: string;
  private readonly dedupWindowSize: number;
  private readonly handlers = new Map<string, WebhookEventHandler[]>();
  private readonly anyHandlers: WebhookEventHandler[] = [];
  private readonly middlewareChain: WebhookMiddleware[] = [];
  private readonly dedupSet = new Set<number>();
  private readonly dedupOrder: number[] = [];

  constructor(options?: WebhookReceiverOptions) {
    this.secret = options?.secret;
    this.signatureHeader = options?.signatureHeader ?? "x-basecamp-signature";
    this.dedupWindowSize = options?.dedupWindowSize ?? 1000;
  }

  /**
   * Register a handler for a specific event kind pattern.
   * Supports exact match ("todo_created") and glob patterns ("todo.*", "*.created").
   */
  on(pattern: string, handler: WebhookEventHandler): this {
    const existing = this.handlers.get(pattern);
    if (existing) {
      existing.push(handler);
    } else {
      this.handlers.set(pattern, [handler]);
    }
    return this;
  }

  /** Register a handler that fires for all events. */
  onAny(handler: WebhookEventHandler): this {
    this.anyHandlers.push(handler);
    return this;
  }

  /** Add middleware to the processing chain. Middleware runs in registration order before handlers. */
  use(middleware: WebhookMiddleware): this {
    this.middlewareChain.push(middleware);
    return this;
  }

  /**
   * Process a raw webhook request.
   * Returns the parsed WebhookEvent.
   * Duplicate events (by ID) return the event but do not trigger handlers.
   * @throws {WebhookVerificationError} if signature verification fails
   */
  async handleRequest(
    rawBody: string | Buffer,
    headers: HeaderAccessor,
  ): Promise<WebhookEvent> {
    // Verify signature if secret is configured
    if (this.secret) {
      const sig = this.getHeader(headers, this.signatureHeader);
      if (!sig || !verifyWebhookSignature(rawBody, sig, this.secret)) {
        throw new WebhookVerificationError();
      }
    }

    // Parse event
    const bodyStr = typeof rawBody === "string" ? rawBody : rawBody.toString("utf8");
    const event: WebhookEvent = JSON.parse(bodyStr);

    // Dedup check
    if (this.isDuplicate(event.id)) {
      return event;
    }

    // Build middleware chain → handlers
    const runHandlers = async () => {
      await this.dispatchHandlers(event);
    };

    let chain = runHandlers;
    for (let i = this.middlewareChain.length - 1; i >= 0; i--) {
      const mw = this.middlewareChain[i]!;
      const next = chain;
      chain = () => mw(event, next);
    }

    await chain();
    return event;
  }

  private getHeader(headers: HeaderAccessor, name: string): string | undefined {
    if (typeof headers === "function") {
      return headers(name);
    }
    const val = headers[name] ?? headers[name.toLowerCase()];
    if (Array.isArray(val)) return val[0];
    return val;
  }

  private isDuplicate(eventId: number | undefined): boolean {
    if (this.dedupWindowSize <= 0 || !eventId) return false;
    if (this.dedupSet.has(eventId)) return true;

    // Evict oldest if at capacity
    if (this.dedupOrder.length >= this.dedupWindowSize) {
      const oldest = this.dedupOrder.shift()!;
      this.dedupSet.delete(oldest);
    }

    this.dedupSet.add(eventId);
    this.dedupOrder.push(eventId);
    return false;
  }

  private async dispatchHandlers(event: WebhookEvent): Promise<void> {
    const matched: WebhookEventHandler[] = [];

    for (const [pattern, handlers] of this.handlers) {
      if (matchPattern(pattern, event.kind ?? "")) {
        matched.push(...handlers);
      }
    }

    matched.push(...this.anyHandlers);

    for (const handler of matched) {
      await handler(event);
    }
  }
}

/**
 * Match a webhook event kind against a glob pattern.
 * "*" matches any sequence of characters.
 */
function matchPattern(pattern: string, value: string): boolean {
  if (pattern === value) return true;

  const parts = pattern.split("*");
  if (parts.length === 1) return false; // No wildcards, exact match already checked

  let remaining = value;
  for (let i = 0; i < parts.length; i++) {
    const part = parts[i]!;
    if (part === "") continue;

    const idx = remaining.indexOf(part);
    if (idx === -1) return false;
    // First part must be a prefix
    if (i === 0 && idx !== 0) return false;
    remaining = remaining.slice(idx + part.length);
  }

  // Last part must be a suffix (if non-empty)
  const lastPart = parts.at(-1)!;
  if (lastPart !== "") {
    return value.endsWith(lastPart);
  }

  return true;
}
