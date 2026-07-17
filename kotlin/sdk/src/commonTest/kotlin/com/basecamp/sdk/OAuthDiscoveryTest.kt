package com.basecamp.sdk

import com.basecamp.sdk.oauth.DiscoveryResult
import com.basecamp.sdk.oauth.OAuthConfig
import com.basecamp.sdk.oauth.ProtectedResourceMetadata
import com.basecamp.sdk.oauth.discover
import com.basecamp.sdk.oauth.discoverFromResource
import com.basecamp.sdk.oauth.discoverProtectedResource
import io.ktor.client.HttpClient
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.MockRequestHandleScope
import io.ktor.client.engine.mock.respond
import io.ktor.client.engine.mock.respondBadRequest
import io.ktor.client.request.HttpResponseData
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import io.ktor.utils.io.ByteReadChannel
import kotlinx.coroutines.TimeoutCancellationException
import kotlinx.coroutines.delay
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.withTimeout
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.fail

/**
 * Resource-first OAuth discovery tests (SPEC.md §16).
 *
 * These scenarios MIRROR the shared, data-only fixtures in
 * `conformance/oauth/fixtures` (same names, same expected outcomes).
 * commonTest can't portably read repo files, so the fixtures are embedded as
 * Kotlin data with the placeholder origins already substituted for this
 * harness's mock hosts and driven against a Ktor [MockEngine]. Hard cases assert
 * the Launchpad host received ZERO requests (`launchpadContacted == false`).
 */
class OAuthDiscoveryTest {

    // Mock origins substituted for the fixture {{...}} placeholders. LAUNCHPAD
    // must be the real origin because the fallback target is Launchpad.
    private val RESOURCE = "https://api.basecamp-test.example"
    private val ISSUER = "https://issuer.basecamp-test.example"
    private val LAUNCHPAD = "https://launchpad.37signals.com"
    private val BC5 = "https://bc5.basecamp-test.example"

    private enum class Op { FROM_RESOURCE, PROTECTED_RESOURCE, DISCOVER }

    /** One mocked HTTP hop. */
    private data class Hop(
        val status: Int = 200,
        val body: String? = null,
        val transportError: Boolean = false,
        val redirectTo: String? = null,
        val oversized: Boolean = false,
    )

    /** One discovery scenario mirroring a shared fixture. */
    private data class Scenario(
        val name: String,
        val op: Op,
        val resourceOrigin: String? = null,
        val issuerOrigin: String? = null,
        val expectedIssuer: String? = null,
        val hop1: Hop? = null,
        val hop2: Hop? = null,
        // Exactly one of the outcome fields is set.
        val selectedIssuer: String? = null,
        val fallbackReason: String? = null,
        val raiseReason: String? = null,      // DiscoverySelection reason token
        val raiseUsage: Boolean = false,      // usage error
        val raiseApiError: Boolean = false,   // discover/discoverProtectedResource hard failure
        // Coarse BasecampException.code the thrown error must map to, mirroring the
        // shared fixtures' `errorCategory`. Set on every raise scenario.
        val errorCategory: String? = null,
        val launchpadMustBeSilent: Boolean = false,
    )

    // Tracks whether the Launchpad (or attacker) host was contacted at all.
    private class Tracker {
        var launchpadContacted = false
        var attackerContacted = false
    }

    private fun MockRequestHandleScope.serve(hop: Hop): HttpResponseData {
        if (hop.transportError) throw RuntimeException("connection refused")
        if (hop.redirectTo != null) {
            return respond(
                content = ByteReadChannel(""),
                status = HttpStatusCode.fromValue(hop.status),
                headers = headersOf(HttpHeaders.Location, hop.redirectTo),
            )
        }
        val content = if (hop.oversized) {
            // A well-formed but oversized JSON document (> 1 MiB cap): the bounded
            // streaming read must abort before buffering the whole body.
            ByteReadChannel("{\"pad\":\"" + "x".repeat(1_100_000) + "\"}")
        } else {
            ByteReadChannel(hop.body ?: "")
        }
        return respond(
            content = content,
            status = HttpStatusCode.fromValue(hop.status),
            headers = headersOf(HttpHeaders.ContentType, "application/json"),
        )
    }

