// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ByPersonCheckinOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct RemindersCheckinOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct ListAnswersCheckinOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct AnswerersCheckinOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct ListQuestionsCheckinOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class CheckinsService: BaseService, @unchecked Sendable {
    public func createAnswer(questionId: Int, req: QuestionAnswerPayload) async throws -> Answer {
        return try await request(
            OperationInfo(service: "Checkins", operation: "CreateAnswer", resourceType: "answer", isMutation: true, resourceId: questionId),
            method: "POST",
            path: "/questions/\(questionId)/answers.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateAnswer")
        )
    }

    public func createQuestion(questionnaireId: Int, req: CreateQuestionRequest) async throws -> Question {
        return try await request(
            OperationInfo(service: "Checkins", operation: "CreateQuestion", resourceType: "question", isMutation: true, resourceId: questionnaireId),
            method: "POST",
            path: "/questionnaires/\(questionnaireId)/questions.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateQuestion")
        )
    }

    public func getAnswer(answerId: Int) async throws -> Answer {
        return try await request(
            OperationInfo(service: "Checkins", operation: "GetAnswer", resourceType: "answer", isMutation: false, resourceId: answerId),
            method: "GET",
            path: "/question_answers/\(answerId)",
            retryConfig: Metadata.retryConfig(for: "GetAnswer")
        )
    }

    public func byPerson(questionId: Int, personId: Int, options: ByPersonCheckinOptions? = nil) async throws -> ListResult<Answer> {
        return try await requestPaginated(
            OperationInfo(service: "Checkins", operation: "GetAnswersByPerson", resourceType: "answers_by_person", isMutation: false, resourceId: personId),
            path: "/questions/\(questionId)/answers/by/\(personId)",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetAnswersByPerson")
        )
    }

    public func getQuestion(questionId: Int) async throws -> Question {
        return try await request(
            OperationInfo(service: "Checkins", operation: "GetQuestion", resourceType: "question", isMutation: false, resourceId: questionId),
            method: "GET",
            path: "/questions/\(questionId)",
            retryConfig: Metadata.retryConfig(for: "GetQuestion")
        )
    }

    public func reminders(options: RemindersCheckinOptions? = nil) async throws -> ListResult<QuestionReminder> {
        return try await requestPaginated(
            OperationInfo(service: "Checkins", operation: "GetQuestionReminders", resourceType: "question_reminder", isMutation: false),
            path: "/my/question_reminders.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetQuestionReminders")
        )
    }

    public func getQuestionnaire(questionnaireId: Int) async throws -> Questionnaire {
        return try await request(
            OperationInfo(service: "Checkins", operation: "GetQuestionnaire", resourceType: "questionnaire", isMutation: false, resourceId: questionnaireId),
            method: "GET",
            path: "/questionnaires/\(questionnaireId)",
            retryConfig: Metadata.retryConfig(for: "GetQuestionnaire")
        )
    }

    public func listAnswers(questionId: Int, options: ListAnswersCheckinOptions? = nil) async throws -> ListResult<Answer> {
        return try await requestPaginated(
            OperationInfo(service: "Checkins", operation: "ListAnswers", resourceType: "answer", isMutation: false, resourceId: questionId),
            path: "/questions/\(questionId)/answers.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListAnswers")
        )
    }

    public func answerers(questionId: Int, options: AnswerersCheckinOptions? = nil) async throws -> ListResult<Person> {
        return try await requestPaginated(
            OperationInfo(service: "Checkins", operation: "ListQuestionAnswerers", resourceType: "question_answerer", isMutation: false, resourceId: questionId),
            path: "/questions/\(questionId)/answers/by.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListQuestionAnswerers")
        )
    }

    public func listQuestions(questionnaireId: Int, options: ListQuestionsCheckinOptions? = nil) async throws -> ListResult<Question> {
        return try await requestPaginated(
            OperationInfo(service: "Checkins", operation: "ListQuestions", resourceType: "question", isMutation: false, resourceId: questionnaireId),
            path: "/questionnaires/\(questionnaireId)/questions.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListQuestions")
        )
    }

    public func pause(questionId: Int) async throws -> PauseQuestionResponseContent {
        return try await request(
            OperationInfo(service: "Checkins", operation: "PauseQuestion", resourceType: "question", isMutation: true, resourceId: questionId),
            method: "POST",
            path: "/questions/\(questionId)/pause.json",
            retryConfig: Metadata.retryConfig(for: "PauseQuestion")
        )
    }

    public func resume(questionId: Int) async throws -> ResumeQuestionResponseContent {
        return try await request(
            OperationInfo(service: "Checkins", operation: "ResumeQuestion", resourceType: "question", isMutation: true, resourceId: questionId),
            method: "DELETE",
            path: "/questions/\(questionId)/pause.json",
            retryConfig: Metadata.retryConfig(for: "ResumeQuestion")
        )
    }

    public func updateAnswer(answerId: Int, req: QuestionAnswerUpdatePayload) async throws {
        try await requestVoid(
            OperationInfo(service: "Checkins", operation: "UpdateAnswer", resourceType: "answer", isMutation: true, resourceId: answerId),
            method: "PUT",
            path: "/question_answers/\(answerId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateAnswer")
        )
    }

    public func updateQuestion(questionId: Int, req: UpdateQuestionRequest) async throws -> Question {
        return try await request(
            OperationInfo(service: "Checkins", operation: "UpdateQuestion", resourceType: "question", isMutation: true, resourceId: questionId),
            method: "PUT",
            path: "/questions/\(questionId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateQuestion")
        )
    }

    public func updateNotificationSettings(questionId: Int, req: UpdateQuestionNotificationSettingsRequest) async throws -> UpdateQuestionNotificationSettingsResponseContent {
        return try await request(
            OperationInfo(service: "Checkins", operation: "UpdateQuestionNotificationSettings", resourceType: "question_notification_setting", isMutation: true, resourceId: questionId),
            method: "PUT",
            path: "/questions/\(questionId)/notification_settings.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateQuestionNotificationSettings")
        )
    }
}
