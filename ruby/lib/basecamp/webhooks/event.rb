# frozen_string_literal: true

module Basecamp
  module Webhooks
    # Structured wrapper around webhook event payloads.
    # Accepts any hash - does not reject unknown fields or event kinds.
    class Event
      attr_reader :id, :kind, :details, :created_at, :recording, :creator, :copy, :raw

      def initialize(hash)
        @raw = hash
        @id = hash["id"]
        @kind = hash["kind"]
        @details = hash["details"] || {}
        @created_at = hash["created_at"]
        @recording = hash["recording"] || {}
        @creator = hash["creator"] || {}
        @copy = hash["copy"]
      end

      # Parse "todo_created" -> { type: "todo", action: "created" }
      def parsed_kind
        return { type: kind, action: "" } unless kind&.include?("_")
        last_underscore = kind.rindex("_")
        {
          type: kind[0...last_underscore],
          action: kind[(last_underscore + 1)..]
        }
      end
    end

    # Known webhook recording types (convenience constants, not exhaustive).
    module RecordingType
      CHECKIN_REPLY = "Checkin::Reply"
      CLOUD_FILE = "CloudFile"
      COMMENT = "Comment"
      DOCUMENT = "Document"
      FORWARD_REPLY = "Forward::Reply"
      GOOGLE_DOCUMENT = "GoogleDocument"
      INBOX_FORWARD = "Inbox::Forward"
      MESSAGE = "Message"
      QUESTION = "Question"
      QUESTION_ANSWER = "Question::Answer"
      SCHEDULE_ENTRY = "Schedule::Entry"
      TODO = "Todo"
      TODOLIST = "Todolist"
      TODOLIST_GROUP = "Todolist::Group"
      UPLOAD = "Upload"
      VAULT = "Vault"
    end
  end
end
