package com.basecamp.sdk

import com.basecamp.sdk.generated.reports
import com.basecamp.sdk.generated.services.GetProgressReportOptions
import io.ktor.client.engine.mock.*
import io.ktor.client.request.HttpRequestData
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class ProgressReportTest {

    private suspend fun capture(options: GetProgressReportOptions): HttpRequestData {
        var captured: HttpRequestData? = null
        val client = testBasecampClient {
            accessToken("test-token")
            engine = MockEngine { request ->
                captured = request
                respond(
                    content = "[]",
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
                )
            }
        }
        client.forAccount("12345").reports.progress(options)
        client.close()
        return captured!!
    }

    @Test
    fun progressSendsDateAndFiltersAsBc3ReadsThem() = runTest {
        val params = capture(
            GetProgressReportOptions(
                date = "2026-08-30",
                projectIds = listOf(27L, 45144734L),
                peopleIds = listOf(49285301L),
                page = 2L,
            ),
        ).url.parameters

        assertEquals("2026-08-30", params["date"])
        assertEquals(listOf("27", "45144734"), params.getAll("project_ids[]"))
        assertEquals(listOf("49285301"), params.getAll("people_ids[]"))
        assertEquals("2", params["page"])
        assertNull(params.getAll("project_ids"))
        assertNull(params.getAll("people_ids"))
    }

    @Test
    fun progressSendsNoQueryWhenUnfiltered() = runTest {
        assertEquals("", capture(GetProgressReportOptions()).url.encodedQuery)
    }
}
