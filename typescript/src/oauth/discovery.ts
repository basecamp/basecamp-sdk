/**
 * OAuth 2.0 discovery for Basecamp SDK.
 *
 * Two composable operations plus an orchestrator (SPEC.md §16, communique §2/§3):
 *   - discover(issuerURL)                    — RFC 8414 AS metadata + issuer binding
 *   - discoverProtectedResource(origin)      — RFC 9728 resource metadata
 *   - discoverFromResource(origin, opts)     — resource-first selection + fallback
 *
 * All fetches are SSRF-hardened: HTTPS-only origins (localhost exempt), origin
 * parsed/validated with the platform URL parser before any socket opens,
 * redirects suppressed, timeouts bounded, and bodies read under a genuine
 * bounded cap that aborts before the whole oversized body is buffered.
 */

import { BasecampError } from "../errors.js";
import { isLocalhost } from "../security.js";
import type {
  OAuthConfig,
  ProtectedResourceMetadata,
  FallbackReason,
  DiscoverySelectionErrorReason,
  DiscoverFromResourceResult,
} from "./types.js";

/** Raw AS metadata response from an OAuth server (RFC 8414). */
interface RawDiscoveryResponse {
  issuer?: string;
  authorization_endpoint?: string;
  token_endpoint?: string;
  device_authorization_endpoint?: string;
  registration_endpoint?: string;
  scopes_supported?: string[];
  grant_types_supported?: string[];
  code_challenge_methods_supported?: string[];
}

/** Raw resource metadata response (RFC 9728). */
interface RawResourceResponse {
  resource?: string;
  authorization_servers?: string[];
}

/** Default cap on a discovery response body (1 MiB) — discovery docs are tiny. */
const MAX_DISCOVERY_BODY_BYTES = 1 * 1024 * 1024;

/** True iff `value` is an array whose every element is a string. */
function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((v) => typeof v === "string");
}

/**
 * Resolves the caller-supplied body cap to a usable byte count. A finite,
 * non-negative number is honored; anything else (Infinity, NaN, negative, a
 * non-number) falls back to the default so the bounded read can never be
 * silently disabled.
 */
function resolveMaxBodyBytes(value: number | undefined): number {
  return typeof value === "number" && Number.isFinite(value) && value >= 0
    ? value
    : MAX_DISCOVERY_BODY_BYTES;
}

/**
 * Options for OAuth discovery.
 */
export interface DiscoverOptions {
  /** Custom fetch function for testing or custom HTTP handling */
  fetch?: typeof globalThis.fetch;
  /** Request timeout in milliseconds (default: 10000) */
  timeoutMs?: number;
  /**
   * Maximum discovery response body size in bytes (default: 1 MiB). Must be a
   * finite, non-negative number; Infinity/NaN/negative values are ignored and
   * the default cap applies (the bound cannot be disabled).
   */
  maxBodyBytes?: number;
}

/**
 * Options for {@link discoverFromResource}.
 */
export interface DiscoverFromResourceOptions extends DiscoverOptions {
  /**
   * Explicit, authoritative issuer selection. When provided, the advertised
   * member equal by code-point is selected; if none matches, discovery raises
   * `expected_issuer_unavailable` (never falls back). Omit to use the
   * Basecamp-profile exclusion heuristic.
   */
  expectedIssuer?: string;
}

/**
 * Hard resource-first selection/validation failure. Thrown — never returned as a
 * fallback — so no consumer can convert it into a Launchpad request.
 */
export class DiscoverySelectionError extends BasecampError {
  readonly reason: DiscoverySelectionErrorReason;

  constructor(
    reason: DiscoverySelectionErrorReason,
    message: string,
    options?: { cause?: Error; httpStatus?: number }
  ) {
    // capability/expected-issuer are consumer/usage-shaped; the rest are AS
    // metadata faults surfaced as api_error.
    const code =
      reason === "capability_unavailable"
        ? "validation"
        : "api_error";
    super(code, message, options);
    this.name = "DiscoverySelectionError";
    this.reason = reason;
  }
}

/**
 * Default Basecamp/Launchpad OAuth server URL.
 */
export const LAUNCHPAD_BASE_URL = "https://launchpad.37signals.com";

