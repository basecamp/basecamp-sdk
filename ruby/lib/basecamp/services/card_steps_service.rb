# frozen_string_literal: true

module Basecamp
  module Services
    # Service for card step (checklist item) operations.
    #
    # Steps are checklist items on cards in card tables.
    #
    # @example Create a step
    #   step = account.card_steps.create(
    #     project_id: 123,
    #     card_id: 456,
    #     title: "Review code",
    #     due_on: "2024-12-15"
    #   )
    #
    # @example Complete a step
    #   account.card_steps.complete(project_id: 123, step_id: 789)
    class CardStepsService < BaseService
      # Creates a new step on a card.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param card_id [Integer, String] card ID
      # @param title [String] step title
      # @param due_on [String, nil] due date (YYYY-MM-DD)
      # @param assignees [Array<Integer>, nil] person IDs to assign
      # @return [Hash] created step
      def create(project_id:, card_id:, title:, due_on: nil, assignees: nil)
        body = compact_params(
          title: title,
          due_on: due_on,
          assignees: assignees
        )
        http_post(bucket_path(project_id, "/card_tables/cards/#{card_id}/steps.json"), body: body).json
      end

      # Updates an existing step.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param step_id [Integer, String] step ID
      # @param title [String, nil] new title
      # @param due_on [String, nil] new due date (YYYY-MM-DD)
      # @param assignees [Array<Integer>, nil] person IDs to assign
      # @return [Hash] updated step
      def update(project_id:, step_id:, title: nil, due_on: nil, assignees: nil)
        body = compact_params(
          title: title,
          due_on: due_on,
          assignees: assignees
        )
        http_put(bucket_path(project_id, "/card_tables/steps/#{step_id}.json"), body: body).json
      end

      # Marks a step as completed.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param step_id [Integer, String] step ID
      # @return [Hash] updated step
      def complete(project_id:, step_id:)
        http_put(bucket_path(project_id, "/card_tables/steps/#{step_id}/completions.json")).json
      end

      # Marks a step as incomplete.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param step_id [Integer, String] step ID
      # @return [Hash] updated step
      def uncomplete(project_id:, step_id:)
        http_delete(bucket_path(project_id, "/card_tables/steps/#{step_id}/completions.json")).json
      end

      # Changes the position of a step within a card.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param card_id [Integer, String] card ID
      # @param step_id [Integer, String] step ID
      # @param position [Integer] new position (0-indexed)
      # @return [void]
      def reposition(project_id:, card_id:, step_id:, position:)
        http_post(bucket_path(project_id, "/card_tables/cards/#{card_id}/positions.json"),
                  body: { source_id: step_id, position: position })
        nil
      end
    end
  end
end
