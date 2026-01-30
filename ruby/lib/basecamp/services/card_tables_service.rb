# frozen_string_literal: true

module Basecamp
  module Services
    # Service for card table (kanban board) operations.
    #
    # Card Tables are kanban-style boards with columns containing cards.
    #
    # @example Get a card table
    #   table = account.card_tables.get(project_id: 123, card_table_id: 456)
    #   puts "#{table["title"]} - #{table["lists"].length} columns"
    class CardTablesService < BaseService
      # Gets a card table by ID.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param card_table_id [Integer, String] card table ID
      # @return [Hash] card table with its columns
      def get(project_id:, card_table_id:)
        http_get(bucket_path(project_id, "/card_tables/#{card_table_id}.json")).json
      end
    end
  end
end