/**
 * Parses a caller- or metadata-supplied origin and enforces the origin-root
 * profile: https (or http+localhost), host present, optional valid port, path
 * empty or exactly "/", and no query/fragment/userinfo. Uses the platform URL
 * parser (never a regex) so bracketed IPv6 and ports agree with the host the
 * client actually dials.
 *
 * Throws `BasecampError("usage")` on violation — a bad *caller* origin is a
 * usage error. Callers validating an *advertised* origin catch and reclassify.
 *
 * @returns the normalized origin (scheme://host[:port], no trailing slash)
 */
export function requireOriginRoot(raw: string, label = "origin"): string {
  // Reject C0 controls, space, and backslash up front: WHATWG URL silently strips
  // tabs/newlines/surrounding spaces and converts backslashes to forward slashes
  // for special schemes, so a malformed spelling ("https:\\host", "https://host\n")
  // would be cleaned and accepted. None is legitimate in an origin root.
  if (/[\u0000-\u0020\\]/.test(raw)) {
    throw new BasecampError("usage", `${label} contains invalid characters: ${raw}`);
  }
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new BasecampError("usage", `Invalid ${label}: not a valid absolute URL: ${raw}`);
  }

  const isLocalhostHttp = url.protocol === "http:" && isLocalhost(url.hostname);
  if (url.protocol !== "https:" && !isLocalhostHttp) {
    throw new BasecampError("usage", `${label} must use HTTPS (or http on localhost): ${raw}`);
  }
  if (!url.hostname) {
    throw new BasecampError("usage", `${label} has no host: ${raw}`);
  }
  // The WHATWG URL parser normalizes delimiter-only userinfo ("https://@host",
  // "https://:@host") to EMPTY username/password and drops it from href, so the
  // parsed fields alone cannot catch it — also inspect the raw authority (the
  // scheme check above guarantees raw starts with http(s)://).
  const rawAuthority = raw.slice(raw.indexOf("//") + 2).split(/[/?#]/, 1)[0] ?? "";
  if (url.username || url.password || rawAuthority.includes("@")) {
    throw new BasecampError("usage", `${label} must not contain userinfo: ${raw}`);
  }
  // WHATWG `URL` exposes a bare trailing "?" or "#" (e.g. "https://host?") as an
  // EMPTY search/hash and normalizes it away from `origin`, so the parsed fields
  // alone miss the delimiter. Also scan the raw input: any "?"/"#" past the
  // scheme is a query/fragment delimiter here (host/port carry neither, and the
  // path is constrained to ""/"/" below).
  if (url.search || url.hash || raw.includes("?") || raw.includes("#")) {
    throw new BasecampError("usage", `${label} must not contain a query or fragment: ${raw}`);
  }
  if (url.pathname !== "" && url.pathname !== "/") {
    throw new BasecampError("usage", `${label} must be an origin root (no path): ${raw}`);
  }
  // `new URL("https://h:notaport")` throws above and WHATWG rejects ports > 65535,
  // but it ACCEPTS port 0 and keeps it in the origin. The origin-root profile (like
  // the other SDKs) rejects any port outside 1–65535, so a caller/advertised issuer
  // using `:0` fails as usage / invalid_issuer_origin rather than proceeding to a fetch.
  if (url.port !== "" && (Number(url.port) < 1 || Number(url.port) > 65535)) {
    throw new BasecampError("usage", `${label} has an invalid port: ${raw}`);
  }
  // WHATWG normalizes a dangling port ("https://host:") to url.port === "", so the
  // check above misses it; scan the raw authority for a trailing ":" (an IPv6
  // authority ends with "]", so only a trailing ":" is a dangling port).
  if (rawAuthority.endsWith(":")) {
    throw new BasecampError("usage", `${label} has an invalid port: ${raw}`);
  }
  // Note: a surviving url has a structurally valid (possibly default) port. url.origin
  // drops a default port and any trailing slash — exactly the normalized origin we want.
  return url.origin;
}

/**
 * True when an issuer string is a valid origin root equal to Launchpad's.
 *
 * The comparison runs both sides through {@link requireOriginRoot}, so an
 * advertised look-alike that is *not* a clean origin root — e.g.
 * `https://launchpad.37signals.com/path` (path), userinfo, or a query — is not
 * treated as Launchpad. It stays a non-Launchpad candidate and later fails hard
 * (`ambiguous_issuers` / `invalid_issuer_origin`) rather than being silently
 * excluded from selection. A trailing-slash-only origin root still matches
 * because `requireOriginRoot` normalizes it away.
 *
 * Exported so the login orchestrator derives the legacy-token decision from the
 * same predicate the selection heuristic uses.
 */
export function isLaunchpadIssuer(issuer: string): boolean {
  try {
    return requireOriginRoot(issuer, "issuer") === requireOriginRoot(LAUNCHPAD_BASE_URL, "issuer");
  } catch {
    return false;
  }
}

/**
 * Reads a Response body under a bounded, streaming cap. Aborts (cancels the
 * stream) the moment the accumulated size exceeds `maxBytes`, so an oversized
 * body is never fully buffered — real memory bounding, not a post-hoc check.
 *
 * Exported for reuse by the device-flow transport (same bounding, different
 * `label` in the error message); not re-exported from the package index.
 */
export async function readBodyBounded(
  response: Response,
  maxBytes: number,
  label = "OAuth discovery"
): Promise<string> {
  const body = response.body;
  if (!body) {
    // No readable stream available (some mock transports): guard the buffered
    // length instead. Real runtimes always expose response.body.
    const text = await response.text();
    if (new TextEncoder().encode(text).length > maxBytes) {
      throw new BasecampError("api_error", `${label} response exceeds size cap`);
    }
    return text;
  }

  const reader = body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  let exceeded = false;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    if (value) {
      total += value.byteLength;
      if (total > maxBytes) {
        exceeded = true;
        break;
      }
      chunks.push(value);
    }
  }
  // Release the stream without blocking on cancellation (some transports hang on
  // an awaited cancel()); the point is that we stopped reading past the cap.
  void reader.cancel().catch(() => {});
  if (exceeded) {
    throw new BasecampError("api_error", `${label} response exceeds size cap`);
  }

  const merged = new Uint8Array(total);
  let offset = 0;
  for (const c of chunks) {
    merged.set(c, offset);
    offset += c.byteLength;
  }
  return new TextDecoder().decode(merged);
}

