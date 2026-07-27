/**
 * Tests for the CheckinsService (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - Client-side checks: required fields on createQuestion (title, schedule),
 *   createAnswer and updateAnswer (content), plus a YYYY-MM-DD format check on
 *   groupOn. Everything else the API validates.
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient, type BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";

// Minimal valid QuestionSchedule, so the createQuestion validation tests vary
// only the field under test.
const SCHEDULE = {
  frequency: "every_day",
  days: [1, 2, 3, 4, 5],
  hour: 16,
  minute: 0,
} as never;

const BASE_URL = "https://3.basecampapi.com/12345";

describe("CheckinsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
  });

  describe("getQuestionnaire", () => {
    it("should get a questionnaire by ID", async () => {
      const mockQuestionnaire = {
        id: 100,
        status: "active",
        visible_to_clients: false,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
        title: "Automatic Check-ins",
        inherits_status: true,
        type: "Questionnaire",
        url: "https://3.basecampapi.com/12345/questionnaires/100.json",
        app_url: "https://3.basecamp.com/12345/questionnaires/100",
        bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
        questions_url:
          "https://3.basecampapi.com/12345/questionnaires/100/questions.json",
        questions_count: 2,
        name: "Automatic Check-ins",
        bucket: { id: 1, name: "Test Project", type: "Project" },
      };

      server.use(
        http.get(`${BASE_URL}/questionnaires/100`, () => {
          return HttpResponse.json(mockQuestionnaire);
        }),
      );

      const questionnaire = await client.checkins.getQuestionnaire(100);

      expect(questionnaire.id).toBe(100);
      expect(questionnaire.name).toBe("Automatic Check-ins");
      expect(questionnaire.questions_count).toBe(2);
    });
  });

  describe("listQuestions", () => {
    it("should list all questions in a questionnaire", async () => {
      const mockQuestions = [
        {
          id: 1,
          status: "active",
          visible_to_clients: false,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
          title: "What did you work on today?",
          inherits_status: true,
          type: "Question",
          url: "https://3.basecampapi.com/12345/questions/1.json",
          app_url: "https://3.basecamp.com/12345/questions/1",
          bookmark_url:
            "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
          subscription_url:
            "https://3.basecampapi.com/12345/recordings/1/subscription.json",
          paused: false,
          schedule: {
            frequency: "every_day",
            days: [1, 2, 3, 4, 5],
            hour: 16,
            minute: 0,
          },
          answers_count: 10,
          answers_url:
            "https://3.basecampapi.com/12345/questions/1/answers.json",
        },
      ];

      server.use(
        http.get(`${BASE_URL}/questionnaires/100/questions.json`, () => {
          return HttpResponse.json(mockQuestions);
        }),
      );

      const questions = await client.checkins.listQuestions(100);

      expect(questions).toHaveLength(1);
      expect(questions[0].title).toBe("What did you work on today?");
      expect(questions[0].paused).toBe(false);
      expect(questions[0].schedule?.frequency).toBe("every_day");
    });
  });

  describe("getQuestion", () => {
    it("should get a question by ID", async () => {
      const mockQuestion = {
        id: 1,
        status: "active",
        visible_to_clients: false,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
        title: "What did you work on today?",
        inherits_status: true,
        type: "Question",
        url: "https://3.basecampapi.com/12345/questions/1.json",
        app_url: "https://3.basecamp.com/12345/questions/1",
        bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
        subscription_url:
          "https://3.basecampapi.com/12345/recordings/1/subscription.json",
        paused: false,
        schedule: {
          frequency: "every_day",
          days: [1, 2, 3, 4, 5],
          hour: 16,
          minute: 0,
        },
        answers_count: 10,
        answers_url: "https://3.basecampapi.com/12345/questions/1/answers.json",
      };

      server.use(
        http.get(`${BASE_URL}/questions/1`, () => {
          return HttpResponse.json(mockQuestion);
        }),
      );

      const question = await client.checkins.getQuestion(1);

      expect(question.id).toBe(1);
      expect(question.title).toBe("What did you work on today?");
    });
  });

  describe("createQuestion", () => {
    it("should create a new question", async () => {
      const mockQuestion = {
        id: 2,
        status: "active",
        visible_to_clients: false,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
        title: "What are your blockers?",
        inherits_status: true,
        type: "Question",
        url: "https://3.basecampapi.com/12345/questions/2.json",
        app_url: "https://3.basecamp.com/12345/questions/2",
        bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
        subscription_url:
          "https://3.basecampapi.com/12345/recordings/2/subscription.json",
        paused: false,
        schedule: {
          frequency: "every_week",
          days: [1],
          hour: 9,
          minute: 0,
        },
        answers_count: 0,
        answers_url: "https://3.basecampapi.com/12345/questions/2/answers.json",
      };

      server.use(
        http.post(
          `${BASE_URL}/questionnaires/100/questions.json`,
          async ({ request }) => {
            const body = (await request.json()) as { title: string };
            expect(body.title).toBe("What are your blockers?");
            return HttpResponse.json(mockQuestion);
          },
        ),
      );

      const question = await client.checkins.createQuestion(100, {
        title: "What are your blockers?",
        schedule: {
          frequency: "every_week",
          days: [1],
          hour: 9,
          minute: 0,
        },
      });

      expect(question.id).toBe(2);
      expect(question.title).toBe("What are your blockers?");
    });

    // Client-side validation short-circuits before any HTTP call. No MSW handler
    // is registered here, so a leaked request fails via onUnhandledRequest: "error".
    it("rejects createQuestion with a missing title or schedule", async () => {
      await expect(
        client.checkins.createQuestion(1, { title: "", schedule: SCHEDULE })
      ).rejects.toMatchObject({ code: "validation", message: "Title is required" });
      await expect(
        client.checkins.createQuestion(1, { title: "Standup", schedule: undefined as never })
      ).rejects.toMatchObject({ code: "validation", message: "Schedule is required" });
    });
  });

  describe("updateQuestion", () => {
    it("should update a question", async () => {
      const mockQuestion = {
        id: 1,
        status: "active",
        visible_to_clients: false,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-02T00:00:00Z",
        title: "What did you work on today?",
        inherits_status: true,
        type: "Question",
        url: "https://3.basecampapi.com/12345/questions/1.json",
        app_url: "https://3.basecamp.com/12345/questions/1",
        bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
        subscription_url:
          "https://3.basecampapi.com/12345/recordings/1/subscription.json",
        paused: true,
        schedule: {
          frequency: "every_day",
          days: [1, 2, 3, 4, 5],
          hour: 16,
          minute: 0,
        },
        answers_count: 10,
        answers_url: "https://3.basecampapi.com/12345/questions/1/answers.json",
      };

      server.use(
        http.put(`${BASE_URL}/questions/1`, () => {
          return HttpResponse.json(mockQuestion);
        }),
      );

      const question = await client.checkins.updateQuestion(1, {
        paused: true,
      });

      expect(question.paused).toBe(true);
    });
  });

  describe("listAnswers", () => {
    it("should list all answers for a question", async () => {
      const mockAnswers = [
        {
          id: 50,
          status: "active",
          visible_to_clients: false,
          created_at: "2024-01-01T16:00:00Z",
          updated_at: "2024-01-01T16:00:00Z",
          title: "",
          inherits_status: true,
          type: "Question::Answer",
          url: "https://3.basecampapi.com/12345/question_answers/50.json",
          app_url: "https://3.basecamp.com/12345/question_answers/50",
          bookmark_url:
            "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
          subscription_url:
            "https://3.basecampapi.com/12345/recordings/50/subscription.json",
          comments_count: 0,
          comments_url:
            "https://3.basecampapi.com/12345/recordings/50/comments.json",
          content: "<p>Worked on the new feature</p>",
          content_attachments: [],
          group_on: "2024-01-01",
          creator: { id: 999, name: "Test User" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/questions/1/answers.json`, () => {
          return HttpResponse.json(mockAnswers);
        }),
      );

      const answers = await client.checkins.listAnswers(1);

      expect(answers).toHaveLength(1);
      expect(answers[0].content).toBe("<p>Worked on the new feature</p>");
      expect(answers[0].group_on).toBe("2024-01-01");
    });
  });

  describe("getAnswer", () => {
    it("should get an answer by ID", async () => {
      const mockAnswer = {
        id: 50,
        status: "active",
        visible_to_clients: false,
        created_at: "2024-01-01T16:00:00Z",
        updated_at: "2024-01-01T16:00:00Z",
        title: "",
        inherits_status: true,
        type: "Question::Answer",
        url: "https://3.basecampapi.com/12345/question_answers/50.json",
        app_url: "https://3.basecamp.com/12345/question_answers/50",
        bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
        subscription_url:
          "https://3.basecampapi.com/12345/recordings/50/subscription.json",
        comments_count: 0,
        comments_url:
          "https://3.basecampapi.com/12345/recordings/50/comments.json",
        content: "<p>Worked on the new feature</p>",
        content_attachments: [],
        group_on: "2024-01-01",
      };

      server.use(
        http.get(`${BASE_URL}/question_answers/50`, () => {
          return HttpResponse.json(mockAnswer);
        }),
      );

      const answer = await client.checkins.getAnswer(50);

      expect(answer.id).toBe(50);
      expect(answer.content).toBe("<p>Worked on the new feature</p>");
    });
  });

  describe("createAnswer", () => {
    it("should create an answer", async () => {
      const mockAnswer = {
        id: 51,
        status: "active",
        visible_to_clients: false,
        created_at: "2024-01-02T16:00:00Z",
        updated_at: "2024-01-02T16:00:00Z",
        title: "",
        inherits_status: true,
        type: "Question::Answer",
        url: "https://3.basecampapi.com/12345/question_answers/51.json",
        app_url: "https://3.basecamp.com/12345/question_answers/51",
        bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
        subscription_url:
          "https://3.basecampapi.com/12345/recordings/51/subscription.json",
        comments_count: 0,
        comments_url:
          "https://3.basecampapi.com/12345/recordings/51/comments.json",
        content: "<p>Finished the feature!</p>",
        content_attachments: [],
        group_on: "2024-01-02",
      };

      server.use(
        http.post(
          `${BASE_URL}/questions/1/answers.json`,
          async ({ request }) => {
            const body = (await request.json()) as { content: string };
            expect(body.content).toBe("<p>Finished the feature!</p>");
            return HttpResponse.json(mockAnswer);
          },
        ),
      );

      const answer = await client.checkins.createAnswer(1, {
        content: "<p>Finished the feature!</p>",
      });

      expect(answer.id).toBe(51);
      expect(answer.content).toBe("<p>Finished the feature!</p>");
    });

    // Client-side validation short-circuits before any HTTP call. No MSW handler
    // is registered here, so a leaked request fails via onUnhandledRequest: "error".
    it("rejects createAnswer with missing content or a malformed groupOn", async () => {
      await expect(
        client.checkins.createAnswer(1, { content: "" })
      ).rejects.toMatchObject({ code: "validation", message: "Content is required" });
      await expect(
        client.checkins.createAnswer(1, { content: "Done", groupOn: "03/15/2026" })
      ).rejects.toMatchObject({ code: "validation", message: "Group on must be in YYYY-MM-DD format" });
    });
  });

  describe("updateAnswer", () => {
    it("should update an answer", async () => {
      server.use(
        http.put(`${BASE_URL}/question_answers/50`, () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      // Should not throw
      await client.checkins.updateAnswer(50, {
        content: "<p>Updated content</p>",
      });
    });

    it("should send group_on when provided", async () => {
      let receivedBody: Record<string, unknown> | null = null;

      server.use(
        http.put(`${BASE_URL}/question_answers/50`, async ({ request }) => {
          receivedBody = (await request.json()) as Record<string, unknown>;
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await client.checkins.updateAnswer(50, {
        content: "<p>Updated content</p>",
        groupOn: "2025-03-01",
      });

      expect(receivedBody).not.toBeNull();
      expect(receivedBody!.content).toBe("<p>Updated content</p>");
      expect(receivedBody!.group_on).toBe("2025-03-01");
    });

    // Client-side validation short-circuits before any HTTP call. No MSW handler
    // is registered here, so a leaked request fails via onUnhandledRequest: "error".
    it("rejects updateAnswer with missing content or a malformed groupOn", async () => {
      await expect(
        client.checkins.updateAnswer(50, { content: "" })
      ).rejects.toMatchObject({ code: "validation", message: "Content is required" });
      await expect(
        client.checkins.updateAnswer(50, { content: "Done", groupOn: "2026-3-1" })
      ).rejects.toMatchObject({ code: "validation", message: "Group on must be in YYYY-MM-DD format" });
    });
  });
});
