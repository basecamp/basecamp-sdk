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
 * Options for reminders.
 */
export interface RemindersCheckinOptions extends PaginationOptions {
}

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


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Checkins operations.
 */
export class CheckinsService extends BaseService {

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

  /**
   * Get a single answer by id
   * @param answerId - The answer ID
   * @returns The Answer
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.checkins.getAnswer(123);
   * ```
   */
  async getAnswer(answerId: number): Promise<Answer> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "GetAnswer",
        resourceType: "answer",
        isMutation: false,
        resourceId: answerId,
      },
      () =>
        this.client.GET("/question_answers/{answerId}", {
          params: {
            path: { answerId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing answer
   * @param answerId - The answer ID
   * @param req - Answer update parameters
   * @returns void
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * await client.checkins.updateAnswer(123, { content: "Hello world" });
   * ```
   */
  async updateAnswer(answerId: number, req: UpdateAnswerCheckinRequest): Promise<void> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    await this.request(
      {
        service: "Checkins",
        operation: "UpdateAnswer",
        resourceType: "answer",
        isMutation: true,
        resourceId: answerId,
      },
      () =>
        this.client.PUT("/question_answers/{answerId}", {
          params: {
            path: { answerId },
          },
          body: {
            content: req.content,
          },
        })
    );
  }

  /**
   * Get a questionnaire (automatic check-ins container) by id
   * @param questionnaireId - The questionnaire ID
   * @returns The Questionnaire
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.checkins.getQuestionnaire(123);
   * ```
   */
  async getQuestionnaire(questionnaireId: number): Promise<Questionnaire> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "GetQuestionnaire",
        resourceType: "questionnaire",
        isMutation: false,
        resourceId: questionnaireId,
      },
      () =>
        this.client.GET("/questionnaires/{questionnaireId}", {
          params: {
            path: { questionnaireId },
          },
        })
    );
    return response;
  }

  /**
   * List all questions in a questionnaire
   * @param questionnaireId - The questionnaire ID
   * @param options - Optional query parameters
   * @returns All Question across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.checkins.listQuestions(123);
   * ```
   */
  async listQuestions(questionnaireId: number, options?: ListQuestionsCheckinOptions): Promise<ListResult<Question>> {
    return this.requestPaginated(
      {
        service: "Checkins",
        operation: "ListQuestions",
        resourceType: "question",
        isMutation: false,
        resourceId: questionnaireId,
      },
      () =>
        this.client.GET("/questionnaires/{questionnaireId}/questions.json", {
          params: {
            path: { questionnaireId },
          },
        })
      , options
    );
  }

  /**
   * Create a new question in a questionnaire
   * @param questionnaireId - The questionnaire ID
   * @param req - Question creation parameters
   * @returns The Question
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.checkins.createQuestion(123, { title: "example", schedule: "example" });
   * ```
   */
  async createQuestion(questionnaireId: number, req: CreateQuestionCheckinRequest): Promise<Question> {
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
        resourceId: questionnaireId,
      },
      () =>
        this.client.POST("/questionnaires/{questionnaireId}/questions.json", {
          params: {
            path: { questionnaireId },
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
   * @param questionId - The question ID
   * @returns The Question
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.checkins.getQuestion(123);
   * ```
   */
  async getQuestion(questionId: number): Promise<Question> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "GetQuestion",
        resourceType: "question",
        isMutation: false,
        resourceId: questionId,
      },
      () =>
        this.client.GET("/questions/{questionId}", {
          params: {
            path: { questionId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing question
   * @param questionId - The question ID
   * @param req - Question update parameters
   * @returns The Question
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.checkins.updateQuestion(123, { });
   * ```
   */
  async updateQuestion(questionId: number, req: UpdateQuestionCheckinRequest): Promise<Question> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "UpdateQuestion",
        resourceType: "question",
        isMutation: true,
        resourceId: questionId,
      },
      () =>
        this.client.PUT("/questions/{questionId}", {
          params: {
            path: { questionId },
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
   * @param questionId - The question ID
   * @param options - Optional query parameters
   * @returns All Answer across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.checkins.listAnswers(123);
   * ```
   */
  async listAnswers(questionId: number, options?: ListAnswersCheckinOptions): Promise<ListResult<Answer>> {
    return this.requestPaginated(
      {
        service: "Checkins",
        operation: "ListAnswers",
        resourceType: "answer",
        isMutation: false,
        resourceId: questionId,
      },
      () =>
        this.client.GET("/questions/{questionId}/answers.json", {
          params: {
            path: { questionId },
          },
        })
      , options
    );
  }

  /**
   * Create a new answer for a question
   * @param questionId - The question ID
   * @param req - Answer creation parameters
   * @returns The Answer
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.checkins.createAnswer(123, { content: "Hello world" });
   * ```
   */
  async createAnswer(questionId: number, req: CreateAnswerCheckinRequest): Promise<Answer> {
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
        resourceId: questionId,
      },
      () =>
        this.client.POST("/questions/{questionId}/answers.json", {
          params: {
            path: { questionId },
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
   * @param questionId - The question ID
   * @param options - Optional query parameters
   * @returns All Person across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.checkins.answerers(123);
   * ```
   */
  async answerers(questionId: number, options?: AnswerersCheckinOptions): Promise<ListResult<Person>> {
    return this.requestPaginated(
      {
        service: "Checkins",
        operation: "ListQuestionAnswerers",
        resourceType: "question_answerer",
        isMutation: false,
        resourceId: questionId,
      },
      () =>
        this.client.GET("/questions/{questionId}/answers/by.json", {
          params: {
            path: { questionId },
          },
        })
      , options
    );
  }

  /**
   * Get all answers from a specific person for a question
   * @param questionId - The question ID
   * @param personId - The person ID
   * @param options - Optional query parameters
   * @returns All Answer across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.checkins.byPerson(123, 123);
   * ```
   */
  async byPerson(questionId: number, personId: number, options?: ByPersonCheckinOptions): Promise<ListResult<Answer>> {
    return this.requestPaginated(
      {
        service: "Checkins",
        operation: "GetAnswersByPerson",
        resourceType: "answers_by_person",
        isMutation: false,
        resourceId: questionId,
      },
      () =>
        this.client.GET("/questions/{questionId}/answers/by/{personId}", {
          params: {
            path: { questionId, personId },
          },
        })
      , options
    );
  }

  /**
   * Update notification settings for a check-in question
   * @param questionId - The question ID
   * @param req - Question_notification_setting update parameters
   * @returns The question_notification_setting
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.checkins.updateNotificationSettings(123, { });
   * ```
   */
  async updateNotificationSettings(questionId: number, req: UpdateNotificationSettingsCheckinRequest): Promise<components["schemas"]["UpdateQuestionNotificationSettingsResponseContent"]> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "UpdateQuestionNotificationSettings",
        resourceType: "question_notification_setting",
        isMutation: true,
        resourceId: questionId,
      },
      () =>
        this.client.PUT("/questions/{questionId}/notification_settings.json", {
          params: {
            path: { questionId },
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
   * @param questionId - The question ID
   * @returns The question
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.checkins.pause(123);
   * ```
   */
  async pause(questionId: number): Promise<components["schemas"]["PauseQuestionResponseContent"]> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "PauseQuestion",
        resourceType: "question",
        isMutation: true,
        resourceId: questionId,
      },
      () =>
        this.client.POST("/questions/{questionId}/pause.json", {
          params: {
            path: { questionId },
          },
        })
    );
    return response;
  }

  /**
   * Resume a paused check-in question (resumes sending reminders)
   * @param questionId - The question ID
   * @returns The question
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.checkins.resume(123);
   * ```
   */
  async resume(questionId: number): Promise<components["schemas"]["ResumeQuestionResponseContent"]> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "ResumeQuestion",
        resourceType: "question",
        isMutation: true,
        resourceId: questionId,
      },
      () =>
        this.client.DELETE("/questions/{questionId}/pause.json", {
          params: {
            path: { questionId },
          },
        })
    );
    return response;
  }
}