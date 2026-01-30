# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Cards operations
    #
    # @generated from OpenAPI spec
    class CardsService < BaseService

      # Get a card by ID
      def get(project_id:, card_id:)
        http_get(bucket_path(project_id, "/card_tables/cards/#{card_id}")).json
      end

      # Update an existing card
      def update(project_id:, card_id:, **body)
        http_put(bucket_path(project_id, "/card_tables/cards/#{card_id}"), body: body).json
      end

      # Move a card to a different column
      def move(project_id:, card_id:, **body)
        http_post(bucket_path(project_id, "/card_tables/cards/#{card_id}/moves.json"), body: body)
        nil
      end

      # List cards in a column
      def list(project_id:, column_id:)
        paginate(bucket_path(project_id, "/card_tables/lists/#{column_id}/cards.json"))
      end

      # Create a card in a column
      def create(project_id:, column_id:, **body)
        http_post(bucket_path(project_id, "/card_tables/lists/#{column_id}/cards.json"), body: body).json
      end
    end
  end
end