/**
 * SSRF-hardened GET of a discovery document. The origin must already be
 * validated (via {@link requireOriginRoot}); this suppresses redirects, bounds
 * the timeout, reads the body under a bounded cap, and maps non-2xx → api_error.
 */
async function fetchDiscoveryDocument(
  url: string,
  options: DiscoverOptions
): Promise<unknown> {
  const { fetch: customFetch = globalThis.fetch, timeoutMs = 10000 } = options;
  const maxBodyBytes = resolveMaxBodyBytes(options.maxBodyBytes);

  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const response = await customFetch(url, {
      method: "GET",
      headers: { Accept: "application/json" },
      signal: controller.signal,
      // Suppress redirects: never chase an attacker-influenced Location. A 3xx
      // surfaces below as a non-2xx api_error rather than a followed request.
      redirect: "manual",
    });

    if (!response.ok || (response.status >= 300 && response.status < 400)) {
      // Drain-and-cap defensively; body is unused on the error path.
      const body = await readBodyBounded(response, maxBodyBytes).catch(() => "");
      throw new BasecampError(
        "api_error",
        `OAuth discovery failed with status ${response.status}: ${body}`,
        { httpStatus: response.status }
      );
    }

    const text = await readBodyBounded(response, maxBodyBytes);
    try {
      return JSON.parse(text);
    } catch (err) {
      throw new BasecampError("api_error", "Failed to parse OAuth discovery response", {
        cause: err instanceof Error ? err : undefined,
      });
    }
  } catch (err) {
    if (err instanceof BasecampError) throw err;
    if (err instanceof Error) {
      if (err.name === "AbortError") {
        throw new BasecampError("network", "OAuth discovery request timed out", {
          cause: err,
          retryable: true,
        });
      }
      throw new BasecampError("network", `OAuth discovery failed: ${err.message}`, {
        cause: err,
        retryable: true,
      });
    }
    throw new BasecampError("network", "OAuth discovery failed with unknown error", {
      retryable: true,
    });
  } finally {
    clearTimeout(timeoutId);
  }
}

