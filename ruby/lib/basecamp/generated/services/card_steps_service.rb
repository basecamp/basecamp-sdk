# frozen_string_literal: true

module Basecamp
  module Services
    # Service for CardSteps operations
    #
    # @generated from OpenAPI spec
    class CardStepsService < BaseService

      # Reposition a step within a card
      # @param project_id [Integer] project id ID
      # @param card_id [Integer] card id ID
      # @param source_id [Integer] source id
      # @param position [Integer] 0-indexed position
      # @return [void]
      def reposition(project_id:, card_id:, source_id:, position:)
        http_post(bucket_path(project_id, "/card_tables/cards/#{card_id}/positions.json"), body: compact_params(source_id: source_id, position: position))
        nil
      end

      # Create a step on a card
      # @param project_id [Integer] project id ID
      # @param card_id [Integer] card id ID
      # @param title [String] title
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param assignees [Array, nil] assignees
      # @return [Hash] response data
      def create(project_id:, card_id:, title:, due_on: nil, assignees: nil)
        http_post(bucket_path(project_id, "/card_tables/cards/#{card_id}/steps.json"), body: compact_params(title: title, due_on: due_on, assignees: assignees)).json
      end

      # Update an existing step
      # @param project_id [Integer] project id ID
      # @param step_id [Integer] step id ID
      # @param title [String, nil] title
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param assignees [Array, nil] assignees
      # @return [Hash] response data
      def update(project_id:, step_id:, title: nil, due_on: nil, assignees: nil)
        http_put(bucket_path(project_id, "/card_tables/steps/#{step_id}"), body: compact_params(title: title, due_on: due_on, assignees: assignees)).json
      end

      # Mark a step as completed
      # @param project_id [Integer] project id ID
      # @param step_id [Integer] step id ID
      # @return [Hash] response data
      def complete(project_id:, step_id:)
        http_put(bucket_path(project_id, "/card_tables/steps/#{step_id}/completions.json")).json
      end

      # Mark a step as incomplete
      # @param project_id [Integer] project id ID
      # @param step_id [Integer] step id ID
      # @return [Hash] response data
      def uncomplete(project_id:, step_id:)
        http_delete(bucket_path(project_id, "/card_tables/steps/#{step_id}/completions.json")).json
      end
    end
  end
end
