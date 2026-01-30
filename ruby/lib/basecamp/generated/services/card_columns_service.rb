# frozen_string_literal: true

module Basecamp
  module Services
    # Service for CardColumns operations
    #
    # @generated from OpenAPI spec
    class CardColumnsService < BaseService

      # Get a card column by ID
      def get(project_id:, column_id:)
        http_get(bucket_path(project_id, "/card_tables/columns/#{column_id}")).json
      end

      # Update an existing column
      def update(project_id:, column_id:, **body)
        http_put(bucket_path(project_id, "/card_tables/columns/#{column_id}"), body: body).json
      end

      # Set the color of a column
      def set_color(project_id:, column_id:, **body)
        http_put(bucket_path(project_id, "/card_tables/columns/#{column_id}/color.json"), body: body).json
      end

      # Enable on-hold section in a column
      def enable_on_hold(project_id:, column_id:)
        http_post(bucket_path(project_id, "/card_tables/columns/#{column_id}/on_hold.json")).json
      end

      # Disable on-hold section in a column
      def disable_on_hold(project_id:, column_id:)
        http_delete(bucket_path(project_id, "/card_tables/columns/#{column_id}/on_hold.json")).json
      end

      # Subscribe to a card column (watch for changes)
      def subscribe_to_column(project_id:, column_id:)
        http_post(bucket_path(project_id, "/card_tables/lists/#{column_id}/subscription.json"))
        nil
      end

      # Unsubscribe from a card column (stop watching for changes)
      def unsubscribe_from_column(project_id:, column_id:)
        http_delete(bucket_path(project_id, "/card_tables/lists/#{column_id}/subscription.json"))
        nil
      end

      # Create a column in a card table
      def create(project_id:, card_table_id:, **body)
        http_post(bucket_path(project_id, "/card_tables/#{card_table_id}/columns.json"), body: body).json
      end

      # Move a column within a card table
      def move(project_id:, card_table_id:, **body)
        http_post(bucket_path(project_id, "/card_tables/#{card_table_id}/moves.json"), body: body)
        nil
      end
    end
  end
end