/**
 * Discovers OAuth 2.0 Authorization Server Metadata (RFC 8414) from
 * `{issuerURL}/.well-known/oauth-authorization-server`, and binds it: the
 * returned `issuer` must equal the requested issuer by code-point (no
 * normalization beyond origin-root parsing). `token_endpoint` is required;
 * `authorization_endpoint` is optional (device-only servers omit it).
 *
 * @param baseUrl - The OAuth server's issuer origin (e.g. "https://launchpad.37signals.com")
 * @throws BasecampError("usage") on a malformed origin, api_error on invalid metadata
 *
 * @example
 * ```ts
 * const config = await discover("https://launchpad.37signals.com");
 * console.log(config.tokenEndpoint);
 * ```
 */
export async function discover(
  baseUrl: string,
  options: DiscoverOptions = {}
): Promise<OAuthConfig> {
  const issuerOrigin = requireOriginRoot(baseUrl, "OAuth discovery base URL");
  // Bind against the caller's raw baseUrl (RFC 8414 §3.3, SPEC.md §16 "NO
  // normalization"); the normalized origin is only for the fetch URL.
  return discoverAndBind(issuerOrigin, baseUrl, options);
}

/**
 * Fetch AS metadata from `issuerOrigin`'s well-known URL but bind the returned
 * `issuer` against `bindIssuer` by code-point. Routing and binding are distinct:
 * fetch from the normalized origin, bind against the caller's raw identifier
 * (which may spell a trailing slash or explicit default port). Public `discover`
 * passes its raw `baseUrl` as `bindIssuer`; `discoverFromResource` passes the
 * advertised issuer. Not exported — no public binding override.
 */
async function discoverAndBind(
  issuerOrigin: string,
  bindIssuer: string,
  options: DiscoverOptions
): Promise<OAuthConfig> {
  const discoveryUrl = `${issuerOrigin}/.well-known/oauth-authorization-server`;

  const data = (await fetchDiscoveryDocument(discoveryUrl, options)) as RawDiscoveryResponse;
  if (typeof data !== "object" || data === null) {
    throw new BasecampError("api_error", "OAuth discovery response is not a JSON object");
  }

  return parseAndBindAsMetadata(data, bindIssuer);
}

/**
 * Module-private structural marker for an RFC 8414 issuer-binding failure: the
 * AS metadata's `issuer` did not equal the requested issuer by code-point.
 *
 * `discoverFromResource` branches on this via `instanceof` to classify
 * `issuer_mismatch` vs `as_fetch_failed` — a structured tag, never a match on
 * the message text. Kept private to this module and NOT re-exported from the
 * package index: to any external caller of `discover`, this is an ordinary
 * `api_error` `BasecampError`.
 */
class IssuerBindingError extends BasecampError {
  constructor(message: string) {
    super("api_error", message);
    this.name = "IssuerBindingError";
  }
}

/**
 * Validates AS metadata and binds `issuer` to `expectedIssuerOrigin` by
 * code-point. Universal validation only: `issuer`+`token_endpoint` present and
 * non-empty; any present endpoint field non-empty. Per-grant endpoint checks
 * are the consumer's responsibility.
 */
