# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Cards operations
    #
    # @generated from OpenAPI spec
    class CardsService < BaseService

      # Get a card by ID
      # @param project_id [Integer] project id ID
      # @param card_id [Integer] card id ID
      # @return [Hash] response data
      def get(project_id:, card_id:)
        http_get(bucket_path(project_id, "/card_tables/cards/#{card_id}")).json
      end

      # Update an existing card
      # @param project_id [Integer] project id ID
      # @param card_id [Integer] card id ID
      # @param title [String, nil] title
      # @param content [String, nil] content
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param assignee_ids [Array, nil] assignee ids
      # @return [Hash] response data
      def update(project_id:, card_id:, title: nil, content: nil, due_on: nil, assignee_ids: nil)
        http_put(bucket_path(project_id, "/card_tables/cards/#{card_id}"), body: compact_params(title: title, content: content, due_on: due_on, assignee_ids: assignee_ids)).json
      end

      # Move a card to a different column
      # @param project_id [Integer] project id ID
      # @param card_id [Integer] card id ID
      # @param column_id [Integer] column id
      # @return [void]
      def move(project_id:, card_id:, column_id:)
        http_post(bucket_path(project_id, "/card_tables/cards/#{card_id}/moves.json"), body: compact_params(column_id: column_id))
        nil
      end

      # List cards in a column
      # @param project_id [Integer] project id ID
      # @param column_id [Integer] column id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:, column_id:)
        paginate(bucket_path(project_id, "/card_tables/lists/#{column_id}/cards.json"))
      end

      # Create a card in a column
      # @param project_id [Integer] project id ID
      # @param column_id [Integer] column id ID
      # @param title [String] title
      # @param content [String, nil] content
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param notify [Boolean, nil] notify
      # @return [Hash] response data
      def create(project_id:, column_id:, title:, content: nil, due_on: nil, notify: nil)
        http_post(bucket_path(project_id, "/card_tables/lists/#{column_id}/cards.json"), body: compact_params(title: title, content: content, due_on: due_on, notify: notify)).json
      end
    end
  end
end
