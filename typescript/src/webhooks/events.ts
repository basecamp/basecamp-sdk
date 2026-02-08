import type { components } from "../generated/schema.js";

// Re-export generated types for convenience
export type WebhookEvent = components["schemas"]["WebhookEvent"];
export type WebhookDelivery = components["schemas"]["WebhookDelivery"];
export type WebhookDeliveryRequest = components["schemas"]["WebhookDeliveryRequest"];
export type WebhookDeliveryResponse = components["schemas"]["WebhookDeliveryResponse"];
export type WebhookCopy = components["schemas"]["WebhookCopy"];

/**
 * Parse "todo_created" → { type: "todo", action: "created" }.
 * For compound types like "question_answer_created" → { type: "question_answer", action: "created" }.
 * The action is always the last underscore-separated segment.
 */
export function parseEventKind(kind: string): { type: string; action: string } {
  const lastUnderscore = kind.lastIndexOf("_");
  if (lastUnderscore === -1) {
    return { type: kind, action: "" };
  }
  return {
    type: kind.slice(0, lastUnderscore),
    action: kind.slice(lastUnderscore + 1),
  };
}

/** Known webhook event kind strings (convenience constants, not exhaustive). */
export const WebhookEventKind = {
  TodoCreated: "todo_created",
  TodoCompleted: "todo_completed",
  TodoUncompleted: "todo_uncompleted",
  TodoChanged: "todo_changed",
  TodolistCreated: "todolist_created",
  TodolistChanged: "todolist_changed",
  MessageCreated: "message_created",
  MessageChanged: "message_changed",
  CommentCreated: "comment_created",
  CommentChanged: "comment_changed",
  DocumentCreated: "document_created",
  DocumentChanged: "document_changed",
  UploadCreated: "upload_created",
  UploadChanged: "upload_changed",
  QuestionAnswerCreated: "question_answer_created",
  QuestionAnswerChanged: "question_answer_changed",
  ScheduleEntryCreated: "schedule_entry_created",
  ScheduleEntryChanged: "schedule_entry_changed",
  CloudFileCreated: "cloud_file_created",
  VaultCreated: "vault_created",
  InboxForwardCreated: "inbox_forward_created",
  ForwardReplyCreated: "forward_reply_created",
  ClientReplyCreated: "client_reply_created",
  ClientApprovalCreated: "client_approval_created",
  ClientApprovalChanged: "client_approval_changed",
  TodoCopied: "todo_copied",
  MessageCopied: "message_copied",
  TodoArchived: "todo_archived",
  TodoUnarchived: "todo_unarchived",
  TodoTrashed: "todo_trashed",
  MessageArchived: "message_archived",
  MessageUnarchived: "message_unarchived",
  MessageTrashed: "message_trashed",
} as const;
