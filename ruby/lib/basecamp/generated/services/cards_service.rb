# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Cards operations
    #
    # @generated from OpenAPI spec
    class CardsService < BaseService

      # Get a card by ID
      # @param card_id [Integer] card id ID
      # @return [Hash] response data
      def get(card_id:)
        with_operation(service: "cards", operation: "get", is_mutation: false, project_id: project_id, resource_id: card_id) do
          http_get("/card_tables/cards/#{card_id}").json
        end
      end

      # Update an existing card
      # @param card_id [Integer] card id ID
      # @param title [String, nil] title
      # @param content [String, nil] content
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param assignee_ids [Array, nil] assignee ids
      # @return [Hash] response data
      def update(card_id:, title: nil, content: nil, due_on: nil, assignee_ids: nil)
        with_operation(service: "cards", operation: "update", is_mutation: true, project_id: project_id, resource_id: card_id) do
          http_put("/card_tables/cards/#{card_id}", body: compact_params(title: title, content: content, due_on: due_on, assignee_ids: assignee_ids)).json
        end
      end

      # Move a card to a different column
      # @param card_id [Integer] card id ID
      # @param column_id [Integer] column id
      # @return [void]
      def move(card_id:, column_id:)
        with_operation(service: "cards", operation: "move", is_mutation: true, project_id: project_id, resource_id: card_id) do
          http_post("/card_tables/cards/#{card_id}/moves.json", body: compact_params(column_id: column_id))
          nil
        end
      end

      # List cards in a column
      # @param column_id [Integer] column id ID
      # @return [Enumerator<Hash>] paginated results
      def list(column_id:)
        wrap_paginated(service: "cards", operation: "list", is_mutation: false, project_id: project_id, resource_id: column_id) do
          paginate("/card_tables/lists/#{column_id}/cards.json")
        end
      end

      # Create a card in a column
      # @param column_id [Integer] column id ID
      # @param title [String] title
      # @param content [String, nil] content
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param notify [Boolean, nil] notify
      # @return [Hash] response data
      def create(column_id:, title:, content: nil, due_on: nil, notify: nil)
        with_operation(service: "cards", operation: "create", is_mutation: true, project_id: project_id, resource_id: column_id) do
          http_post("/card_tables/lists/#{column_id}/cards.json", body: compact_params(title: title, content: content, due_on: due_on, notify: notify)).json
        end
      end
    end
  end
end
