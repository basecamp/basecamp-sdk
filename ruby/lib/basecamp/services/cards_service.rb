# frozen_string_literal: true

module Basecamp
  module Services
    # Service for card operations within card tables.
    #
    # Cards are items in card table columns. They can have steps (checklist items),
    # assignees, and due dates.
    #
    # @example List cards in a column
    #   account.cards.list(project_id: 123, column_id: 456).each do |card|
    #     puts "#{card["title"]} - #{card["completed"] ? "done" : "pending"}"
    #   end
    #
    # @example Create a card
    #   card = account.cards.create(
    #     project_id: 123,
    #     column_id: 456,
    #     title: "New Feature",
    #     content: "<p>Feature description</p>",
    #     due_on: "2024-12-31"
    #   )
    class CardsService < BaseService
      # Lists all cards in a column.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param column_id [Integer, String] column ID
      # @return [Enumerator<Hash>] cards
      def list(project_id:, column_id:)
        paginate(bucket_path(project_id, "/card_tables/lists/#{column_id}/cards.json"))
      end

      # Gets a card by ID.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param card_id [Integer, String] card ID
      # @return [Hash] card data
      def get(project_id:, card_id:)
        http_get(bucket_path(project_id, "/card_tables/cards/#{card_id}.json")).json
      end

      # Creates a new card in a column.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param column_id [Integer, String] column ID
      # @param title [String] card title
      # @param content [String, nil] card body in HTML
      # @param due_on [String, nil] due date (YYYY-MM-DD)
      # @param notify [Boolean, nil] notify assignees
      # @return [Hash] created card
      def create(project_id:, column_id:, title:, content: nil, due_on: nil, notify: nil)
        body = compact_params(
          title: title,
          content: content,
          due_on: due_on,
          notify: notify
        )
        http_post(bucket_path(project_id, "/card_tables/lists/#{column_id}/cards.json"), body: body).json
      end

      # Updates an existing card.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param card_id [Integer, String] card ID
      # @param title [String, nil] new title
      # @param content [String, nil] new content
      # @param due_on [String, nil] new due date (YYYY-MM-DD)
      # @param assignee_ids [Array<Integer>, nil] person IDs to assign
      # @return [Hash] updated card
      def update(project_id:, card_id:, title: nil, content: nil, due_on: nil, assignee_ids: nil)
        body = compact_params(
          title: title,
          content: content,
          due_on: due_on,
          assignee_ids: assignee_ids
        )
        http_put(bucket_path(project_id, "/card_tables/cards/#{card_id}.json"), body: body).json
      end

      # Moves a card to a different column.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param card_id [Integer, String] card ID
      # @param column_id [Integer, String] destination column ID
      # @return [void]
      def move(project_id:, card_id:, column_id:)
        http_post(bucket_path(project_id, "/card_tables/cards/#{card_id}/moves.json"),
                  body: { column_id: column_id })
        nil
      end
    end
  end
end
