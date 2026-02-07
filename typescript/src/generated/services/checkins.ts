/**
 * Checkins service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Answer entity from the Basecamp API. */
export type Answer = components["schemas"]["QuestionAnswer"];
/** Questionnaire entity from the Basecamp API. */
export type Questionnaire = components["schemas"]["Questionnaire"];
/** Question entity from the Basecamp API. */
export type Question = components["schemas"]["Question"];
/** Person entity from the Basecamp API. */
export type Person = components["schemas"]["Person"];

/**
 * Request parameters for updateAnswer.
 */
export interface UpdateAnswerCheckinRequest {
  /** Text content */
  content: string;
}

/**
 * Options for listQuestions.
 */
export interface ListQuestionsCheckinOptions extends PaginationOptions {
}

/**
 * Request parameters for createQuestion.
 */
export interface CreateQuestionCheckinRequest {
  /** Title */
  title: string;
  /** Schedule */
  schedule: components["schemas"]["QuestionSchedule"];
}

/**
 * Request parameters for updateQuestion.
 */
export interface UpdateQuestionCheckinRequest {
  /** Title */
  title?: string;
  /** Schedule */
  schedule?: components["schemas"]["QuestionSchedule"];
  /** Paused */
  paused?: boolean;
}

/**
 * Options for listAnswers.
 */
export interface ListAnswersCheckinOptions extends PaginationOptions {
}

/**
 * Request parameters for createAnswer.
 */
export interface CreateAnswerCheckinRequest {
  /** Text content */
  content: string;
  /** Group on (YYYY-MM-DD) */
  groupOn?: string;
}

/**
 * Options for answerers.
 */
export interface AnswerersCheckinOptions extends PaginationOptions {
}

/**
 * Options for byPerson.
 */
export interface ByPersonCheckinOptions extends PaginationOptions {
}

/**
 * Request parameters for updateNotificationSettings.
 */
export interface UpdateNotificationSettingsCheckinRequest {
  /** Notify when someone answers */
  notifyOnAnswer?: boolean;
  /** Include unanswered in digest */
  digestIncludeUnanswered?: boolean;
}

/**
 * Options for reminders.
 */
export interface RemindersCheckinOptions extends PaginationOptions {
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Checkins operations.
 */
export class CheckinsService extends BaseService {

