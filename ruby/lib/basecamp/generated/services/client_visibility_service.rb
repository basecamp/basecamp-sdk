# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientVisibility operations
    #
    # @generated from OpenAPI spec
    class ClientVisibilityService < BaseService

      # Set client visibility for a recording
      # @param recording_id [Integer] recording id ID
      # @param visible_to_clients [Boolean] visible to clients
      # @return [Hash] response data
      def set_visibility(recording_id:, visible_to_clients:)
        with_operation(service: "clientvisibility", operation: "set_visibility", is_mutation: true, resource_id: recording_id) do
          http_put("/recordings/#{recording_id}/client_visibility.json", body: compact_params(visible_to_clients: visible_to_clients)).json
        end
      end
    end
  end
end