    private fun engineFor(scenario: Scenario, tracker: Tracker): HttpClient {
        val engine = MockEngine { request ->
            val host = request.url.host
            val path = request.url.encodedPath
            if (host.contains("launchpad")) tracker.launchpadContacted = true
            if (host.contains("attacker")) tracker.attackerContacted = true

            when {
                path.endsWith("oauth-protected-resource") ->
                    serve(scenario.hop1 ?: fail("${scenario.name}: no hop1 configured"))

                path.endsWith("oauth-authorization-server") && host.contains("launchpad") ->
                    // Documentary Launchpad fallback config; hard cases must never reach here.
                    respond(
                        content = ByteReadChannel(
                            "{\"issuer\":\"$LAUNCHPAD\",\"token_endpoint\":\"$LAUNCHPAD/authorization/token\"}"
                        ),
                        status = HttpStatusCode.OK,
                        headers = headersOf(HttpHeaders.ContentType, "application/json"),
                    )

                path.endsWith("oauth-authorization-server") ->
                    serve(scenario.hop2 ?: fail("${scenario.name}: no hop2 configured"))

                else -> respondBadRequest()
            }
        }
        return HttpClient(engine)
    }

    private suspend fun drive(scenario: Scenario, client: HttpClient): Any? = when (scenario.op) {
        Op.FROM_RESOURCE -> discoverFromResource(scenario.resourceOrigin!!, scenario.expectedIssuer, client)
        Op.PROTECTED_RESOURCE -> discoverProtectedResource(scenario.resourceOrigin!!, client)
        Op.DISCOVER -> discover(scenario.issuerOrigin!!, client)
    }

    private fun runScenario(scenario: Scenario) = runTest {
        val tracker = Tracker()
        val client = engineFor(scenario, tracker)
        try {
            val result: Any?
            var thrown: Throwable? = null
            try {
                result = drive(scenario, client)
            } catch (e: Throwable) {
                thrown = e
                // Assert on the throw path below.
                when {
                    scenario.raiseUsage -> assertIs<BasecampException.Usage>(e, "${scenario.name}: expected usage")
                    scenario.raiseApiError ->
                        assertIs<BasecampException>(e, "${scenario.name}: expected BasecampException")
                    scenario.raiseReason != null -> {
                        assertIs<BasecampException.DiscoverySelection>(e, "${scenario.name}: expected DiscoverySelection")
                        assertEquals(scenario.raiseReason, e.reason, scenario.name)
                    }
                    else -> fail("${scenario.name}: unexpected throw: $e")
                }
                // Every raise scenario carries its coarse errorCategory (mirrors the
                // shared fixtures); the thrown BasecampException.code must equal it.
                if (scenario.errorCategory != null) {
                    assertIs<BasecampException>(e, "${scenario.name}: expected BasecampException")
                    assertEquals(scenario.errorCategory, e.code, "${scenario.name}: errorCategory")
                }
                assertSilenceIfRequired(scenario, tracker)
                return@runTest
            }

            // No throw: must be a fallback or selected scenario.
            when {
                scenario.fallbackReason != null -> {
                    val r = assertIs<DiscoveryResult.FallBack>(result, scenario.name)
                    assertEquals(scenario.fallbackReason, r.reason.code, scenario.name)
                }
                scenario.selectedIssuer != null -> when (scenario.op) {
                    Op.FROM_RESOURCE -> {
                        val r = assertIs<DiscoveryResult.Selected>(result, scenario.name)
                        assertEquals(scenario.selectedIssuer, r.issuer, scenario.name)
                    }
                    Op.PROTECTED_RESOURCE -> {
                        val r = assertIs<ProtectedResourceMetadata>(result, scenario.name)
                        assertEquals(scenario.selectedIssuer, r.resource, scenario.name)
                    }
                    Op.DISCOVER -> {
                        val r = assertIs<OAuthConfig>(result, scenario.name)
                        assertEquals(scenario.selectedIssuer, r.issuer, scenario.name)
                    }
                }
                scenario.raiseUsage || scenario.raiseApiError || scenario.raiseReason != null ->
                    fail("${scenario.name}: expected a throw but got $result (thrown=$thrown)")
            }
            assertSilenceIfRequired(scenario, tracker)
        } finally {
            client.close()
        }
    }

