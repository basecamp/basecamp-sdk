from __future__ import annotations


class WebhookEventKind:
    """Known webhook event kind strings (convenience constants, not exhaustive)."""

    TODO_CREATED = "todo_created"
    TODO_COMPLETED = "todo_completed"
    TODO_UNCOMPLETED = "todo_uncompleted"
    TODO_CHANGED = "todo_changed"
    TODOLIST_CREATED = "todolist_created"
    TODOLIST_CHANGED = "todolist_changed"
    MESSAGE_CREATED = "message_created"
    MESSAGE_CHANGED = "message_changed"
    COMMENT_CREATED = "comment_created"
    COMMENT_CHANGED = "comment_changed"
    DOCUMENT_CREATED = "document_created"
    DOCUMENT_CHANGED = "document_changed"
    UPLOAD_CREATED = "upload_created"
    UPLOAD_CHANGED = "upload_changed"
    QUESTION_ANSWER_CREATED = "question_answer_created"
    QUESTION_ANSWER_CHANGED = "question_answer_changed"
    SCHEDULE_ENTRY_CREATED = "schedule_entry_created"
    SCHEDULE_ENTRY_CHANGED = "schedule_entry_changed"
    CLOUD_FILE_CREATED = "cloud_file_created"
    VAULT_CREATED = "vault_created"
    INBOX_FORWARD_CREATED = "inbox_forward_created"
    FORWARD_REPLY_CREATED = "forward_reply_created"
    CLIENT_REPLY_CREATED = "client_reply_created"
    CLIENT_APPROVAL_CREATED = "client_approval_created"
    CLIENT_APPROVAL_CHANGED = "client_approval_changed"
    TODO_COPIED = "todo_copied"
    MESSAGE_COPIED = "message_copied"
    TODO_ARCHIVED = "todo_archived"
    TODO_UNARCHIVED = "todo_unarchived"
    TODO_TRASHED = "todo_trashed"
    MESSAGE_ARCHIVED = "message_archived"
    MESSAGE_UNARCHIVED = "message_unarchived"
    MESSAGE_TRASHED = "message_trashed"


def parse_event_kind(kind: str) -> tuple[str, str]:
    """Parse a webhook event kind into (type, action).

    Splits at the last underscore: "todo_created" -> ("todo", "created").
    Compound types split correctly: "question_answer_created" -> ("question_answer", "created").
    Returns (kind, "") if there is no underscore.
    """
    last_underscore = kind.rfind("_")
    if last_underscore == -1:
        return (kind, "")
    return (kind[:last_underscore], kind[last_underscore + 1 :])
