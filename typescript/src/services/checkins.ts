/**
 * Checkins service for the Basecamp API.
 *
 * Checkins (also called Automatic Check-ins) are scheduled questions
 * that get sent to team members. The questionnaire contains questions,
 * and each question can have multiple answers from different people.
 *
 * @example
 * ```ts
 * const questionnaire = await client.checkins.getQuestionnaire(projectId, questionnaireId);
 * const questions = await client.checkins.listQuestions(projectId, questionnaireId);
 * const answer = await client.checkins.createAnswer(projectId, questionId, {
 *   content: "<p>Making great progress!</p>",
 * });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A person reference (simplified).
 */
export interface PersonRef {
  id: number;
  name: string;
  email_address?: string;
  avatar_url?: string;
  admin?: boolean;
  owner?: boolean;
}

/**
 * A bucket (project) reference.
 */
export interface BucketRef {
  id: number;
  name: string;
  type: string;
}

/**
 * A parent reference.
 */
export interface ParentRef {
  id: number;
  title: string;
  type: string;
  url: string;
  app_url: string;
}

/**
 * Schedule configuration for a check-in question.
 */
export interface QuestionSchedule {
  /** Frequency: "every_day", "every_week", etc. */
  frequency: string;
  /** Days of the week (0 = Sunday, 6 = Saturday) */
  days: number[];
  /** Hour of the day (0-23) */
  hour: number;
  /** Minute of the hour (0-59) */
  minute: number;
  /** Week instance for monthly schedules (optional) */
  week_instance?: number;
  /** Week interval for recurring schedules (optional) */
  week_interval?: number;
  /** Month interval for recurring schedules (optional) */
  month_interval?: number;
  /** Start date in ISO 8601 format (optional) */
  start_date?: string;
  /** End date in ISO 8601 format (optional) */
  end_date?: string;
}

/**
 * A Basecamp automatic check-in questionnaire.
 */
export interface Questionnaire {
  id: number;
  status: string;
  visible_to_clients: boolean;
  created_at: string;
  updated_at: string;
  title: string;
  inherits_status: boolean;
  type: string;
  url: string;
  app_url: string;
  bookmark_url: string;
  questions_url: string;
  questions_count: number;
  name: string;
  bucket?: BucketRef;
  creator?: PersonRef;
}

/**
 * A Basecamp automatic check-in question.
 */
export interface Question {
  id: number;
  status: string;
  visible_to_clients: boolean;
  created_at: string;
  updated_at: string;
  title: string;
  inherits_status: boolean;
  type: string;
  url: string;
  app_url: string;
  bookmark_url: string;
  subscription_url: string;
  paused: boolean;
  schedule?: QuestionSchedule;
  answers_count: number;
  answers_url: string;
  parent?: ParentRef;
  bucket?: BucketRef;
  creator?: PersonRef;
}

/**
 * An answer to a check-in question.
 */
export interface QuestionAnswer {
  id: number;
  status: string;
  visible_to_clients: boolean;
  created_at: string;
  updated_at: string;
  title: string;
  inherits_status: boolean;
  type: string;
  url: string;
  app_url: string;
  bookmark_url: string;
  subscription_url: string;
  comments_count: number;
  comments_url: string;
  content: string;
  group_on: string;
  parent?: ParentRef;
  bucket?: BucketRef;
  creator?: PersonRef;
}

/**
 * Request to create a new check-in question.
 */
export interface CreateQuestionRequest {
  /** Question text (required) */
  title: string;
  /** Schedule configuration (required) */
  schedule: QuestionSchedule;
}

/**
 * Request to update an existing check-in question.
 */
export interface UpdateQuestionRequest {
  /** Question text (optional) */
  title?: string;
  /** Schedule configuration (optional) */
  schedule?: QuestionSchedule;
  /** Whether the question is paused (optional) */
  paused?: boolean;
}

/**
 * Request to create an answer to a check-in question.
 */
export interface CreateAnswerRequest {
  /** Answer content in HTML (required) */
  content: string;
  /** Date to group the answer with, ISO 8601 format (optional) */
  groupOn?: string;
}

/**
 * Request to update an existing answer.
 */
