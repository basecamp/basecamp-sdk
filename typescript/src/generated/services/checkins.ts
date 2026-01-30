/**
 * Checkins service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

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
  /** content */
  content: string;
}

/**
 * Request parameters for createQuestion.
 */
export interface CreateQuestionCheckinRequest {
  /** title */
  title: string;
  /** schedule */
  schedule: components["schemas"]["QuestionSchedule"];
}

/**
 * Request parameters for updateQuestion.
 */
export interface UpdateQuestionCheckinRequest {
  /** title */
  title?: string;
  /** schedule */
  schedule?: components["schemas"]["QuestionSchedule"];
  /** paused */
  paused?: boolean;
}

/**
 * Request parameters for createAnswer.
 */
export interface CreateAnswerCheckinRequest {
  /** content */
  content: string;
  /** group on (YYYY-MM-DD) */
  groupOn?: string;
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
   * Get a single answer by id
   * @param projectId - The project ID
   * @param answerId - The answer ID
   * @returns The Answer
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
   * @param req - Request parameters
   * @returns The Answer
   */
  async updateAnswer(projectId: number, answerId: number, req: UpdateAnswerCheckinRequest): Promise<Answer> {
    const response = await this.request(
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
          body: req as any,
        })
    );
    return response;
  }

  /**
   * Get a questionnaire (automatic check-ins container) by id
   * @param projectId - The project ID
   * @param questionnaireId - The questionnaire ID
   * @returns The Questionnaire
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
   * @returns Array of Question
   */
  async listQuestions(projectId: number, questionnaireId: number): Promise<Question[]> {
    const response = await this.request(
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
    );
    return response ?? [];
  }

  /**
   * Create a new question in a questionnaire
   * @param projectId - The project ID
   * @param questionnaireId - The questionnaire ID
   * @param req - Request parameters
   * @returns The Question
   *
   * @example
   * ```ts
   * const result = await client.checkins.createQuestion(123, 123, { ... });
   * ```
   */
  async createQuestion(projectId: number, questionnaireId: number, req: CreateQuestionCheckinRequest): Promise<Question> {
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
          body: req as any,
        })
    );
    return response;
  }

  /**
   * Get a single question by id
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @returns The Question
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
   * @param req - Request parameters
   * @returns The Question
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
          body: req as any,
        })
    );
    return response;
  }

  /**
   * List all answers for a question
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @returns Array of Answer
   */
  async listAnswers(projectId: number, questionId: number): Promise<Answer[]> {
    const response = await this.request(
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
    );
    return response ?? [];
  }

  /**
   * Create a new answer for a question
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @param req - Request parameters
   * @returns The Answer
   *
   * @example
   * ```ts
   * const result = await client.checkins.createAnswer(123, 123, { ... });
   * ```
   */
  async createAnswer(projectId: number, questionId: number, req: CreateAnswerCheckinRequest): Promise<Answer> {
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
   * @returns Array of Person
   */
  async answerers(projectId: number, questionId: number): Promise<Person[]> {
    const response = await this.request(
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
    );
    return response ?? [];
  }

  /**
   * Get all answers from a specific person for a question
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @param personId - The person ID
   * @returns Array of Answer
   */
  async byPerson(projectId: number, questionId: number, personId: number): Promise<Answer[]> {
    const response = await this.request(
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
    );
    return response ?? [];
  }

  /**
   * Update notification settings for a check-in question
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @param req - Request parameters
   * @returns void
   */
  async updateNotificationSettings(projectId: number, questionId: number, req: UpdateNotificationSettingsCheckinRequest): Promise<void> {
    await this.request(
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
  }

  /**
   * Pause a check-in question (stops sending reminders)
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @returns void
   */
  async pause(projectId: number, questionId: number): Promise<void> {
    await this.request(
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
  }

  /**
   * Resume a paused check-in question (resumes sending reminders)
   * @param projectId - The project ID
   * @param questionId - The question ID
   * @returns void
   */
  async resume(projectId: number, questionId: number): Promise<void> {
    await this.request(
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
  }

  /**
   * Get pending check-in reminders for the current user
   * @returns Array of results
   */
  async reminders(): Promise<components["schemas"]["GetQuestionRemindersResponseContent"]> {
    const response = await this.request(
      {
        service: "Checkins",
        operation: "GetQuestionReminders",
        resourceType: "question_reminder",
        isMutation: false,
      },
      () =>
        this.client.GET("/my/question_reminders.json", {
        })
    );
    return response ?? [];
  }
}