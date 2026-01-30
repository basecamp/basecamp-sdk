# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientVisibility operations
    #
    # @generated from OpenAPI spec
    class ClientVisibilityService < BaseService

      # Set client visibility for a recording
      def set_visibility(project_id:, recording_id:, **body)
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/client_visibility.json"), body: body).json
      end
    end
  end
end
