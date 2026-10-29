package com.basecamp.sdk

import com.basecamp.sdk.generated.recordings
import com.basecamp.sdk.generated.services.CopyRecordingBody
import com.basecamp.sdk.generated.services.MoveRecordingBody
import com.basecamp.sdk.services.FilingContainer
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.time.Duration.Companion.milliseconds

class FilingWaitTest {

    private fun mockClient(handler: MockRequestHandler): BasecampClient = testBasecampClient {
        accessToken("test-token")
        this.engine = MockEngine(handler)
    }

    private fun jsonHeaders() = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString())

    private fun filing(status: String, destination: Boolean = false): String {
        val dest = if (destination) {
            ""","destination_recording":{"id":900,"type":"Message","url":"u","app_url":"a"}"""
        } else {
            ""
        }
        return """{"id":4,"status":"$status","type":"Copy","source_recording_id":456,"url":"u"$dest}"""
    }

    @Test
    fun copyAndWaitPollsUntilCompleted() = runTest {
        var starts = 0
        var polls = 0
        val client = mockClient { request ->
            if (request.method == HttpMethod.Post) {
                starts++
                respond(filing("pending"), HttpStatusCode.Created, jsonHeaders())
            } else {
                polls++
                respond(filing(if (polls < 2) "processing" else "completed", polls >= 2), HttpStatusCode.OK, jsonHeaders())
            }
        }

        val result = client.forAccount("12345").recordings.copyAndWait(456, CopyRecordingBody(filingDestinationId = 789))

        assertEquals(900L, result.destinationRecording?.id)
        assertEquals(1, starts)
        assertEquals(2, polls)
        client.close()
    }

    @Test
    fun moveAndWaitReportsAFailedFiling() = runTest {
        val client = mockClient { request ->
            if (request.method == HttpMethod.Post) {
                respond(filing("pending"), HttpStatusCode.Created, jsonHeaders())
            } else {
                respond(filing("failed"), HttpStatusCode.OK, jsonHeaders())
            }
        }

        val error = assertFailsWith<BasecampException.FilingFailure> {
            client.forAccount("12345").recordings.moveAndWait(456, MoveRecordingBody(filingDestinationId = 789))
        }
        assertEquals(BasecampException.FILING_FAILED, error.reason)
        assertEquals(4L, error.filing?.id)
        client.close()
    }

    @Test
    fun theWaitEndsWhileProcessing() = runTest {
        var starts = 0
        val client = mockClient { request ->
            if (request.method == HttpMethod.Post) {
                starts++
                respond(filing("pending"), HttpStatusCode.Created, jsonHeaders())
            } else {
                respond(filing("processing"), HttpStatusCode.OK, jsonHeaders())
            }
        }

        val error = assertFailsWith<BasecampException.FilingFailure> {
            client.forAccount("12345").recordings.copyAndWait(
                456, CopyRecordingBody(filingDestinationId = 789), timeout = 600.milliseconds,
            )
        }
        assertEquals(BasecampException.FILING_UNSETTLED, error.reason)
        assertEquals("processing", error.filing?.status)
        assertFalse(error.retryable)
        assertEquals(1, starts)
        client.close()
    }

    @Test
    fun aFailedPollIsUnsettledNotRefused() = runTest {
        val client = mockClient { request ->
            if (request.method == HttpMethod.Post) {
                respond(filing("pending"), HttpStatusCode.Created, jsonHeaders())
            } else {
                respond("""{"error":"Not Found"}""", HttpStatusCode.NotFound, jsonHeaders())
            }
        }

        val error = assertFailsWith<BasecampException.FilingFailure> {
            client.forAccount("12345").recordings.copyAndWait(456, CopyRecordingBody(filingDestinationId = 789))
        }
        assertEquals(BasecampException.FILING_UNSETTLED, error.reason)
        assertEquals(BasecampException.CODE_API, error.code)
        assertIs<BasecampException.NotFound>(error.cause)
        client.close()
    }

    @Test
    fun aPollThatFailsOutsideTheSdkStillCarriesTheFiling() = runTest {
        val client = mockClient { request ->
            if (request.method == HttpMethod.Post) {
                respond(filing("pending"), HttpStatusCode.Created, jsonHeaders())
            } else {
                throw IllegalStateException("token store unavailable")
            }
        }

        val error = assertFailsWith<BasecampException.FilingFailure> {
            client.forAccount("12345").recordings.copyAndWait(456, CopyRecordingBody(filingDestinationId = 789))
        }
        assertEquals(BasecampException.FILING_UNSETTLED, error.reason)
        assertEquals(4L, error.filing?.id)
        client.close()
    }

    @Test
    fun aRefusedStartPollsNothing() = runTest {
        for (status in listOf(HttpStatusCode.Forbidden, HttpStatusCode.NotFound, HttpStatusCode.UnprocessableEntity)) {
            var requests = 0
            val client = mockClient {
                requests++
                respond("{}", status, jsonHeaders())
            }

            val error = assertFailsWith<BasecampException.FilingFailure> {
                client.forAccount("12345").recordings.moveAndWait(456, MoveRecordingBody(filingDestinationId = 789))
            }
            assertEquals(BasecampException.FILING_REFUSED, error.reason)
            assertEquals(status.value, error.httpStatus)
            assertEquals(1, requests)
            client.close()
        }
    }

    @Test
    fun aStartThatMayHaveFiledIsSentOnce() = runTest {
        for (status in listOf(HttpStatusCode.InternalServerError, HttpStatusCode.ServiceUnavailable)) {
            var requests = 0
            val client = mockClient {
                requests++
                respond("{}", status, jsonHeaders())
            }

            val error = assertFailsWith<BasecampException.FilingFailure> {
                client.forAccount("12345").recordings.copyAndWait(456, CopyRecordingBody(filingDestinationId = 789))
            }
            assertEquals(BasecampException.FILING_MAY_HAVE_STARTED, error.reason)
            assertFalse(error.retryable)
            assertEquals(1, requests)
            client.close()
        }
    }

    @Test
    fun filingContainers() {
        val rows = listOf(
            Triple("Kanban::Card", "Kanban::Triage", "kanban_board"),
            Triple("Kanban::Column", "Kanban::Board", "kanban_board"),
            Triple("Message", "Message::Board", "message_board"),
            Triple("Document", "Vault", "vault"),
            Triple("Upload", "Vault", "vault"),
            Triple("CloudFile", "Vault", "vault"),
            Triple("GoogleDocument", "Vault", "vault"),
            Triple("Vault", "Vault", "vault"),
            Triple("Schedule::Entry", "Schedule", "schedule"),
            Triple("Question", "Questionnaire", "questionnaire"),
            Triple("Inbox::Forward", "Inbox", "inbox"),
            Triple("Todolist", "Todoset", "todoset"),
            Triple("Todolist", "Todolist", "todolist"),
            Triple("Todo", "Todolist", "todolist"),
            Triple("Vault", "", null),
            Triple("Message::Board", "", null),
            Triple("Kanban::Triage", "Kanban::Board", null),
            Triple("Comment", "Message", null),
            Triple("Question::Answer", "Question", null),
        )
        for ((recording, parent, want) in rows) {
            val got = FilingContainer.of(recording, parent)
            assertEquals(want, got?.dockName, "$recording under $parent")
            if (got != null) assertEquals(want == "vault", got.takesFolders)
        }
    }
}