  /**
   * Get a single answer by id
   * @param projectId - The project ID
   * @param answerId - The answer ID
   * @returns The Answer
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.checkins.getAnswer(123, 123);
   * ```
   */
  async getAnswer(projectId: number, answerId: number): Promise<Answer> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "GetAnswer",
        resourceType: "answer",
        isMutation: false,
        projectId,
        resourceId: answerId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/question_answers/{answerId}", {
          params: {
            path: { projectId, answerId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing answer
   * @param projectId - The project ID
   * @param answerId - The answer ID
   * @param req - Answer update parameters
   * @returns void
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * await client.checkins.updateAnswer(123, 123, { content: "Hello world" });
   * ```
   */
  async updateAnswer(projectId: number, answerId: number, req: UpdateAnswerCheckinRequest): Promise<void> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    await this.request(
      {
        service: "Checkins",
        operation: "UpdateAnswer",
        resourceType: "answer",
        isMutation: true,
        projectId,
        resourceId: answerId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/question_answers/{answerId}", {
          params: {
            path: { projectId, answerId },
          },
          body: {
            content: req.content,
          },
        })
    );
  }

  /**
   * Get a questionnaire (automatic check-ins container) by id
   * @param projectId - The project ID
   * @param questionnaireId - The questionnaire ID
   * @returns The Questionnaire
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.checkins.getQuestionnaire(123, 123);
   * ```
   */
  async getQuestionnaire(projectId: number, questionnaireId: number): Promise<Questionnaire> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "GetQuestionnaire",
        resourceType: "questionnaire",
        isMutation: false,
        projectId,
        resourceId: questionnaireId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/questionnaires/{questionnaireId}", {
          params: {
            path: { projectId, questionnaireId },
          },
        })
    );
    return response;
  }

  /**
   * List all questions in a questionnaire
   * @param projectId - The project ID
   * @param questionnaireId - The questionnaire ID
   * @param options - Optional query parameters
   * @returns All Question across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.checkins.listQuestions(123, 123);
   * ```
   */
  async listQuestions(projectId: number, questionnaireId: number, options?: ListQuestionsCheckinOptions): Promise<ListResult<Question>> {
    return this.requestPaginated(
      {
        service: "Checkins",
        operation: "ListQuestions",
        resourceType: "question",
        isMutation: false,
        projectId,
        resourceId: questionnaireId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json", {
          params: {
            path: { projectId, questionnaireId },
          },
        })
      , options
    );
  }

  /**
   * Create a new question in a questionnaire
   * @param projectId - The project ID
   * @param questionnaireId - The questionnaire ID
   * @param req - Question creation parameters
   * @returns The Question
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.checkins.createQuestion(123, 123, { title: "example", schedule: "example" });
   * ```
   */
  async createQuestion(projectId: number, questionnaireId: number, req: CreateQuestionCheckinRequest): Promise<Question> {
    if (!req.title) {
      throw Errors.validation("Title is required");
    }
    if (!req.schedule) {
      throw Errors.validation("Schedule is required");
    }
    const response = await this.request(
      {
        service: "Checkins",
        operation: "CreateQuestion",
        resourceType: "question",
        isMutation: true,
        projectId,
        resourceId: questionnaireId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json", {
          params: {
            path: { projectId, questionnaireId },
          },
          body: {
            title: req.title,
            schedule: req.schedule,
          },
        })
    );
    return response;
  }

  /**
   * Get a single question by id
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @returns The Question
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.checkins.getQuestion(123, 123);
   * ```
   */
  async getQuestion(projectId: number, questionId: number): Promise<Question> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "GetQuestion",
        resourceType: "question",
        isMutation: false,
        projectId,
        resourceId: questionId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/questions/{questionId}", {
          params: {
            path: { projectId, questionId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing question
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @param req - Question update parameters
   * @returns The Question
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.checkins.updateQuestion(123, 123, { });
   * ```
   */
  async updateQuestion(projectId: number, questionId: number, req: UpdateQuestionCheckinRequest): Promise<Question> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "UpdateQuestion",
        resourceType: "question",
        isMutation: true,
        projectId,
        resourceId: questionId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/questions/{questionId}", {
          params: {
            path: { projectId, questionId },
          },
          body: {
            title: req.title,
            schedule: req.schedule,
            paused: req.paused,
          },
        })
    );
    return response;
  }

  /**
   * List all answers for a question
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @param options - Optional query parameters
   * @returns All Answer across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.checkins.listAnswers(123, 123);
   * ```
   */
  async listAnswers(projectId: number, questionId: number, options?: ListAnswersCheckinOptions): Promise<ListResult<Answer>> {
    return this.requestPaginated(
      {
        service: "Checkins",
        operation: "ListAnswers",
        resourceType: "answer",
        isMutation: false,
        projectId,
        resourceId: questionId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/questions/{questionId}/answers.json", {
          params: {
            path: { projectId, questionId },
          },
        })
      , options
    );
  }

  /**
   * Create a new answer for a question
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @param req - Answer creation parameters
   * @returns The Answer
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.checkins.createAnswer(123, 123, { content: "Hello world" });
   * ```
   */
  async createAnswer(projectId: number, questionId: number, req: CreateAnswerCheckinRequest): Promise<Answer> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    if (req.groupOn && !/^\d{4}-\d{2}-\d{2}$/.test(req.groupOn)) {
      throw Errors.validation("Group on must be in YYYY-MM-DD format");
    }
    const response = await this.request(
      {
        service: "Checkins",
        operation: "CreateAnswer",
        resourceType: "answer",
        isMutation: true,
        projectId,
        resourceId: questionId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/questions/{questionId}/answers.json", {
          params: {
            path: { projectId, questionId },
          },
          body: {
            content: req.content,
            group_on: req.groupOn,
          },
        })
    );
    return response;
  }

  /**
   * List all people who have answered a question (answerers)
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @param options - Optional query parameters
   * @returns All Person across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.checkins.answerers(123, 123);
   * ```
   */
  async answerers(projectId: number, questionId: number, options?: AnswerersCheckinOptions): Promise<ListResult<Person>> {
    return this.requestPaginated(
      {
        service: "Checkins",
        operation: "ListQuestionAnswerers",
        resourceType: "question_answerer",
        isMutation: false,
        projectId,
        resourceId: questionId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/questions/{questionId}/answers/by.json", {
          params: {
            path: { projectId, questionId },
          },
        })
      , options
    );
  }

  /**
   * Get all answers from a specific person for a question
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @param personId - The person ID
   * @param options - Optional query parameters
   * @returns All Answer across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.checkins.byPerson(123, 123, 123);
   * ```
   */
  async byPerson(projectId: number, questionId: number, personId: number, options?: ByPersonCheckinOptions): Promise<ListResult<Answer>> {
    return this.requestPaginated(
      {
        service: "Checkins",
        operation: "GetAnswersByPerson",
        resourceType: "answers_by_person",
        isMutation: false,
        projectId,
        resourceId: questionId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/questions/{questionId}/answers/by/{personId}", {
          params: {
            path: { projectId, questionId, personId },
          },
        })
      , options
    );
  }

  /**
   * Update notification settings for a check-in question
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @param req - Question_notification_setting update parameters
   * @returns The question_notification_setting
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.checkins.updateNotificationSettings(123, 123, { });
   * ```
   */
  async updateNotificationSettings(projectId: number, questionId: number, req: UpdateNotificationSettingsCheckinRequest): Promise<components["schemas"]["UpdateQuestionNotificationSettingsResponseContent"]> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "UpdateQuestionNotificationSettings",
        resourceType: "question_notification_setting",
        isMutation: true,
        projectId,
        resourceId: questionId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/questions/{questionId}/notification_settings.json", {
          params: {
            path: { projectId, questionId },
          },
          body: {
            notify_on_answer: req.notifyOnAnswer,
            digest_include_unanswered: req.digestIncludeUnanswered,
          },
        })
    );
    return response;
  }

  /**
   * Pause a check-in question (stops sending reminders)
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @returns The question
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.checkins.pause(123, 123);
   * ```
   */
  async pause(projectId: number, questionId: number): Promise<components["schemas"]["PauseQuestionResponseContent"]> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "PauseQuestion",
        resourceType: "question",
        isMutation: true,
        projectId,
        resourceId: questionId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/questions/{questionId}/pause.json", {
          params: {
            path: { projectId, questionId },
          },
        })
    );
    return response;
  }

  /**
   * Resume a paused check-in question (resumes sending reminders)
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @returns The question
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.checkins.resume(123, 123);
   * ```
   */
  async resume(projectId: number, questionId: number): Promise<components["schemas"]["ResumeQuestionResponseContent"]> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "ResumeQuestion",
        resourceType: "question",
        isMutation: true,
        projectId,
        resourceId: questionId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/questions/{questionId}/pause.json", {
          params: {
            path: { projectId, questionId },
          },
        })
    );
    return response;
  }

  /**
   * Get pending check-in reminders for the current user
   * @param options - Optional query parameters
   * @returns All results across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.checkins.reminders();
   * ```
   */
  async reminders(options?: RemindersCheckinOptions): Promise<components["schemas"]["GetQuestionRemindersResponseContent"]> {
    return this.requestPaginated(
      {
        service: "Checkins",
        operation: "GetQuestionReminders",
        resourceType: "question_reminder",
        isMutation: false,
      },
      () =>
        this.client.GET("/my/question_reminders.json", {
        })
      , options
    );
  }
}