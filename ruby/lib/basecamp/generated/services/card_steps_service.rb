# frozen_string_literal: true

module Basecamp
  module Services
    # Service for CardSteps operations
    #
    # @generated from OpenAPI spec
    class CardStepsService < BaseService

      # Reposition a step within a card
      # @param card_id [Integer] card id ID
      # @param source_id [Integer] source id
      # @param position [Integer] 0-indexed position
      # @return [void]
      def reposition(card_id:, source_id:, position:)
        http_post("/card_tables/cards/#{card_id}/positions.json", body: compact_params(source_id: source_id, position: position))
        nil
      end

      # Create a step on a card
      # @param card_id [Integer] card id ID
      # @param title [String] title
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param assignees [Array, nil] assignees
      # @return [Hash] response data
      def create(card_id:, title:, due_on: nil, assignees: nil)
        http_post("/card_tables/cards/#{card_id}/steps.json", body: compact_params(title: title, due_on: due_on, assignees: assignees)).json
      end

      # Update an existing step
      # @param step_id [Integer] step id ID
      # @param title [String, nil] title
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param assignees [Array, nil] assignees
      # @return [Hash] response data
      def update(step_id:, title: nil, due_on: nil, assignees: nil)
        http_put("/card_tables/steps/#{step_id}", body: compact_params(title: title, due_on: due_on, assignees: assignees)).json
      end

      # Set card step completion status (PUT with completion: "on" to complete, "" to uncomplete)
      # @param step_id [Integer] step id ID
      # @param completion [String] Set to "on" to complete the step, "" (empty) to uncomplete
      # @return [Hash] response data
      def set_completion(step_id:, completion:)
        http_put("/card_tables/steps/#{step_id}/completions.json", body: compact_params(completion: completion)).json
      end
    end
  end
end
