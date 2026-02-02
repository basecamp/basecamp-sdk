# frozen_string_literal: true

module Basecamp
  module Services
    # Service for CardTables operations
    #
    # @generated from OpenAPI spec
    class CardTablesService < BaseService

      # Get a card table by ID
      # @param card_table_id [Integer] card table id ID
      # @return [Hash] response data
      def get(card_table_id:)
        with_operation(service: "cardtables", operation: "get", is_mutation: false, project_id: project_id, resource_id: card_table_id) do
          http_get("/card_tables/#{card_table_id}").json
        end
      end
    end
  end
end
