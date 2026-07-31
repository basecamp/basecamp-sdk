# frozen_string_literal: true

module Basecamp
  module Services
    # Service for MyNotes operations
    #
    # @generated from OpenAPI spec
    class MyNotesService < BaseService

      # Get the authenticated user's note — a per-person notebook singleton at
      # @return [Hash] response data
      def get_my_note()
        with_operation(service: "mynotes", operation: "get_my_note", is_mutation: false) do
          http_get("/my/notes.json", operation: "GetMyNote").json
        end
      end

      # Replace the note's content, recording a new revision server-side. The first
      # @param note [Hash] note
      # @return [Hash] response data
      def update_my_note(note:)
        with_operation(service: "mynotes", operation: "update_my_note", is_mutation: true) do
          http_put("/my/notes.json", body: compact_params(note: note)).json
        end
      end
    end
  end
end
