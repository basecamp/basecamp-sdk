# frozen_string_literal: true

module Basecamp
  module Services
    # Service for CardTables operations
    #
    # @generated from OpenAPI spec
    class CardTablesService < BaseService

      # Get a card table by ID
      def get(project_id:, card_table_id:)
        http_get(bucket_path(project_id, "/card_tables/#{card_table_id}")).json
      end
    end
  end
end
