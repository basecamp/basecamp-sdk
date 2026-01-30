/**
 * Service for Checkins operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Checkins operations
 */
export class CheckinsService extends BaseService {

  /**
   * Get a single answer by id
   */
  async getAnswer(projectId: number, answerId: number): Promise<components["schemas"]["GetAnswerResponseContent"]> {
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
   */
  async updateAnswer(projectId: number, answerId: number, req: components["schemas"]["QuestionAnswerUpdatePayload"]): Promise<components["schemas"]["UpdateAnswerResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Get a questionnaire (automatic check-ins container) by id
   */
  async getQuestionnaire(projectId: number, questionnaireId: number): Promise<components["schemas"]["GetQuestionnaireResponseContent"]> {
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
   */
  async listQuestions(projectId: number, questionnaireId: number): Promise<components["schemas"]["ListQuestionsResponseContent"]> {
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
   */
  async createQuestion(projectId: number, questionnaireId: number, req: components["schemas"]["CreateQuestionRequestContent"]): Promise<components["schemas"]["CreateQuestionResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Get a single question by id
   */
  async getQuestion(projectId: number, questionId: number): Promise<components["schemas"]["GetQuestionResponseContent"]> {
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
   */
  async updateQuestion(projectId: number, questionId: number, req: components["schemas"]["UpdateQuestionRequestContent"]): Promise<components["schemas"]["UpdateQuestionResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * List all answers for a question
   */
  async listAnswers(projectId: number, questionId: number): Promise<components["schemas"]["ListAnswersResponseContent"]> {
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
   */
  async createAnswer(projectId: number, questionId: number, req: components["schemas"]["QuestionAnswerPayload"]): Promise<components["schemas"]["CreateAnswerResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * List all people who have answered a question (answerers)
   */
  async answerers(projectId: number, questionId: number): Promise<components["schemas"]["ListQuestionAnswerersResponseContent"]> {
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
   */
  async byPerson(projectId: number, questionId: number, personId: number): Promise<components["schemas"]["GetAnswersByPersonResponseContent"]> {
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
    return response;
  }

  /**
   * Update notification settings for a check-in question
   */
  async updateNotificationSettings(projectId: number, questionId: number, req: components["schemas"]["UpdateQuestionNotificationSettingsRequestContent"]): Promise<void> {
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
          body: req,
        })
    );
  }

  /**
   * Pause a check-in question (stops sending reminders)
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
    return response;
  }
}