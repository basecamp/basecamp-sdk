import { describe, it, expect } from "vitest";
import { parseEventKind, WebhookEventKind } from "../../src/webhooks/events.js";

describe("parseEventKind", () => {
  it("parses simple event kinds", () => {
    expect(parseEventKind("todo_created")).toEqual({ type: "todo", action: "created" });
    expect(parseEventKind("todo_completed")).toEqual({ type: "todo", action: "completed" });
    expect(parseEventKind("message_created")).toEqual({ type: "message", action: "created" });
  });

  it("parses compound types (last underscore is the split point)", () => {
    expect(parseEventKind("question_answer_created")).toEqual({
      type: "question_answer",
      action: "created",
    });
    expect(parseEventKind("schedule_entry_changed")).toEqual({
      type: "schedule_entry",
      action: "changed",
    });
    expect(parseEventKind("cloud_file_created")).toEqual({
      type: "cloud_file",
      action: "created",
    });
  });

  it("handles kinds without underscore", () => {
    expect(parseEventKind("ping")).toEqual({ type: "ping", action: "" });
  });

  it("handles unknown future kinds gracefully", () => {
    expect(parseEventKind("new_thing_activated")).toEqual({
      type: "new_thing",
      action: "activated",
    });
  });
});

describe("WebhookEventKind", () => {
  it("has expected constant values", () => {
    expect(WebhookEventKind.TodoCreated).toBe("todo_created");
    expect(WebhookEventKind.MessageCopied).toBe("message_copied");
    expect(WebhookEventKind.QuestionAnswerCreated).toBe("question_answer_created");
  });
});