    private fun assertSilenceIfRequired(scenario: Scenario, tracker: Tracker) {
        if (scenario.launchpadMustBeSilent) {
            assertFalse(tracker.launchpadContacted, "${scenario.name}: Launchpad must not be contacted")
            assertFalse(tracker.attackerContacted, "${scenario.name}: attacker host must not be contacted")
        }
    }

    // JSON body helpers -------------------------------------------------------

    private fun resourceBody(resource: String, servers: List<String>?): String {
        val serversJson = servers?.joinToString(",", prefix = "[", postfix = "]") { "\"$it\"" }
        return if (serversJson == null) "{\"resource\":\"$resource\"}"
        else "{\"resource\":\"$resource\",\"authorization_servers\":$serversJson}"
    }

    // =========================================================================
    // Fixtures 01-20 mirrored
    // =========================================================================

    @Test fun `01 two-hop happy path`() = runScenario(
        Scenario(
            name = "two-hop-happy-path",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            hop1 = Hop(body = resourceBody(RESOURCE, listOf(BC5, LAUNCHPAD))),
            hop2 = Hop(
                body = "{\"issuer\":\"$BC5\",\"authorization_endpoint\":\"$BC5/oauth/authorize\"," +
                    "\"token_endpoint\":\"$BC5/oauth/token\",\"device_authorization_endpoint\":\"$BC5/oauth/device\"," +
                    "\"grant_types_supported\":[\"urn:ietf:params:oauth:grant-type:device_code\",\"refresh_token\"]}"
            ),
            selectedIssuer = BC5,
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `02 no-as-advertised absent`() = runScenario(
        Scenario(
            name = "no-as-advertised-absent",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            hop1 = Hop(body = resourceBody(RESOURCE, null)),
            fallbackReason = "no_as_advertised",
        )
    )

    @Test fun `03 no-as-advertised empty array`() = runScenario(
        Scenario(
            name = "no-as-advertised-empty-array",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            hop1 = Hop(body = resourceBody(RESOURCE, emptyList())),
            fallbackReason = "no_as_advertised",
        )
    )

    @Test fun `04 only launchpad`() = runScenario(
        Scenario(
            name = "only-launchpad",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            hop1 = Hop(body = resourceBody(RESOURCE, listOf(LAUNCHPAD))),
            fallbackReason = "no_as_advertised",
        )
    )

    @Test fun `05 resource mismatch`() = runScenario(
        Scenario(
            name = "resource-mismatch",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            hop1 = Hop(body = resourceBody("https://attacker.example.com", listOf(BC5, LAUNCHPAD))),
            fallbackReason = "resource_discovery_failed",
        )
    )

    @Test fun `06 hop1 transport failure`() = runScenario(
        Scenario(
            name = "hop1-transport-failure",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            hop1 = Hop(transportError = true),
            fallbackReason = "resource_discovery_failed",
        )
    )

    @Test fun `07 issuer binding mismatch`() = runScenario(
        Scenario(
            name = "issuer-binding-mismatch",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            hop1 = Hop(body = resourceBody(RESOURCE, listOf(BC5, LAUNCHPAD))),
            hop2 = Hop(
                body = "{\"issuer\":\"https://impostor.example.com\"," +
                    "\"authorization_endpoint\":\"$BC5/oauth/authorize\",\"token_endpoint\":\"$BC5/oauth/token\"}"
            ),
            raiseReason = "issuer_mismatch",
            errorCategory = "api_error",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `08 as metadata 500`() = runScenario(
        Scenario(
            name = "as-metadata-500",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            hop1 = Hop(body = resourceBody(RESOURCE, listOf(BC5, LAUNCHPAD))),
            hop2 = Hop(status = 500, body = "{\"error\":\"internal_server_error\"}"),
            raiseReason = "as_fetch_failed",
            errorCategory = "api_error",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `09 ambiguous issuers`() = runScenario(
        Scenario(
            name = "ambiguous-issuers",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            hop1 = Hop(body = resourceBody(RESOURCE, listOf(BC5, "https://other-as.example.com", LAUNCHPAD))),
            raiseReason = "ambiguous_issuers",
            errorCategory = "api_error",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `10 expected issuer selected`() = runScenario(
        Scenario(
            name = "expected-issuer-selected",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            expectedIssuer = BC5,
            hop1 = Hop(body = resourceBody(RESOURCE, listOf(BC5, "https://other-as.example.com", LAUNCHPAD))),
            hop2 = Hop(
                body = "{\"issuer\":\"$BC5\",\"authorization_endpoint\":\"$BC5/oauth/authorize\"," +
                    "\"token_endpoint\":\"$BC5/oauth/token\"}"
            ),
            selectedIssuer = BC5,
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `11 expected issuer unavailable`() = runScenario(
        Scenario(
            name = "expected-issuer-unavailable",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            expectedIssuer = "https://not-advertised.example.com",
            hop1 = Hop(body = resourceBody(RESOURCE, listOf(BC5, LAUNCHPAD))),
            raiseReason = "expected_issuer_unavailable",
            errorCategory = "api_error",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `12 empty-string endpoint rejected`() = runScenario(
        Scenario(
            name = "empty-string-endpoint",
            op = Op.DISCOVER,
            issuerOrigin = ISSUER,
            hop2 = Hop(
                body = "{\"issuer\":\"$ISSUER\",\"authorization_endpoint\":\"$ISSUER/oauth/authorize\"," +
                    "\"token_endpoint\":\"\"}"
            ),
            raiseApiError = true,
            errorCategory = "api_error",
        )
    )

    @Test fun `13 device-only AS`() = runScenario(
        Scenario(
            name = "device-only-as",
            op = Op.DISCOVER,
            issuerOrigin = ISSUER,
            hop2 = Hop(
                body = "{\"issuer\":\"$ISSUER\",\"token_endpoint\":\"$ISSUER/oauth/token\"," +
                    "\"device_authorization_endpoint\":\"$ISSUER/oauth/device\"," +
                    "\"grant_types_supported\":[\"urn:ietf:params:oauth:grant-type:device_code\",\"refresh_token\"]}"
            ),
            selectedIssuer = ISSUER,
        )
    )

    @Test fun `14 origin-root http non-localhost`() = runScenario(
        Scenario(
            name = "origin-root-http-nonlocalhost",
            op = Op.PROTECTED_RESOURCE,
            resourceOrigin = "http://api.example.com",
            raiseUsage = true,
            errorCategory = "usage",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `15 origin-root malformed port`() = runScenario(
        Scenario(
            name = "origin-root-malformed-port",
            op = Op.PROTECTED_RESOURCE,
            resourceOrigin = "https://api.example.com:notaport",
            raiseUsage = true,
            errorCategory = "usage",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `16 origin-root path rejected`() = runScenario(
        Scenario(
            name = "origin-root-path-rejected",
            op = Op.PROTECTED_RESOURCE,
            resourceOrigin = "https://api.example.com/tenant/1",
            raiseUsage = true,
            errorCategory = "usage",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `17 origin-root ipv6 localhost accept`() = runScenario(
        Scenario(
            name = "origin-root-ipv6-localhost-accept",
            op = Op.PROTECTED_RESOURCE,
            resourceOrigin = "http://[::1]:3000",
            hop1 = Hop(body = resourceBody("http://[::1]:3000", listOf(LAUNCHPAD))),
            selectedIssuer = "http://[::1]:3000",
        )
    )

    @Test fun `18 ssrf oversized body`() = runScenario(
        Scenario(
            name = "ssrf-oversized-body",
            op = Op.DISCOVER,
            issuerOrigin = ISSUER,
            hop2 = Hop(oversized = true),
            raiseApiError = true,
            errorCategory = "api_error",
        )
    )

    @Test fun `19 ssrf redirect not followed`() = runScenario(
        Scenario(
            name = "ssrf-redirect-not-followed",
            op = Op.DISCOVER,
            issuerOrigin = ISSUER,
            hop2 = Hop(
                status = 302,
                redirectTo = "https://attacker.example.com/.well-known/oauth-authorization-server",
            ),
            raiseApiError = true,
            errorCategory = "api_error",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `20 invalid issuer origin`() = runScenario(
        Scenario(
            name = "invalid-issuer-origin",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            hop1 = Hop(body = resourceBody(RESOURCE, listOf("https://bc5.example.com/oauth", LAUNCHPAD))),
            raiseReason = "invalid_issuer_origin",
            errorCategory = "api_error",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `21 authorization_servers not array`() = runScenario(
        Scenario(
            name = "authorization-servers-not-array",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            // authorization_servers is a bare JSON string, not an array. Structural
            // decode into List<String>? must reject it as malformed hop-1 metadata
            // (never iterate its characters as issuers) → soft fallback to Launchpad.
            hop1 = Hop(body = "{\"resource\":\"$RESOURCE\",\"authorization_servers\":\"$BC5\"}"),
            fallbackReason = "resource_discovery_failed",
        )
    )

    @Test fun `22 grant_types not array`() = runScenario(
        Scenario(
            name = "grant-types-not-array",
            op = Op.DISCOVER,
            issuerOrigin = ISSUER,
            // grant_types_supported is a bare JSON string, not an array. Structural
            // decode into List<String> must reject it as invalid metadata (never
            // substring-match it, which would falsely enable device flow) → api_error.
            hop2 = Hop(
                body = "{\"issuer\":\"$ISSUER\",\"token_endpoint\":\"$ISSUER/oauth/token\"," +
                    "\"grant_types_supported\":\"urn:ietf:params:oauth:grant-type:device_code\"}"
            ),
            raiseApiError = true,
            errorCategory = "api_error",
        )
    )

    @Test fun `23 as metadata missing token_endpoint`() = runScenario(
        Scenario(
            name = "as-metadata-missing-token-endpoint",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            hop1 = Hop(body = resourceBody(RESOURCE, listOf(BC5, LAUNCHPAD))),
            // Committed AS: issuer binds correctly, but token_endpoint is absent.
            // This is a non-mismatch AS fault whose message never mentions "issuer
            // mismatch" — it must classify as as_fetch_failed via the marker TYPE,
            // never via message text.
            hop2 = Hop(body = "{\"issuer\":\"$BC5\",\"authorization_endpoint\":\"$BC5/oauth/authorize\"}"),
            raiseReason = "as_fetch_failed",
            errorCategory = "api_error",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `24 launchpad lookalike with path stays a candidate`() = runScenario(
        Scenario(
            name = "launchpad-lookalike-with-path",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            // A Launchpad look-alike carrying a path is NOT the Launchpad issuer
            // under the origin-root profile, so isLaunchpadIssuer must not exclude
            // it (comparing only isSameOrigin, which ignores the path, would). With
            // BC5 also advertised there are two non-Launchpad candidates → HARD
            // ambiguous_issuers, never a silent BC5 selection or Launchpad fallback.
            hop1 = Hop(body = resourceBody(RESOURCE, listOf("$LAUNCHPAD/path", BC5))),
            raiseReason = "ambiguous_issuers",
            errorCategory = "api_error",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `25 mixed-case launchpad host excluded`() = runScenario(
        Scenario(
            name = "mixed-case-launchpad-excluded",
            op = Op.FROM_RESOURCE,
            resourceOrigin = RESOURCE,
            // Hosts are case-insensitive, so a mixed-case Launchpad host must be
            // recognized as Launchpad and excluded — zero non-Launchpad issuers →
            // no_as_advertised, not committed as a distinct BC5 issuer.
            hop1 = Hop(body = resourceBody(RESOURCE, listOf("https://LAUNCHPAD.37signals.com"))),
            fallbackReason = "no_as_advertised",
        )
    )

    @Test fun `26 origin-root bare query rejected`() = runScenario(
        Scenario(
            name = "origin-root-bare-query-rejected",
            op = Op.PROTECTED_RESOURCE,
            // A bare '?' is a query-bearing origin (empty query) and must be rejected
            // like any other query, not normalized away.
            resourceOrigin = "https://api.example.com?",
            raiseUsage = true,
            errorCategory = "usage",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `24 duplicate issuer deduped`() = runScenario(
        Scenario(
            name = "duplicate-issuer-deduped",
            op = Op.FROM_RESOURCE,
            // The same non-Launchpad issuer advertised twice is ONE candidate: the
            // exclusion heuristic dedupes by code-point rather than raising ambiguous.
            resourceOrigin = RESOURCE,
            hop1 = Hop(body = resourceBody(RESOURCE, listOf(BC5, BC5, LAUNCHPAD))),
            hop2 = Hop(body = "{\"issuer\":\"$BC5\",\"token_endpoint\":\"$BC5/oauth/token\"}"),
            selectedIssuer = BC5,
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `25 origin-root bare fragment rejected`() = runScenario(
        Scenario(
            name = "origin-root-bare-fragment",
            op = Op.PROTECTED_RESOURCE,
            // A bare '#' is a fragment-bearing origin (empty fragment) and must be
            // rejected — Ktor has no trailingQuery equivalent for '#', so the raw
            // input is scanned.
            resourceOrigin = "https://api.example.com#",
            raiseUsage = true,
            errorCategory = "usage",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `26 advertised issuer trailing slash binds`() = runScenario(
        Scenario(
            name = "advertised-issuer-trailing-slash-binds",
            op = Op.FROM_RESOURCE,
            // The advertised issuer carries a trailing slash that normalizes away for
            // routing. Binding is code-point-exact against the ADVERTISED string, so
            // an AS echoing the trailing-slash issuer binds (no false issuer_mismatch)
            // and the selected issuer is the advertised spelling.
            resourceOrigin = RESOURCE,
            hop1 = Hop(body = resourceBody(RESOURCE, listOf("$BC5/"))),
            hop2 = Hop(body = "{\"issuer\":\"$BC5/\",\"token_endpoint\":\"$BC5/oauth/token\"}"),
            selectedIssuer = "$BC5/",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `27 authorization_servers present null`() = runScenario(
        Scenario(
            name = "authorization-servers-present-null",
            op = Op.FROM_RESOURCE,
            // A present authorization_servers: null is MALFORMED (not absent, not
            // empty): the nullable field must not collapse it to absent. It fails
            // hop-1 → soft resource_discovery_failed, never a silent no_as_advertised.
            resourceOrigin = RESOURCE,
            hop1 = Hop(body = "{\"resource\":\"$RESOURCE\",\"authorization_servers\":null}"),
            fallbackReason = "resource_discovery_failed",
        )
    )

    @Test fun `28 origin-root dangling port rejected`() = runScenario(
        Scenario(
            name = "origin-root-dangling-port",
            op = Op.PROTECTED_RESOURCE,
            // A dangling ":" ("https://host:") normalizes the port away; the raw
            // authority's trailing ":" must still be rejected.
            resourceOrigin = "https://api.example.com:",
            raiseUsage = true,
            errorCategory = "usage",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `29 origin-root empty userinfo rejected`() = runScenario(
        Scenario(
            name = "origin-root-empty-userinfo",
            op = Op.PROTECTED_RESOURCE,
            // Delimiter-only userinfo ("https://@host") reports an empty (falsy)
            // userinfo; the raw authority's "@" must be rejected on presence.
            resourceOrigin = "https://@api.example.com",
            raiseUsage = true,
            errorCategory = "usage",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `30 origin-root port zero rejected`() = runScenario(
        Scenario(
            name = "origin-root-port-zero",
            op = Op.PROTECTED_RESOURCE,
            // Ktor's url.port is the EFFECTIVE port, so ":0" looks like the default;
            // the raw authority's port token (0) must be rejected as out of range.
            resourceOrigin = "https://api.example.com:0",
            raiseUsage = true,
            errorCategory = "usage",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `31 resource trailing slash binds`() = runScenario(
        Scenario(
            name = "resource-trailing-slash-binds",
            op = Op.PROTECTED_RESOURCE,
            // The caller's identifier carries a trailing slash that normalizes away
            // for the fetch URL; binding is code-point-exact against the ORIGINAL
            // caller identifier, so a resource echoing the trailing slash binds.
            resourceOrigin = "$RESOURCE/",
            hop1 = Hop(body = "{\"resource\":\"$RESOURCE/\"}"),
            selectedIssuer = "$RESOURCE/",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `resource default port binds against the raw caller identifier`() = runScenario(
        Scenario(
            name = "resource-default-port-binds",
            op = Op.PROTECTED_RESOURCE,
            // Default-port variant of the raw-identifier bind: ":443" normalizes
            // away for the fetch URL but the resource must echo it by code-point.
            resourceOrigin = "$RESOURCE:443",
            hop1 = Hop(body = "{\"resource\":\"$RESOURCE:443\"}"),
            selectedIssuer = "$RESOURCE:443",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `32 origin-root signed port rejected`() = runScenario(
        Scenario(
            name = "origin-root-signed-port",
            op = Op.PROTECTED_RESOURCE,
            // "+1" would coerce to port 1 via toIntOrNull; the port token must be
            // restricted to ASCII digits so a sign is rejected.
            resourceOrigin = "https://api.example.com:+1",
            raiseUsage = true,
            errorCategory = "usage",
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `33 discover issuer trailing slash binds`() = runScenario(
        Scenario(
            name = "discover-issuer-trailing-slash-binds",
            op = Op.DISCOVER,
            // Public discover() binds against the caller's RAW issuer spelling: the
            // trailing slash normalizes away for the fetch URL but the AS issuer
            // must echo it by code-point (RFC 8414 §3.3).
            issuerOrigin = "$ISSUER/",
            hop2 = Hop(body = "{\"issuer\":\"$ISSUER/\",\"token_endpoint\":\"$ISSUER/oauth/token\"}"),
            selectedIssuer = "$ISSUER/",
        )
    )

    @Test fun `34 resource-first trailing slash binds`() = runScenario(
        Scenario(
            name = "resource-first-trailing-slash-binds",
            op = Op.FROM_RESOURCE,
            // The orchestrator passes the RAW resource identifier through to hop-1
            // binding, so a resource echoing the trailing-slash identifier binds and
            // selection proceeds (not a false resource_discovery_failed).
            resourceOrigin = "$RESOURCE/",
            hop1 = Hop(body = resourceBody("$RESOURCE/", listOf(BC5, LAUNCHPAD))),
            hop2 = Hop(body = "{\"issuer\":\"$BC5\",\"token_endpoint\":\"$BC5/oauth/token\"}"),
            selectedIssuer = BC5,
            launchpadMustBeSilent = true,
        )
    )

    @Test fun `discover surfaces issuer mismatch as api_error to external callers`() = runTest {
        // The module-private binding marker must NOT leak: an external discover()
        // caller sees an ordinary api_error, identical to any other invalid AS
        // metadata. Only discoverFromResource branches on the marker type.
        val engine = MockEngine {
            respond(
                content = ByteReadChannel(
                    "{\"issuer\":\"https://impostor.example.com\",\"token_endpoint\":\"$ISSUER/oauth/token\"}"
                ),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, "application/json"),
            )
        }
        val client = HttpClient(engine)
        try {
            val e = assertFailsWith<BasecampException.Api> { discover(ISSUER, client) }
            assertEquals("api_error", e.code)
        } finally {
            client.close()
        }
    }

    // =========================================================================
    // Parser-boundary asserts for the origin-root profile (no transport).
    // =========================================================================

    @Test fun `origin-root accepts bracketed ipv6 localhost`() {
        assertEquals("http://[::1]:3000", requireOriginRoot("http://[::1]:3000"))
    }

    @Test fun `origin-root rejects malformed port`() {
        assertFailsWith<BasecampException.Usage> { requireOriginRoot("https://api.example.com:notaport") }
    }

    @Test fun `origin-root rejects empty userinfo`() {
        // `https://@host` carries an empty userinfo; Ktor's Url.user is null for it,
        // so the raw authority must be inspected for an '@'.
        assertFailsWith<BasecampException.Usage> { requireOriginRoot("https://@api.example.com") }
    }

    @Test fun `origin-root rejects userinfo`() {
        assertFailsWith<BasecampException.Usage> { requireOriginRoot("https://user@api.example.com") }
        assertFailsWith<BasecampException.Usage> { requireOriginRoot("https://user:pass@api.example.com") }
    }

    @Test fun `discovery cancellation propagates and is not converted to network`() = runTest {
        // A cancelled discovery fetch must surface the CancellationException, never
        // soft-fall-back by masquerading as BasecampException.Network.
        val engine = MockEngine {
            delay(10_000)
            respond(
                content = ByteReadChannel("{}"),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, "application/json"),
            )
        }
        val client = HttpClient(engine)
        try {
            assertFailsWith<TimeoutCancellationException> {
                withTimeout(3_000) { discover(ISSUER, client) }
            }
        } finally {
            client.close()
        }
    }

    @Test fun `origin-root drops default port and trailing slash`() {
        assertEquals("https://api.example.com", requireOriginRoot("https://api.example.com/"))
        assertEquals("https://api.example.com", requireOriginRoot("https://api.example.com:443"))
    }
}
