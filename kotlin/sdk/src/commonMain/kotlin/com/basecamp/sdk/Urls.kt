package com.basecamp.sdk

import io.ktor.http.Url
import io.ktor.http.parseUrl

/**
 * Shared URL helpers used by pagination guards, OAuth discovery, and the token
 * POST. Centralizing them keeps every SSRF/same-origin decision on the SAME
 * parser the transport dials with (Ktor's [parseUrl]) rather than a hand-rolled
 * regex that could disagree about the host actually contacted.
 */

/**
 * Parses an absolute URL with Ktor's own parser — the SAME parser the
 * transport uses to dial — so a guard can never disagree with the client
 * about which host a URL targets. A hand-rolled parser here previously let
 * `http://evil.example\.localhost/x` pass the localhost carve-out while Ktor
 * treats `\` as a path separator and dials `evil.example`.
 *
 * Returns null (fail closed) when the input is malformed or not absolute:
 * Ktor parses a scheme-less string as a relative reference against
 * `http://localhost`, which must never be blessed as localhost/same-origin.
 */
internal fun parseAbsoluteUrl(url: String): Url? {
    val parsed = parseUrl(url) ?: return null
    if (!url.startsWith("${parsed.protocol.name}://", ignoreCase = true)) return null
    return parsed
}

/**
 * True when a bare host (no scheme/port) denotes loopback: `localhost`,
 * `127.0.0.1`, `::1`, or the RFC 6761 `.localhost` TLD. Case-insensitive;
 * tolerates bracketed IPv6 literals (`[::1]`) that some parsers retain.
 */
internal fun isLocalhostHost(host: String): Boolean {
    val h = host.lowercase().removePrefix("[").removeSuffix("]")
    return h == "localhost" ||
        h == "127.0.0.1" ||
        h == "::1" ||
        h.endsWith(".localhost")
}

/** Returns true if the URL points to localhost over HTTP(S) (for dev/test). */
internal fun isLocalhost(url: String): Boolean {
    val parsed = parseAbsoluteUrl(url) ?: return false
    // The carve-out is limited to HTTP(S) so the credential backstop fails
    // closed on any other scheme (e.g. ws://localhost).
    when (parsed.protocol.name) {
        "http", "https" -> {}
        else -> return false
    }
    return isLocalhostHost(parsed.host)
}

/**
 * Validates that an endpoint URL is secure: HTTPS everywhere, with plain HTTP
 * permitted only for localhost (RFC 6761) during local development. Any other
 * scheme is rejected. Used to guard caller-supplied token/authorization
 * endpoints before credentials are attached.
 *
 * @throws BasecampException.Validation if the URL is malformed or insecure.
 */
internal fun requireSecureEndpoint(url: String, label: String) {
    val parsed = parseAbsoluteUrl(url)
        ?: throw BasecampException.Validation("Invalid $label: ${BasecampException.truncateMessage(url)}")
    val isLocalhostHttp = parsed.protocol.name == "http" && isLocalhostHost(parsed.host)
    if (parsed.protocol.name != "https" && !isLocalhostHttp) {
        throw BasecampException.Validation("$label must use HTTPS: ${BasecampException.truncateMessage(url)}")
    }
}

/**
 * Parses a caller- or metadata-supplied origin and enforces the origin-root
 * profile (SPEC.md §16): https (or http on localhost), host present, a valid or
 * absent port, path empty or exactly "/", and no query/fragment/userinfo. Uses
 * [parseAbsoluteUrl] (never a regex) so bracketed IPv6 (`http://[::1]:3000`) and
 * ports agree with the host the client actually dials.
 *
 * Throws [BasecampException.Usage] on violation — a bad *caller* origin is a
 * usage error. Callers validating an *advertised* origin catch and reclassify.
 *
 * @return the normalized origin (scheme://host[:port], no trailing slash).
 */
internal fun requireOriginRoot(raw: String, label: String = "origin"): String {
    val url = parseAbsoluteUrl(raw)
        ?: throw BasecampException.Usage("Invalid $label: not a valid absolute URL: ${BasecampException.truncateMessage(raw)}")

    val scheme = url.protocol.name
    val isLocalhostHttp = scheme == "http" && isLocalhostHost(url.host)
    if (scheme != "https" && !isLocalhostHttp) {
        throw BasecampException.Usage("$label must use HTTPS (or http on localhost): ${BasecampException.truncateMessage(raw)}")
    }
    if (url.host.isEmpty()) {
        throw BasecampException.Usage("$label has no host: ${BasecampException.truncateMessage(raw)}")
    }
    // Reject ANY userinfo, including an empty one (e.g. `https://@host`): Ktor's
    // Url.user/Url.password are null for empty userinfo, so inspect the raw
    // authority substring for an '@' rather than trusting the parsed fields.
    val authority = raw.substringAfter("://", "")
        .substringBefore('/')
        .substringBefore('?')
        .substringBefore('#')
    if (authority.contains('@') || !url.user.isNullOrEmpty() || !url.password.isNullOrEmpty()) {
        throw BasecampException.Usage("$label must not contain userinfo: ${BasecampException.truncateMessage(raw)}")
    }
    // trailingQuery catches a bare '?' with an empty query (e.g. `https://host?`),
    // whose encodedQuery is empty but which is still a query-bearing origin. Ktor
    // has no trailingQuery equivalent for a bare '#' (encodedFragment is empty for
    // both absent and empty), so scan the raw input too: a '#' only ever delimits
    // a fragment here.
    if (url.encodedQuery.isNotEmpty() || url.trailingQuery || url.encodedFragment.isNotEmpty() || raw.contains('#')) {
        throw BasecampException.Usage("$label must not contain a query or fragment: ${BasecampException.truncateMessage(raw)}")
    }
    val path = url.encodedPath
    if (path.isNotEmpty() && path != "/") {
        throw BasecampException.Usage("$label must be an origin root (no path): ${BasecampException.truncateMessage(raw)}")
    }

    // parseAbsoluteUrl fails closed on a malformed port, so a surviving url has a
    // structurally valid (possibly default) port. Rebuild the origin explicitly,
    // re-bracketing IPv6 literals and dropping a default port.
    // Lowercase the host: DNS names and schemes are case-insensitive (RFC 3986
    // §3.1/§6.2.2.1), so a mixed-case advertised issuer such as
    // https://Launchpad.37signals.com normalizes to the same origin as its
    // canonical form — otherwise the Launchpad exclusion misses it.
    val host = url.host.lowercase()
    val hostForOrigin = if (host.contains(':') && !host.startsWith("[")) "[$host]" else host
    val portPart = if (url.port != url.protocol.defaultPort) ":${url.port}" else ""
    return "$scheme://$hostForOrigin$portPart"
}
