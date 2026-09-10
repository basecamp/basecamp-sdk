package com.basecamp.sdk

import io.ktor.client.network.sockets.ConnectTimeoutException
import io.ktor.client.network.sockets.SocketTimeoutException
import io.ktor.client.plugins.HttpRequestTimeoutException
import io.ktor.http.Url

/**
 * SPEC §9 projection of a transport failure before it becomes a
 * [BasecampException] or reaches a hook.
 *
 * Ktor's timeout exceptions render the request URL in their message —
 * `Request timeout has expired [url=…, request_timeout=… ms]` — so a timeout
 * on a request whose query is signed put the signature into the SDK error's
 * message, `toString()`, stack trace and cause chain, and into
 * `RequestResult.error`. Each is rebuilt from parts this SDK chooses: the same
 * class, so a caller (and the device flow's own timeout classification) still
 * recognises it, and the URL the SDK issued — a redirect's target is the
 * transport's business and is not rendered — projected to origin and path
 * when it sits on [trustedOrigin] without userinfo, and to the origin alone
 * anywhere else, since a storage service can sign the path as readily as the
 * query. Every other exception is the transport's own diagnostic (a JVM
 * socket failure names host and port at most) and passes through unchanged,
 * cause chain included.
 */
internal fun redactTransportError(
    e: Throwable,
    url: String,
    trustedOrigin: String? = null,
    timeoutMillis: Long? = null,
): Throwable {
    val shown = if (trustedOrigin != null && isSameOrigin(url, trustedOrigin) && !hasUserinfo(url)) {
        displayUrl(url)
    } else {
        displayOrigin(url)
    }
    return when (e) {
        is HttpRequestTimeoutException -> HttpRequestTimeoutException(shown, timeoutMillis)
        is ConnectTimeoutException ->
            ConnectTimeoutException("Connect timeout has expired [url=$shown, connect_timeout=${timeoutMillis ?: "unknown"} ms]")
        is SocketTimeoutException ->
            SocketTimeoutException("Socket timeout has expired [url=$shown, socket_timeout=${timeoutMillis ?: "unknown"} ms]")
        else -> e
    }
}

/**
 * Renders a URL for hooks and errors: origin and path only — no userinfo (a
 * configured base URL can carry one), no query (where a signed credential
 * rides), no fragment (SPEC §9). Rebuilt from a parse; a URL with no complete
 * origin renders as the fixed token, never as any of its own text.
 */
internal fun displayUrl(url: String): String {
    val parsed = parseForDisplay(url) ?: return "unparsable"
    return "${displayOrigin(parsed)}${parsed.encodedPath}"
}

/** Renders a URL as its origin alone: scheme, host and any non-default port (SPEC §9). */
internal fun displayOrigin(url: String): String {
    val parsed = parseForDisplay(url) ?: return "unparsable"
    return displayOrigin(parsed)
}

private fun displayOrigin(parsed: Url): String {
    val host = if (parsed.host.contains(':') && !parsed.host.startsWith("[")) "[${parsed.host}]" else parsed.host
    val port = if (parsed.specifiedPort != 0 && parsed.specifiedPort != parsed.protocol.defaultPort) {
        ":${parsed.specifiedPort}"
    } else {
        ""
    }
    return "${parsed.protocol.name}://$host$port"
}

private fun parseForDisplay(url: String): Url? {
    val parsed = try {
        Url(url)
    } catch (_: Exception) {
        return null
    }
    return parsed.takeIf { it.host.isNotEmpty() }
}

private fun hasUserinfo(url: String): Boolean = parseForDisplay(url)?.let { it.user != null || it.password != null } ?: true
