# frozen_string_literal: true

module Basecamp
  module Services
    # Service for card column operations.
    #
    # Columns are lists within a card table that contain cards.
    #
    # @example Create a column
    #   column = account.card_columns.create(
    #     project_id: 123,
    #     card_table_id: 456,
    #     title: "In Review"
    #   )
    #
    # @example Set column color
    #   account.card_columns.set_color(project_id: 123, column_id: 456, color: "blue")
    class CardColumnsService < BaseService
      # Gets a column by ID.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param column_id [Integer, String] column ID
      # @return [Hash] column data
      def get(project_id:, column_id:)
        http_get(bucket_path(project_id, "/card_tables/columns/#{column_id}.json")).json
      end

      # Creates a new column in a card table.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param card_table_id [Integer, String] card table ID
      # @param title [String] column title
      # @param description [String, nil] column description
      # @return [Hash] created column
      def create(project_id:, card_table_id:, title:, description: nil)
        body = compact_params(
          title: title,
          description: description
        )
        http_post(bucket_path(project_id, "/card_tables/#{card_table_id}/columns.json"), body: body).json
      end

      # Updates an existing column.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param column_id [Integer, String] column ID
      # @param title [String, nil] new title
      # @param description [String, nil] new description
      # @return [Hash] updated column
      def update(project_id:, column_id:, title: nil, description: nil)
        body = compact_params(
          title: title,
          description: description
        )
        http_put(bucket_path(project_id, "/card_tables/columns/#{column_id}.json"), body: body).json
      end

      # Moves a column within a card table.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param card_table_id [Integer, String] card table ID
      # @param source_id [Integer, String] column ID to move
      # @param target_id [Integer, String] column ID to move relative to
      # @param position [Integer, nil] position relative to target
      # @return [void]
      def move(project_id:, card_table_id:, source_id:, target_id:, position: nil)
        body = compact_params(
          source_id: source_id,
          target_id: target_id,
          position: position
        )
        http_post(bucket_path(project_id, "/card_tables/#{card_table_id}/moves.json"), body: body)
        nil
      end

      # Sets the color of a column.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param column_id [Integer, String] column ID
      # @param color [String] color name (white, red, orange, yellow, green, blue, aqua, purple, gray, pink, brown)
      # @return [Hash] updated column
      def set_color(project_id:, column_id:, color:)
        http_put(bucket_path(project_id, "/card_tables/columns/#{column_id}/color.json"),
                 body: { color: color }).json
      end

      # Adds an on-hold section to a column.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param column_id [Integer, String] column ID
      # @return [Hash] updated column
      def enable_on_hold(project_id:, column_id:)
        http_post(bucket_path(project_id, "/card_tables/columns/#{column_id}/on_hold.json")).json
      end

      # Removes the on-hold section from a column.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param column_id [Integer, String] column ID
      # @return [Hash] updated column
      def disable_on_hold(project_id:, column_id:)
        http_delete(bucket_path(project_id, "/card_tables/columns/#{column_id}/on_hold.json")).json
      end
    end
  end
end