export interface UpdateAnswerRequest {
  /** Updated answer content in HTML (required) */
  content: string;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp automatic check-ins.
 */
export class CheckinsService extends BaseService {
  /**
   * Gets a questionnaire by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param questionnaireId - The questionnaire ID
   * @returns The questionnaire
   * @throws BasecampError with code "not_found" if questionnaire doesn't exist
   *
   * @example
   * ```ts
   * const questionnaire = await client.checkins.getQuestionnaire(projectId, questionnaireId);
   * console.log(questionnaire.name, questionnaire.questions_count);
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
          params: { path: { projectId, questionnaireId } },
        })
    );

    return response as unknown as Questionnaire;
  }

  /**
   * Lists all questions in a questionnaire.
   *
   * @param projectId - The project (bucket) ID
   * @param questionnaireId - The questionnaire ID
   * @returns Array of questions
   *
   * @example
   * ```ts
   * const questions = await client.checkins.listQuestions(projectId, questionnaireId);
   * questions.forEach(q => console.log(q.title, q.paused ? "(paused)" : ""));
   * ```
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
          params: { path: { projectId, questionnaireId } },
        })
    );

    return (response ?? []) as Question[];
  }

  /**
   * Gets a question by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param questionId - The question ID
   * @returns The question
   * @throws BasecampError with code "not_found" if question doesn't exist
   *
   * @example
   * ```ts
   * const question = await client.checkins.getQuestion(projectId, questionId);
   * console.log(question.title, question.schedule);
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
          params: { path: { projectId, questionId } },
        })
    );

    return response as unknown as Question;
  }

  /**
   * Creates a new question in a questionnaire.
   *
   * @param projectId - The project (bucket) ID
   * @param questionnaireId - The questionnaire ID
   * @param req - Question creation parameters
   * @returns The created question
   * @throws BasecampError with code "validation" if title or schedule is missing
   *
   * @example
   * ```ts
   * const question = await client.checkins.createQuestion(projectId, questionnaireId, {
   *   title: "What did you work on today?",
   *   schedule: {
   *     frequency: "every_day",
   *     days: [1, 2, 3, 4, 5],
   *     hour: 16,
   *     minute: 0,
   *   },
   * });
   * ```
   */
  async createQuestion(
    projectId: number,
    questionnaireId: number,
    req: CreateQuestionRequest
  ): Promise<Question> {
    if (!req.title) {
      throw Errors.validation("Question title is required");
    }
    if (!req.schedule) {
      throw Errors.validation("Question schedule is required");
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
          params: { path: { projectId, questionnaireId } },
          body: {
            title: req.title,
            schedule: req.schedule,
          },
        })
    );

    return response as unknown as Question;
  }

  /**
   * Updates an existing question.
   *
   * @param projectId - The project (bucket) ID
   * @param questionId - The question ID
   * @param req - Question update parameters
   * @returns The updated question
   *
   * @example
   * ```ts
   * const question = await client.checkins.updateQuestion(projectId, questionId, {
   *   paused: true,
   * });
   * ```
   */
  async updateQuestion(
    projectId: number,
    questionId: number,
    req: UpdateQuestionRequest
  ): Promise<Question> {
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
          params: { path: { projectId, questionId } },
          body: {
            title: req.title,
            schedule: req.schedule,
            paused: req.paused,
          },
        })
    );

    return response as unknown as Question;
  }

  /**
   * Lists all answers for a question.
   *
   * @param projectId - The project (bucket) ID
   * @param questionId - The question ID
   * @returns Array of answers
   *
   * @example
   * ```ts
   * const answers = await client.checkins.listAnswers(projectId, questionId);
   * answers.forEach(a => console.log(a.creator?.name, a.content));
   * ```
   */
  async listAnswers(projectId: number, questionId: number): Promise<QuestionAnswer[]> {
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
          params: { path: { projectId, questionId } },
        })
    );

    return (response ?? []) as QuestionAnswer[];
  }

  /**
   * Gets an answer by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param answerId - The answer ID
   * @returns The answer
   * @throws BasecampError with code "not_found" if answer doesn't exist
   *
   * @example
   * ```ts
   * const answer = await client.checkins.getAnswer(projectId, answerId);
   * console.log(answer.content, answer.group_on);
   * ```
   */
  async getAnswer(projectId: number, answerId: number): Promise<QuestionAnswer> {
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
          params: { path: { projectId, answerId } },
        })
    );

    return response as unknown as QuestionAnswer;
  }

  /**
   * Creates a new answer for a question.
   *
   * @param projectId - The project (bucket) ID
   * @param questionId - The question ID
   * @param req - Answer creation parameters
   * @returns The created answer
   * @throws BasecampError with code "validation" if content is missing
   *
   * @example
   * ```ts
   * const answer = await client.checkins.createAnswer(projectId, questionId, {
   *   content: "<p>Finished the new feature!</p>",
   * });
   * ```
   */
  async createAnswer(
    projectId: number,
    questionId: number,
    req: CreateAnswerRequest
  ): Promise<QuestionAnswer> {
    if (!req.content) {
      throw Errors.validation("Answer content is required");
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
          params: { path: { projectId, questionId } },
          body: {
            content: req.content,
            group_on: req.groupOn,
          },
        })
    );

    return response as unknown as QuestionAnswer;
  }

  /**
   * Updates an existing answer.
   *
   * @param projectId - The project (bucket) ID
   * @param answerId - The answer ID
   * @param req - Answer update parameters
   * @throws BasecampError with code "validation" if content is missing
   *
   * @example
   * ```ts
   * await client.checkins.updateAnswer(projectId, answerId, {
   *   content: "<p>Updated my response.</p>",
   * });
   * ```
   */
  async updateAnswer(
    projectId: number,
    answerId: number,
    req: UpdateAnswerRequest
  ): Promise<void> {
    if (!req.content) {
      throw Errors.validation("Answer content is required");
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
          params: { path: { projectId, answerId } },
          body: {
            content: req.content,
          },
        })
    );
  }
}
