package com.basecamp.sdk

import com.basecamp.sdk.generated.models.Comment
import com.basecamp.sdk.generated.projects
import com.basecamp.sdk.generated.reports
import com.basecamp.sdk.generated.services.BookmarksService
import com.basecamp.sdk.generated.services.CommentsService
import com.basecamp.sdk.generated.services.ListCommentsOptions
import com.basecamp.sdk.generated.services.ListProjectsOptions
import com.basecamp.sdk.generated.services.PersonProgressResult
import com.basecamp.sdk.generated.services.SearchService
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import io.ktor.util.date.GMTDate
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class PaginationTest {

    @Test
    fun listResultDelegatesToList() {
        val items = listOf("a", "b", "c")
        val result = ListResult(items, ListMeta(totalCount = 10, truncated = false))

        assertEquals(3, result.size)
        assertEquals("a", result[0])
        assertEquals("c", result[2])
        assertEquals(10, result.meta.totalCount)
        assertFalse(result.meta.truncated)
    }

    @Test
    fun listResultWorksWithCollectionOperations() {
        val items = listOf(1, 2, 3, 4, 5)
        val result = ListResult(items, ListMeta(totalCount = 100, truncated = true))

        // map returns plain List
        val doubled = result.map { it * 2 }
        assertEquals(listOf(2, 4, 6, 8, 10), doubled)

        // filter
        val even = result.filter { it % 2 == 0 }
        assertEquals(listOf(2, 4), even)

        // forEach
        var sum = 0
        result.forEach { sum += it }
        assertEquals(15, sum)

        // spread into another list
        val spread = listOf(0) + result
        assertEquals(listOf(0, 1, 2, 3, 4, 5), spread)
    }

    @Test
    fun listResultEmptyCase() {
        val result = ListResult(emptyList<String>(), ListMeta(totalCount = 0, truncated = false))
        assertEquals(0, result.size)
        assertTrue(result.isEmpty())
    }

    @Test
    fun listResultEqualityIncludesMeta() {
        val a = ListResult(listOf(1, 2), ListMeta(10, false))
        val b = ListResult(listOf(1, 2), ListMeta(10, false))
        val c = ListResult(listOf(1, 2), ListMeta(20, true))

        assertEquals(a, b)
        assertFalse(a == c)
    }

    // =========================================================================
    // parseNextLink
    // =========================================================================

    @Test
    fun parseNextLinkExtractsUrl() {
        val header = """<https://3.basecampapi.com/12345/projects.json?page=2>; rel="next""""
        assertEquals("https://3.basecampapi.com/12345/projects.json?page=2", parseNextLink(header))
    }

    @Test
    fun parseNextLinkHandlesMultipleRels() {
        val header = """<https://example.com?page=1>; rel="prev", <https://example.com?page=3>; rel="next""""
        assertEquals("https://example.com?page=3", parseNextLink(header))
    }

    @Test
    fun parseNextLinkReturnsNullWhenNoNext() {
        assertNull(parseNextLink("""<https://example.com?page=1>; rel="prev""""))
        assertNull(parseNextLink(null))
        assertNull(parseNextLink(""))
    }

    // =========================================================================
    // parseNextLink — adversarial input
    //
    // The Link header is attacker-influenced (isSameOrigin exists to stop SSRF
    // through a poisoned one), so malformed shapes are a contract, not a
    // curiosity. The same six cases exist in all six SDKs.
    // =========================================================================

    @Test
    fun parseNextLinkReturnsNullWhenBracketNeverCloses() {
        assertNull(parseNextLink("""<https://api.example.com/page2; rel="next""""))
    }

    @Test
    fun parseNextLinkReadsClosingBracketBeforeOpeningBracket() {
        // Was broken here: indexOf('<') and indexOf('>') ran independently from
        // 0, so a '>' ahead of the '<' gave end < start and extraction silently
        // failed. The regex SDKs always read this correctly.
        assertEquals(
            "https://api.example.com/page2",
            parseNextLink(""">x<https://api.example.com/page2>; rel="next""""),
        )
    }

    @Test
    fun parseNextLinkTruncatesUrlAtFirstRawClosingBracket() {
        // Parity with the old <([^>]+)> spelling: [^>] cannot span a '>'.
        assertEquals(
            "https://api.example.com/page2?q=a",
            parseNextLink("""<https://api.example.com/page2?q=a>b>; rel="next""""),
        )
    }

    @Test
    fun parseNextLinkTakesFirstOfMultipleBracketPairsInOnePart() {
        // Parity with the old spelling: leftmost match wins.
        assertEquals(
            "https://api.example.com/a",
            parseNextLink("""<https://api.example.com/a> <https://api.example.com/b>; rel="next""""),
        )
    }

    @Test
    fun parseNextLinkSkipsEmptyBracketPair() {
        // Parity with the old spelling: [^>]+ requires at least one character,
        // so an empty <> is not a match and the scan moves on. A naive
        // indexOf('>', start + 1) without this check would return "".
        assertEquals(
            "https://api.example.com/page2",
            parseNextLink("""<> <https://api.example.com/page2>; rel="next""""),
        )
    }

    @Test
    fun parseNextLinkKeepsScanningPastAMalformedPart() {
        assertEquals(
            "https://api.example.com/page2",
            parseNextLink("""<malformed; rel="next", <https://api.example.com/page2>; rel="next""""),
        )
    }

    @Test
    fun parseNextLinkHandlesPathologicalHeader() {
        // Many '<' start positions with no reachable '>' — the shape that
        // punishes a backtracking regex. Asserting behaviour and completion,
        // not elapsed time: this suite already has timing flakiness (#655) and
        // a duration bound would add more.
        val many = "<".repeat(50_000)
        assertNull(parseNextLink("""$many; rel="next""""))
        // A '>' present but unreachable defeats the literal-prescan shortcut
        // some regex engines use to bail early.
        assertNull(parseNextLink(""">$many; rel="next""""))
    }

    @Test
    fun parseNextLinkHandlesManyEmptyBracketPairs() {
        // The pathological case for the scan that replaced the regex, which is
        // a different shape from the one above: that header returns after a
        // single indexOf('>') and never takes the empty-<> branch, so the skip
        // loop's own worst case went untested. Every "<>" here advances the
        // cursor by one and goes round again — the only path where a
        // non-constant-time index lookup would compound into quadratic
        // behaviour. Behaviour and completion again, not elapsed time.
        val pairs = "<>".repeat(50_000)
        // No non-empty pair anywhere: every iteration skips, then it runs out.
        assertNull(parseNextLink("""$pairs; rel="next""""))
        // Same prefix, but the skips have to land on a real pair at the end.
        assertEquals(
            "https://api.example.com/page2",
            parseNextLink("""$pairs<https://api.example.com/page2>; rel="next""""),
        )
    }

    // =========================================================================
    // isSameOrigin
    // =========================================================================

    @Test
    fun sameOriginMatchesExactly() {
        assertTrue(isSameOrigin(
            "https://3.basecampapi.com/12345/projects.json",
            "https://3.basecampapi.com/12345/todos.json",
        ))
    }

    @Test
    fun sameOriginRejectsDifferentHosts() {
        assertFalse(isSameOrigin(
            "https://3.basecampapi.com/12345/projects.json",
            "https://evil.com/12345/projects.json",
        ))
    }

    @Test
    fun sameOriginRejectsDifferentSchemes() {
        assertFalse(isSameOrigin(
            "https://example.com/path",
            "http://example.com/path",
        ))
    }

    @Test
    fun sameOriginRejectsDifferentPorts() {
        assertFalse(isSameOrigin(
            "https://example.com:443/path",
            "https://example.com:8443/path",
        ))
    }

    @Test
    fun sameOriginNormalizesDefaultPorts() {
        // An explicit default port is the same origin as no port (RFC 3986),
        // so e.g. a :443 pagination Link against a portless base must pass.
        assertTrue(isSameOrigin(
            "https://3.basecampapi.com:443/12345/projects.json",
            "https://3.basecampapi.com",
        ))
        assertTrue(isSameOrigin(
            "http://localhost:80/x",
            "http://localhost",
        ))
        // A non-default port is still a different origin.
        assertFalse(isSameOrigin(
            "https://3.basecampapi.com:8443/x",
            "https://3.basecampapi.com",
        ))
        // A query or fragment may directly follow the authority (no path); the
        // default port must still be normalized.
        assertTrue(isSameOrigin(
            "https://3.basecampapi.com:443?page=2",
            "https://3.basecampapi.com",
        ))
        assertTrue(isSameOrigin(
            "https://3.basecampapi.com:443#top",
            "https://3.basecampapi.com",
        ))
    }

    @Test
    fun sameOriginIgnoresSchemeCase() {
        // Scheme is case-insensitive (RFC 3986): an uppercase-scheme URL must
        // still be recognized as same-origin, not misclassified as foreign.
        assertTrue(isSameOrigin(
            "HTTPS://3.basecampapi.com/12345/projects.json",
            "https://3.basecampapi.com",
        ))
    }

    // =========================================================================
    // isLocalhost
    // =========================================================================

    @Test
    fun isLocalhostRecognizesLoopback() {
        assertTrue(isLocalhost("https://localhost:3000/x.json"))
        assertTrue(isLocalhost("http://127.0.0.1:8080/x"))
        // Hostnames are case-insensitive (RFC 3986).
        assertTrue(isLocalhost("https://LOCALHOST/x.json"))
        // RFC 6761 .localhost TLD.
        assertTrue(isLocalhost("https://myapp.localhost/x.json"))
    }

    @Test
    fun isLocalhostRejectsLocalhostLookalikes() {
        // A host that merely contains "localhost" is not localhost.
        assertFalse(isLocalhost("https://localhost.evil.example/x"))
        assertFalse(isLocalhost("https://notlocalhost/x"))
        // The host ends at ?, or # — localhost text in a query or fragment
        // must not make a foreign host pass the carve-out.
        assertFalse(isLocalhost("http://evil.example#foo.localhost"))
        assertFalse(isLocalhost("http://evil.example?x=.localhost"))
        // Userinfo is not the host: localhost text before '@' must not make a
        // foreign host pass the carve-out.
        assertFalse(isLocalhost("http://localhost:80@evil.example/path"))
        assertFalse(isLocalhost("http://localhost@evil.example/path"))
        // A genuine localhost URL with userinfo still qualifies.
        assertTrue(isLocalhost("http://user:secret@localhost:3000/x"))
    }

    @Test
    fun isLocalhostRecognizesBracketedIpv6Loopback() {
        // RFC 3986 requires IPv6 literals in URLs to be bracketed, e.g. [::1].
        assertTrue(isLocalhost("http://[::1]:8080/path"))
        assertTrue(isLocalhost("https://[::1]/x.json"))
    }

    @Test
    fun isLocalhostRejectsForeignHosts() {
        assertFalse(isLocalhost("https://3.basecampapi.com/x"))
        assertFalse(isLocalhost("https://evil.example/x"))
    }

    @Test
    fun isLocalhostLimitsCarveOutToHttpSchemes() {
        // The credential backstop must fail closed on non-HTTP(S) schemes,
        // even for localhost.
        assertFalse(isLocalhost("ws://localhost:3000/x"))
        assertFalse(isLocalhost("ftp://127.0.0.1/x"))
    }

    @Test
    fun guardsFailClosedOnRelativeInput() {
        // Ktor parses a scheme-less string as a relative reference against
        // http://localhost — the guards must reject it, not bless it.
        assertFalse(isLocalhost("localhost"))
        assertFalse(isLocalhost("evil.example/x"))
        assertFalse(isSameOrigin("3.basecampapi.com", "https://3.basecampapi.com"))
        assertFalse(isSameOrigin("https://3.basecampapi.com", "3.basecampapi.com"))
    }

    // =========================================================================
    // Parser-differential regression
    // =========================================================================

    /**
     * A security guard must decide with the SAME parser the transport uses to
     * dial. For each adversarial URL: whenever the guard blesses it, the host
     * Ktor would actually dial (parseUrl — what HttpClient.request uses) must
     * be the host the guard thought it blessed. Near-tautological after the
     * parseUrl rewrite — but it fails loudly if anyone reintroduces a second
     * parser here.
     */
    @Test
    fun guardDecidesWithTheTransportParser() {
        val base = "https://3.basecampapi.com"
        val corpus = listOf(
            """http://evil.example\.localhost/x""",
            "http://localhost@evil.example/x",
            "http://evil.example#foo.localhost",
            "http://evil.example?x=.localhost",
            "http://localhost:80@evil.example/x",
            "https://3.basecampapi.com:443@evil.example/x",
            "http://[::1]/x",
            "HTTPS://localhost/x",
            "https://3.basecampapi.com:443/x",
            "http://localhost.evil.example/x",
        )
        for (url in corpus) {
            val dialed = parseUrl(url)?.host?.lowercase()?.removePrefix("[")?.removeSuffix("]")
            if (isLocalhost(url)) {
                assertTrue(
                    dialed == "localhost" || dialed == "127.0.0.1" || dialed == "::1" ||
                        dialed?.endsWith(".localhost") == true,
                    "isLocalhost blessed $url but the transport dials $dialed",
                )
            }
            if (isSameOrigin(url, base)) {
                assertEquals(
                    parseUrl(base)!!.host.lowercase(), dialed,
                    "isSameOrigin blessed $url against $base but the transport dials $dialed",
                )
            }
        }
    }

    // =========================================================================
    // parseRetryAfter
    // =========================================================================

    @Test
    fun parseRetryAfterParsesSeconds() {
        assertEquals(30, parseRetryAfter("30"))
        assertEquals(1, parseRetryAfter("1"))
    }

    @Test
    fun parseRetryAfterReturnsNullForInvalid() {
        assertNull(parseRetryAfter(null))
        assertNull(parseRetryAfter(""))
        assertNull(parseRetryAfter("0"))
        assertNull(parseRetryAfter("-1"))
        assertNull(parseRetryAfter("not-a-number"))
    }

    /**
     * SPEC §6 step 2's POSITIVE half, which conformance cannot reach: a fixture
     * is a static literal with no clock, so a date near enough to assert a delay
     * against expires the day it is written and one far enough ahead to survive
     * would make a compliant SDK sleep for years (#780). These four tests are
     * therefore the only guard on the branch, which is how it stayed missing
     * from Kotlin for the SDK's whole life while the suite ran green (#564).
     *
     * The date is written out as an IMF-fixdate literal rather than formatted by
     * the same library the parser uses, so the accepted wire format is pinned
     * independently of the round trip asserted below.
     */
    /**
     * The literal's job is the WIRE FORMAT, so the assertion is only that a
     * future IMF-fixdate yields a positive delay. The magnitude belongs to the
     * dynamic test below, which owns the arithmetic.
     *
     * This bound used to be `> 1_000_000_000`, which was a calendar time bomb:
     * 1 January 2060 stops being a billion seconds away on **2028-04-23**, at
     * which point the test would have started failing while the parser stayed
     * perfectly correct. On 2029-01-01 the remaining interval is 978,220,800
     * seconds — false under the old bound, true under this one.
     *
     * A positive-delay bound decays too, just not for 32 years: the literal is
     * spent on 2060-01-01. That is the horizon of the literal itself and cannot
     * be pushed out without giving up the hand-written string, which is the
     * whole point of the test — it pins the format independently of the
     * formatter the parser uses. Recorded here rather than left to be
     * rediscovered.
     */
    @Test
    fun parseRetryAfterParsesFutureHttpDate() {
        val seconds = parseRetryAfter("Thu, 01 Jan 2060 00:00:00 GMT")
        assertNotNull(seconds, "a future IMF-fixdate must yield a delay")
        assertTrue(seconds > 0, "expected a positive delay, got $seconds")
    }

    /**
     * Pins the arithmetic rather than the format: three minutes ahead, so the
     * at-most-one-second lost to rounding cannot flip the sign or the assertion.
     */
    @Test
    fun parseRetryAfterComputesSecondsUntilHttpDate() {
        val threeMinutesOut = GMTDate(GMTDate().timestamp + 180_000).toHttpDate()
        val seconds = parseRetryAfter(threeMinutesOut)
        assertNotNull(seconds)
        assertTrue(seconds in 179..180, "expected ~180s, got $seconds")
    }

    /**
     * `max(0, date - now())` is not returned when it is zero: a past date lands
     * on step 3 and the caller backs off. Returning 0 here would mean "retry
     * immediately", which is the opposite instruction.
     */
    @Test
    fun parseRetryAfterRejectsPastHttpDate() {
        assertNull(parseRetryAfter("Wed, 09 Jun 2021 10:18:14 GMT"))
    }

    /**
     * A date beyond Int seconds must saturate, not wrap: a bare Long-to-Int
     * conversion turns a distant date into a negative delay, which is worse than
     * the missing branch it replaced.
     */
    @Test
    fun parseRetryAfterSaturatesRatherThanOverflowing() {
        val seconds = parseRetryAfter("Mon, 01 Jan 2300 00:00:00 GMT")
        assertEquals(Int.MAX_VALUE, seconds)
    }

    /**
     * Malformed input must fall through to step 3, not escape as an exception:
     * the parser sits on the retry path, where a throw would replace a backoff
     * with a crash. The ISO-8601 case is the realistic near miss.
     */
    @Test
    fun parseRetryAfterRejectsMalformedHttpDate() {
        assertNull(parseRetryAfter("Thu, 99 Xyz 2099 99:99:99 GMT"))
        assertNull(parseRetryAfter("2060-01-01T00:00:00Z"))
        assertNull(parseRetryAfter("Thu, 01 Jan 2060 00:00:00"))
    }

    // =========================================================================
    // parseTotalCount
    // =========================================================================

    @Test
    fun parseTotalCountExtractsValue() {
        val headers = mapOf("X-Total-Count" to listOf("42"))
        assertEquals(42, parseTotalCount(headers))
    }

    @Test
    fun parseTotalCountReturnsZeroForMissing() {
        assertEquals(0, parseTotalCount(emptyMap()))
    }

    @Test
    fun parseTotalCountReturnsZeroForInvalid() {
        val headers = mapOf("X-Total-Count" to listOf("not-a-number"))
        assertEquals(0, parseTotalCount(headers))
    }

    // =========================================================================
    // Paginated request integration tests
    // =========================================================================

    private fun projectJson(id: Long, name: String) = """{
        "id": $id, "status": "active", "name": "$name",
        "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
        "url": "https://3.basecampapi.com/12345/projects/$id.json",
        "app_url": "https://3.basecamp.com/12345/projects/$id",
        "dock": []
    }"""

    private val COMMENT_JSON = """{
        "id": 1,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2025-01-01T00:00:00Z",
        "updated_at": "2025-01-01T00:00:00Z",
        "title": "Re: Test",
        "inherits_status": true,
        "type": "Comment",
        "url": "https://3.basecampapi.com/12345/buckets/1/comments/1.json",
        "app_url": "https://3.basecamp.com/12345/buckets/1/comments/1",
        "content": "c1",
        "content_attachments": [],
        "parent": {"id": 100, "title": "Parent", "type": "Todo", "url": "https://3.basecampapi.com/12345/buckets/1/todos/100.json", "app_url": "https://3.basecamp.com/12345/buckets/1/todos/100"},
        "bucket": {"id": 1, "name": "Project", "type": "Project"},
        "creator": {"id": 1, "name": "Test User", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z"}
    }"""

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    @Test
    fun ssrfRejectionWhenLinkRedirectsToDifferentOrigin() = runTest {
        val client = mockClient { request ->
            respond(
                content = """[${projectJson(1, "Project 1")}]""",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "Link" to listOf("""<https://evil.com/12345/projects.json?page=2>; rel="next""""),
                    "X-Total-Count" to listOf("2"),
                ),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.projects.list()
            assertTrue(false, "Should have thrown for SSRF")
        } catch (e: BasecampException.Api) {
            assertTrue(e.message!!.contains("different origin"))
        }
        client.close()
    }

    @Test
    fun emptyResultNoItemsNoLinkHeader() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """[]""",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "X-Total-Count" to listOf("0"),
                ),
            )
        }

        val account = client.forAccount("12345")
        val projects = account.projects.list()

        assertEquals(0, projects.size)
        assertTrue(projects.isEmpty())
        assertEquals(0L, projects.meta.totalCount)
        assertFalse(projects.meta.truncated)
        client.close()
    }

    @Test
    fun paginationFollowsMultiplePages() = runTest {
        var requestCount = 0
        val client = mockClient { request ->
            requestCount++
            val page = request.url.parameters["page"]?.toIntOrNull() ?: 1
            when (page) {
                1 -> respond(
                    content = """[${projectJson(1, "Project 1")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/projects.json?page=2>; rel="next""""),
                        "X-Total-Count" to listOf("3"),
                    ),
                )
                2 -> respond(
                    content = """[${projectJson(2, "Project 2")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/projects.json?page=3>; rel="next""""),
                    ),
                )
                else -> respond(
                    content = """[${projectJson(3, "Project 3")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    ),
                )
            }
        }

        val account = client.forAccount("12345")
        val projects = account.projects.list()

        assertEquals(3, projects.size)
        assertEquals(1L, projects[0].id)
        assertEquals(2L, projects[1].id)
        assertEquals(3L, projects[2].id)
        assertEquals(3L, projects.meta.totalCount)
        assertFalse(projects.meta.truncated)
        client.close()
    }

    @Test
    fun maxItemsExactBoundaryOnFollowedPageIsNotTruncated() = runTest {
        val client = mockClient { request ->
            val page = request.url.parameters["page"]?.toIntOrNull() ?: 1
            when (page) {
                1 -> respond(
                    content = """[${projectJson(1, "Project 1")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/projects.json?page=2>; rel="next""""),
                        "X-Total-Count" to listOf("3"),
                    ),
                )
                else -> respond(  // 2 items, NO Link — collection ends exactly at maxItems
                    content = """[${projectJson(2, "Project 2")},${projectJson(3, "Project 3")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString())),
                )
            }
        }
        val projects = client.forAccount("12345").projects.list(ListProjectsOptions(maxItems = 3))
        assertEquals(3, projects.size)
        assertFalse(projects.meta.truncated)
        client.close()
    }

    @Test
    fun maxItemsDropOnFollowedPageIsTruncated() = runTest {
        // Keep-true companion: same fixture, maxItems=2 — page 2 overshoots the
        // cap (3 collected > 2), so an item is dropped and truncated must stay true.
        val client = mockClient { request ->
            val page = request.url.parameters["page"]?.toIntOrNull() ?: 1
            when (page) {
                1 -> respond(
                    content = """[${projectJson(1, "Project 1")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/projects.json?page=2>; rel="next""""),
                        "X-Total-Count" to listOf("3"),
                    ),
                )
                else -> respond(
                    content = """[${projectJson(2, "Project 2")},${projectJson(3, "Project 3")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString())),
                )
            }
        }
        val projects = client.forAccount("12345").projects.list(ListProjectsOptions(maxItems = 2))
        assertEquals(2, projects.size)
        assertTrue(projects.meta.truncated)
        client.close()
    }

    @Test
    fun maxItemsExactBoundaryWithNextLinkIsTruncated() = runTest {
        // Keep-true companion: collection ends exactly at maxItems but page 2
        // still advertises a next page — more items remain, so truncated is true.
        val client = mockClient { request ->
            val page = request.url.parameters["page"]?.toIntOrNull() ?: 1
            when (page) {
                1 -> respond(
                    content = """[${projectJson(1, "Project 1")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/projects.json?page=2>; rel="next""""),
                        "X-Total-Count" to listOf("5"),
                    ),
                )
                else -> respond(
                    content = """[${projectJson(2, "Project 2")},${projectJson(3, "Project 3")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/projects.json?page=3>; rel="next""""),
                    ),
                )
            }
        }
        val projects = client.forAccount("12345").projects.list(ListProjectsOptions(maxItems = 3))
        assertEquals(3, projects.size)
        assertTrue(projects.meta.truncated)
        client.close()
    }

    // =========================================================================
    // Wrapped pagination (PersonProgress)
    // =========================================================================

    private fun wrappedPageJson(events: List<Pair<Long, String>>) = buildString {
        append("""{"person":{"id":456,"name":"Jane Doe","email_address":"jane@example.com"},""")
        append(""""events":[""")
        append(events.joinToString(",") { (id, action) ->
            """{"id":$id,"action":"$action","target":"todo","title":"Event $id"}"""
        })
        append("]}")
    }

    @Test
    fun wrappedPaginationAccumulatesAcrossPages() = runTest {
        var requestCount = 0
        val client = mockClient { request ->
            requestCount++
            val page = request.url.parameters["page"]?.toIntOrNull() ?: 1
            when (page) {
                1 -> respond(
                    content = wrappedPageJson(listOf(1L to "created", 2L to "completed")),
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/reports/users/progress/456.json?page=2>; rel="next""""),
                        "X-Total-Count" to listOf("3"),
                    ),
                )
                else -> respond(
                    content = wrappedPageJson(listOf(3L to "updated")),
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    ),
                )
            }
        }

        val account = client.forAccount("12345")
        val result: PersonProgressResult = account.reports.personProgress(456)

        // Wrapper field preserved from page 1
        assertEquals("Jane Doe", result.person.name)

        // Events accumulated across both pages
        assertEquals(3, result.events.size)
        assertEquals("created", result.events[0].action)
        assertEquals("completed", result.events[1].action)
        assertEquals("updated", result.events[2].action)
        assertEquals(3L, result.events.meta.totalCount)
        assertFalse(result.events.meta.truncated)
        client.close()
    }

    @Test
    fun wrappedMaxItemsExactBoundaryOnFollowedPageIsNotTruncated() = runTest {
        val client = mockClient { request ->
            val page = request.url.parameters["page"]?.toIntOrNull() ?: 1
            when (page) {
                1 -> respond(
                    content = wrappedPageJson(listOf(1L to "created", 2L to "completed")),
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/reports/users/progress/456.json?page=2>; rel="next""""),
                        "X-Total-Count" to listOf("3"),
                    ),
                )
                else -> respond(  // 1 event, NO Link — collection ends exactly at maxItems
                    content = wrappedPageJson(listOf(3L to "updated")),
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString())),
                )
            }
        }
        val result = client.forAccount("12345").reports.personProgress(456, PaginationOptions(maxItems = 3))
        assertEquals(3, result.events.size)
        assertFalse(result.events.meta.truncated)
        client.close()
    }

    /**
     * The PaginationOptions compatibility overload reaches exactly the
     * operations whose options type changed — a compile-time assertion in both
     * directions.
     *
     * An operation that gained its first optional query parameter moved from
     * `options: PaginationOptions?` to its own options class, so it keeps a
     * bridge and the pre-#561 call shape still resolves; the explicitly typed
     * reference below fails to compile if the bridge is dropped. (The runtime
     * half of that direction is `personProgress(456, PaginationOptions(...))`
     * in the test above, which is the untouched pre-#561 call site.)
     *
     * An operation that always had its own options class gets no bridge, so an
     * untyped callable reference to it stays unambiguous. Nothing constrains
     * `unbridged` below, so a second applicable one-argument overload turns it
     * into an overload-resolution ambiguity and the suite stops compiling —
     * which is what a blanket bridge did to 33 operations.
     */
    @Test
    fun paginationOptionsBridgeReachesOnlyOperationsWhoseOptionsTypeChanged() {
        val bridged: suspend (CommentsService, Long, PaginationOptions?) -> ListResult<Comment> = CommentsService::list
        val unbridged = BookmarksService::listMyBookmarks
        val alsoUnbridged = SearchService::search

        assertNotNull(bridged)
        assertNotNull(unbridged)
        assertNotNull(alsoUnbridged)
    }

    /**
     * Every call shape `ListComments` accepted before it gained a `page`
     * parameter still compiles.
     *
     * The bridge keeps the old `options: PaginationOptions? = null` verbatim
     * rather than a non-null variant, because a caller holding a nullable
     * `PaginationOptions?` — `savedOptions` below — matches neither the new
     * options class nor a non-null bridge, and would have been broken by a
     * bridge that only accepted non-null values.
     *
     * The lambda is type-checked but never invoked: these are assertions about
     * overload resolution, not about the wire.
     */
    @Test
    fun everyPreExistingCallShapeStillResolves() {
        val savedOptions: PaginationOptions? = PaginationOptions(maxItems = 3)

        val shapes: suspend (CommentsService) -> Unit = { comments ->
            comments.list(1)                                        // no options
            comments.list(1, null)                                  // explicit null
            comments.list(1, PaginationOptions(maxItems = 3))       // non-null value
            comments.list(1, savedOptions)                          // nullable variable
            comments.list(1, options = savedOptions)                // named, nullable
            comments.list(1, ListCommentsOptions(page = 3))         // and the new shape
        }

        assertNotNull(shapes)
    }

    /**
     * The compatibility bridge must forward `page`, not just `maxItems`.
     *
     * `PaginationOptions` gained `page` in #566. A bridge that forwarded only
     * `maxItems` would drop it from BOTH the query string and the pagination
     * options, so `list(id, PaginationOptions(page = 3))` would auto-paginate
     * the whole collection — precisely the bug #566 exists to remove, reachable
     * through the older of the two call shapes.
     */
    @Test
    fun paginationOptionsBridgeForwardsThePinnedPage() = runTest {
        var requestCount = 0
        val seenPages = mutableListOf<String?>()
        val client = mockClient { request ->
            requestCount++
            seenPages += request.url.parameters["page"]
            respond(
                content = """[$COMMENT_JSON]""",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "Link" to listOf("""<https://3.basecampapi.com/12345/buckets/1/recordings/1/comments.json?page=4>; rel="next""""),
                ),
            )
        }

        val comments = CommentsService(client.forAccount("12345"))
        val result = comments.list(1, PaginationOptions(page = 3))

        assertEquals(listOf<String?>("3"), seenPages.toList())
        assertEquals(1, requestCount)
        assertEquals(1, result.size)
        assertTrue(result.meta.truncated)
        client.close()
    }
}
