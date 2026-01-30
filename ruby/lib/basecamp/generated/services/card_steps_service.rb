# frozen_string_literal: true

module Basecamp
  module Services
    # Service for CardSteps operations
    #
    # @generated from OpenAPI spec
    class CardStepsService < BaseService

      # Reposition a step within a card
      def reposition(project_id:, card_id:, **body)
        http_post(bucket_path(project_id, "/card_tables/cards/#{card_id}/positions.json"), body: body)
        nil
      end

      # Create a step on a card
      def create(project_id:, card_id:, **body)
        http_post(bucket_path(project_id, "/card_tables/cards/#{card_id}/steps.json"), body: body).json
      end

      # Update an existing step
      def update(project_id:, step_id:, **body)
        http_put(bucket_path(project_id, "/card_tables/steps/#{step_id}"), body: body).json
      end

      # Mark a step as completed
      def complete(project_id:, step_id:)
        http_put(bucket_path(project_id, "/card_tables/steps/#{step_id}/completions.json")).json
      end

      # Mark a step as incomplete
      def uncomplete(project_id:, step_id:)
        http_delete(bucket_path(project_id, "/card_tables/steps/#{step_id}/completions.json")).json
      end
    end
  end
end
