# frozen_string_literal: true

module Basecamp
  module Services
    # Service for CardColumns operations
    #
    # @generated from OpenAPI spec
    class CardColumnsService < BaseService

      # Get a card column by ID
      # @param column_id [Integer] column id ID
      # @return [Hash] response data
      def get(column_id:)
        http_get("/card_tables/columns/#{column_id}").json
      end

      # Update an existing column
      # @param column_id [Integer] column id ID
      # @param title [String, nil] title
      # @param description [String, nil] description
      # @return [Hash] response data
      def update(column_id:, title: nil, description: nil)
        http_put("/card_tables/columns/#{column_id}", body: compact_params(title: title, description: description)).json
      end

      # Set the color of a column
      # @param column_id [Integer] column id ID
      # @param color [String] Valid colors: white, red, orange, yellow, green, blue, aqua, purple, gray, pink, brown
      # @return [Hash] response data
      def set_color(column_id:, color:)
        http_put("/card_tables/columns/#{column_id}/color.json", body: compact_params(color: color)).json
      end

      # Enable on-hold section in a column
      # @param column_id [Integer] column id ID
      # @return [Hash] response data
      def enable_on_hold(column_id:)
        http_post("/card_tables/columns/#{column_id}/on_hold.json").json
      end

      # Disable on-hold section in a column
      # @param column_id [Integer] column id ID
      # @return [Hash] response data
      def disable_on_hold(column_id:)
        http_delete("/card_tables/columns/#{column_id}/on_hold.json").json
      end

      # Subscribe to a card column (watch for changes)
      # @param column_id [Integer] column id ID
      # @return [void]
      def subscribe_to_column(column_id:)
        http_post("/card_tables/lists/#{column_id}/subscription.json")
        nil
      end

      # Unsubscribe from a card column (stop watching for changes)
      # @param column_id [Integer] column id ID
      # @return [void]
      def unsubscribe_from_column(column_id:)
        http_delete("/card_tables/lists/#{column_id}/subscription.json")
        nil
      end

      # Create a column in a card table
      # @param card_table_id [Integer] card table id ID
      # @param title [String] title
      # @param description [String, nil] description
      # @return [Hash] response data
      def create(card_table_id:, title:, description: nil)
        http_post("/card_tables/#{card_table_id}/columns.json", body: compact_params(title: title, description: description)).json
      end

      # Move a column within a card table
      # @param card_table_id [Integer] card table id ID
      # @param source_id [Integer] source id
      # @param target_id [Integer] target id
      # @param position [Integer, nil] position
      # @return [void]
      def move(card_table_id:, source_id:, target_id:, position: nil)
        http_post("/card_tables/#{card_table_id}/moves.json", body: compact_params(source_id: source_id, target_id: target_id, position: position))
        nil
      end
    end
  end
end