function parseAndBindAsMetadata(
  data: RawDiscoveryResponse,
  expectedIssuerOrigin: string
): OAuthConfig {
  if (typeof data.issuer !== "string" || !data.issuer) {
    throw new BasecampError("api_error", "Invalid OAuth discovery response: missing required fields (issuer)");
  }
  // RFC 8414 §3.3/§4: issuer identical by code-point. No normalization. Thrown
  // as the structural IssuerBindingError so discoverFromResource can classify it
  // without matching the message text.
  if (data.issuer !== expectedIssuerOrigin) {
    throw new IssuerBindingError(
      `OAuth issuer mismatch: metadata issuer "${data.issuer}" does not equal "${expectedIssuerOrigin}"`
    );
  }
  if (typeof data.token_endpoint !== "string" || !data.token_endpoint) {
    throw new BasecampError("api_error", "Invalid OAuth discovery response: missing required fields (token_endpoint)");
  }
  // Every present endpoint field must be a non-empty string (reject "", arrays,
  // numbers, etc. — a non-string endpoint is malformed, not merely empty).
  for (const [key, value] of Object.entries(data)) {
    if (key.endsWith("_endpoint") && value !== undefined && (typeof value !== "string" || value === "")) {
      throw new BasecampError("api_error", `Invalid OAuth discovery response: invalid ${key}`);
    }
  }
  // grant_types_supported, when present, must be an array of strings — never a
  // bare string (substring-matching it would falsely enable a grant).
  if (data.grant_types_supported !== undefined && !isStringArray(data.grant_types_supported)) {
    throw new BasecampError(
      "api_error",
      "Invalid OAuth discovery response: grant_types_supported must be an array of strings"
    );
  }
  // code_challenge_methods_supported, when present, must likewise be an array of
  // strings — a bare string would be substring-matched during PKCE negotiation
  // and could falsely appear to advertise "S256".
  if (
    data.code_challenge_methods_supported !== undefined &&
    !isStringArray(data.code_challenge_methods_supported)
  ) {
    throw new BasecampError(
      "api_error",
      "Invalid OAuth discovery response: code_challenge_methods_supported must be an array of strings"
    );
  }
  // scopes_supported, when present, must be an array of strings — a bare string
  // ("read write") or null would otherwise reach callers typed as an array and
  // yield substring/null behavior instead of an api_error.
  if (data.scopes_supported !== undefined && !isStringArray(data.scopes_supported)) {
    throw new BasecampError(
      "api_error",
      "Invalid OAuth discovery response: scopes_supported must be an array of strings"
    );
  }

  return {
    issuer: data.issuer,
    authorizationEndpoint: data.authorization_endpoint,
    tokenEndpoint: data.token_endpoint,
    deviceAuthorizationEndpoint: data.device_authorization_endpoint,
    registrationEndpoint: data.registration_endpoint,
    scopesSupported: data.scopes_supported,
    grantTypesSupported: data.grant_types_supported,
    codeChallengeMethodsSupported: data.code_challenge_methods_supported,
  };
}

/**
 * Discovers RFC 9728 protected-resource metadata from
 * `{resourceOrigin}/.well-known/oauth-protected-resource`. `resource` is
 * required and must equal the requested origin by code-point.
 * `authorization_servers` is preserved distinctly as absent vs `[]`.
 *
 * @throws BasecampError("usage") on a malformed caller origin, api_error on
 *   invalid metadata
 */
export async function discoverProtectedResource(
  resourceOrigin: string,
  options: DiscoverOptions = {}
): Promise<ProtectedResourceMetadata> {
  const origin = requireOriginRoot(resourceOrigin, "resource origin");
  const url = `${origin}/.well-known/oauth-protected-resource`;

  const data = (await fetchDiscoveryDocument(url, options)) as RawResourceResponse;
  if (typeof data !== "object" || data === null) {
    throw new BasecampError("api_error", "Resource metadata response is not a JSON object");
  }
  if (typeof data.resource !== "string" || !data.resource) {
    throw new BasecampError("api_error", "Invalid resource metadata: missing required field (resource)");
  }
  // Bind the resource identifier to the requested identifier (the raw caller
  // origin), code-point exact, NO normalization (RFC 9728 §3.3, SPEC.md §16): the
  // well-known URL is built from the normalized origin, but doc.resource must be
  // identical to what the caller supplied.
  if (data.resource !== resourceOrigin) {
    throw new BasecampError(
      "api_error",
      `Resource identifier mismatch: metadata resource "${data.resource}" does not equal "${resourceOrigin}"`
    );
  }

  // authorization_servers, when present, MUST be an array of strings. A bare
  // string slipped through and was iterated char-by-char during selection; a
  // present JSON null is likewise malformed (an array is required when the key is
  // present) — not a present-empty list. Reject every present non-array value so
  // the orchestrator classifies it as resource_discovery_failed rather than
  // silently taking no_as_advertised.
  let authorizationServers: string[] | undefined;
  if (Object.prototype.hasOwnProperty.call(data, "authorization_servers")) {
    const raw: unknown = (data as Record<string, unknown>).authorization_servers;
    if (isStringArray(raw)) {
      authorizationServers = raw;
    } else {
      throw new BasecampError(
        "api_error",
        "Invalid resource metadata: authorization_servers must be an array of strings when present"
      );
    }
  }

  return { resource: data.resource, authorizationServers };
}

