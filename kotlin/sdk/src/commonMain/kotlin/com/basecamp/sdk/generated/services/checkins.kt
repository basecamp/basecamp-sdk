package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Checkins operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class CheckinsService(client: AccountClient) : BaseService(client) {

    /**
     * Get pending check-in reminders for the current user
     * @param options Optional query parameters and pagination control
     */
    suspend fun reminders(options: PaginationOptions? = null): ListResult<JsonElement> {
        val info = OperationInfo(
            service = "Checkins",
            operation = "GetQuestionReminders",
            resourceType = "question_reminder",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return requestPaginated(info, options, {
            httpGet("/my/question_reminders.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<JsonElement>>(body)
        }
    }

    /**
     * Get a single answer by id
     * @param answerId The answer ID
     */
    suspend fun getAnswer(answerId: Long): Answer {
        val info = OperationInfo(
            service = "Checkins",
            operation = "GetAnswer",
            resourceType = "answer",
            isMutation = false,
            projectId = null,
            resourceId = answerId,
        )
        return request(info, {
            httpGet("/question_answers/${answerId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Answer>(body)
        }
    }

    /**
     * Update an existing answer
     * @param answerId The answer ID
     * @param body Request body
     */
    suspend fun updateAnswer(answerId: Long, body: UpdateAnswerBody): Unit {
        val info = OperationInfo(
            service = "Checkins",
            operation = "UpdateAnswer",
            resourceType = "answer",
            isMutation = true,
            projectId = null,
            resourceId = answerId,
        )
        request(info, {
            httpPut("/question_answers/${answerId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("content", kotlinx.serialization.json.JsonPrimitive(body.content))
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * Get a questionnaire (automatic check-ins container) by id
     * @param questionnaireId The questionnaire ID
     */
    suspend fun getQuestionnaire(questionnaireId: Long): Questionnaire {
        val info = OperationInfo(
            service = "Checkins",
            operation = "GetQuestionnaire",
            resourceType = "questionnaire",
            isMutation = false,
            projectId = null,
            resourceId = questionnaireId,
        )
        return request(info, {
            httpGet("/questionnaires/${questionnaireId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Questionnaire>(body)
        }
    }

    /**
     * List all questions in a questionnaire
     * @param questionnaireId The questionnaire ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listQuestions(questionnaireId: Long, options: PaginationOptions? = null): ListResult<Question> {
        val info = OperationInfo(
            service = "Checkins",
            operation = "ListQuestions",
            resourceType = "question",
            isMutation = false,
            projectId = null,
            resourceId = questionnaireId,
        )
        return requestPaginated(info, options, {
            httpGet("/questionnaires/${questionnaireId}/questions.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Question>>(body)
        }
    }

    /**
     * Create a new question in a questionnaire
     * @param questionnaireId The questionnaire ID
     * @param body Request body
     */
    suspend fun createQuestion(questionnaireId: Long, body: CreateQuestionBody): Question {
        val info = OperationInfo(
            service = "Checkins",
            operation = "CreateQuestion",
            resourceType = "question",
            isMutation = true,
            projectId = null,
            resourceId = questionnaireId,
        )
        return request(info, {
            httpPost("/questionnaires/${questionnaireId}/questions.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("title", kotlinx.serialization.json.JsonPrimitive(body.title))
                put("schedule", body.schedule)
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Question>(body)
        }
    }

    /**
     * Get a single question by id
     * @param questionId The question ID
     */
    suspend fun getQuestion(questionId: Long): Question {
        val info = OperationInfo(
            service = "Checkins",
            operation = "GetQuestion",
            resourceType = "question",
            isMutation = false,
            projectId = null,
            resourceId = questionId,
        )
        return request(info, {
            httpGet("/questions/${questionId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Question>(body)
        }
    }

    /**
     * Update an existing question
     * @param questionId The question ID
     * @param body Request body
     */
    suspend fun updateQuestion(questionId: Long, body: UpdateQuestionBody): Question {
        val info = OperationInfo(
            service = "Checkins",
            operation = "UpdateQuestion",
            resourceType = "question",
            isMutation = true,
            projectId = null,
            resourceId = questionId,
        )
        return request(info, {
            httpPut("/questions/${questionId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.schedule?.let { put("schedule", it) }
                body.paused?.let { put("paused", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Question>(body)
        }
    }

    /**
     * List all answers for a question
     * @param questionId The question ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listAnswers(questionId: Long, options: PaginationOptions? = null): ListResult<Answer> {
        val info = OperationInfo(
            service = "Checkins",
            operation = "ListAnswers",
            resourceType = "answer",
            isMutation = false,
            projectId = null,
            resourceId = questionId,
        )
        return requestPaginated(info, options, {
            httpGet("/questions/${questionId}/answers.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Answer>>(body)
        }
    }

    /**
     * Create a new answer for a question
     * @param questionId The question ID
     * @param body Request body
     */
    suspend fun createAnswer(questionId: Long, body: CreateAnswerBody): Answer {
        val info = OperationInfo(
            service = "Checkins",
            operation = "CreateAnswer",
            resourceType = "answer",
            isMutation = true,
            projectId = null,
            resourceId = questionId,
        )
        return request(info, {
            httpPost("/questions/${questionId}/answers.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("content", kotlinx.serialization.json.JsonPrimitive(body.content))
                body.groupOn?.let { put("group_on", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Answer>(body)
        }
    }

    /**
     * List all people who have answered a question (answerers)
     * @param questionId The question ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun answerers(questionId: Long, options: PaginationOptions? = null): ListResult<Person> {
        val info = OperationInfo(
            service = "Checkins",
            operation = "ListQuestionAnswerers",
            resourceType = "question_answerer",
            isMutation = false,
            projectId = null,
            resourceId = questionId,
        )
        return requestPaginated(info, options, {
            httpGet("/questions/${questionId}/answers/by.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Person>>(body)
        }
    }

    /**
     * Get all answers from a specific person for a question
     * @param questionId The question ID
     * @param personId The person ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun byPerson(questionId: Long, personId: Long, options: PaginationOptions? = null): ListResult<Answer> {
        val info = OperationInfo(
            service = "Checkins",
            operation = "GetAnswersByPerson",
            resourceType = "answers_by_person",
            isMutation = false,
            projectId = null,
            resourceId = questionId,
        )
        return requestPaginated(info, options, {
            httpGet("/questions/${questionId}/answers/by/${personId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Answer>>(body)
        }
    }

    /**
     * Update notification settings for a check-in question
     * @param questionId The question ID
     * @param body Request body
     */
    suspend fun updateNotificationSettings(questionId: Long, body: UpdateQuestionNotificationSettingsBody): JsonElement {
        val info = OperationInfo(
            service = "Checkins",
            operation = "UpdateQuestionNotificationSettings",
            resourceType = "question_notification_setting",
            isMutation = true,
            projectId = null,
            resourceId = questionId,
        )
        return request(info, {
            httpPut("/questions/${questionId}/notification_settings.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.notifyOnAnswer?.let { put("notify_on_answer", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.digestIncludeUnanswered?.let { put("digest_include_unanswered", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Pause a check-in question (stops sending reminders)
     * @param questionId The question ID
     */
    suspend fun pause(questionId: Long): JsonElement {
        val info = OperationInfo(
            service = "Checkins",
            operation = "PauseQuestion",
            resourceType = "question",
            isMutation = true,
            projectId = null,
            resourceId = questionId,
        )
        return request(info, {
            httpPost("/questions/${questionId}/pause.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Resume a paused check-in question (resumes sending reminders)
     * @param questionId The question ID
     */
    suspend fun resume(questionId: Long): JsonElement {
        val info = OperationInfo(
            service = "Checkins",
            operation = "ResumeQuestion",
            resourceType = "question",
            isMutation = true,
            projectId = null,
            resourceId = questionId,
        )
        return request(info, {
            httpDelete("/questions/${questionId}/pause.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }
}