/**
 * Resource-first discovery orchestrator (SPEC.md §16). Composes RFC 9728 + RFC
 * 8414 and applies the stage-sensitive fallback state machine.
 *
 * Returns `{ kind: "selected", config, issuer }` or `{ kind: "fallback", reason }`
 * where `reason ∈ {resource_discovery_failed, no_as_advertised}`. Every hard
 * failure throws {@link DiscoverySelectionError} — callers MUST NOT convert a
 * throw into a Launchpad request.
 */
export async function discoverFromResource(
  resourceOrigin: string,
  options: DiscoverFromResourceOptions = {}
): Promise<DiscoverFromResourceResult> {
  const { expectedIssuer, ...discoverOptions } = options;

  // --- Hop 1: resource metadata. Failure here is soft (before selection). ---
  // Pass the RAW resourceOrigin so binding is code-point-exact against the caller's
  // identifier (SPEC.md §16); discoverProtectedResource normalizes only its fetch
  // URL. A malformed caller origin surfaces as usage (re-raised below), not a fallback.
  let resource: ProtectedResourceMetadata;
  try {
    resource = await discoverProtectedResource(resourceOrigin, discoverOptions);
  } catch (err) {
    if (err instanceof BasecampError && err.code === "usage") throw err;
    return { kind: "fallback", reason: "resource_discovery_failed" };
  }

  const advertised = resource.authorizationServers ?? [];

  // --- Selection ---
  let selectedIssuer: string;
  if (expectedIssuer !== undefined) {
    const match = advertised.find((s) => s === expectedIssuer);
    if (!match) {
      throw new DiscoverySelectionError(
        "expected_issuer_unavailable",
        `Expected issuer "${expectedIssuer}" is not advertised by the resource`
      );
    }
    selectedIssuer = match;
  } else {
    // Deduplicate exact issuer strings first: a resource that advertises the same
    // non-Launchpad issuer more than once denotes ONE issuer, not an ambiguous set.
    const nonLaunchpad = [...new Set(advertised.filter((s) => !isLaunchpadIssuer(s)))];
    if (nonLaunchpad.length >= 2) {
      throw new DiscoverySelectionError(
        "ambiguous_issuers",
        `Multiple non-Launchpad issuers advertised; pass expectedIssuer to disambiguate: ${nonLaunchpad.join(", ")}`
      );
    }
    if (nonLaunchpad.length === 0) {
      // Valid resource metadata omits BC5 — soft fallback (before selection).
      return { kind: "fallback", reason: "no_as_advertised" };
    }
    selectedIssuer = nonLaunchpad[0]!;
  }

  // --- BC5 is now committed: every subsequent failure is fatal (no Launchpad). ---
  let issuerOrigin: string;
  try {
    issuerOrigin = requireOriginRoot(selectedIssuer, "advertised issuer");
  } catch (err) {
    throw new DiscoverySelectionError(
      "invalid_issuer_origin",
      `Advertised issuer "${selectedIssuer}" is not a valid origin root`,
      { cause: err instanceof Error ? err : undefined }
    );
  }

  let config: OAuthConfig;
  try {
    // Fetch from the normalized origin, but bind against the exact advertised
    // issuer string (selectedIssuer), not the normalized origin.
    config = await discoverAndBind(issuerOrigin, selectedIssuer, discoverOptions);
  } catch (err) {
    // Distinguish issuer-binding mismatch from a generic fetch failure via the
    // structural marker (never the message text).
    if (err instanceof IssuerBindingError) {
      throw new DiscoverySelectionError("issuer_mismatch", err.message, { cause: err });
    }
    throw new DiscoverySelectionError(
      "as_fetch_failed",
      `AS metadata fetch failed for committed issuer "${issuerOrigin}": ${err instanceof Error ? err.message : String(err)}`,
      { cause: err instanceof Error ? err : undefined }
    );
  }

  return { kind: "selected", config, issuer: config.issuer };
}

/**
 * Discovers OAuth configuration from Basecamp's Launchpad server.
 */
export async function discoverLaunchpad(
  options: DiscoverOptions = {}
): Promise<OAuthConfig> {
  return discover(LAUNCHPAD_BASE_URL, options);
}
