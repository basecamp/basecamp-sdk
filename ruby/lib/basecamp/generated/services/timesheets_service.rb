# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Timesheets operations
    #
    # @generated from OpenAPI spec
    class TimesheetsService < BaseService

      # Get timesheet for a specific recording
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @param from [String, nil] from
      # @param to [String, nil] to
      # @param person_id [Integer, nil] person id
      # @return [Hash] response data
      def for_recording(project_id:, recording_id:, from: nil, to: nil, person_id: nil)
        http_get(bucket_path(project_id, "/recordings/#{recording_id}/timesheet.json"), params: compact_params(from: from, to: to, person_id: person_id)).json
      end

      # Get timesheet for a specific project
      # @param project_id [Integer] project id ID
      # @param from [String, nil] from
      # @param to [String, nil] to
      # @param person_id [Integer, nil] person id
      # @return [Hash] response data
      def for_project(project_id:, from: nil, to: nil, person_id: nil)
        http_get(bucket_path(project_id, "/timesheet.json"), params: compact_params(from: from, to: to, person_id: person_id)).json
      end

      # Get account-wide timesheet report
      # @param from [String, nil] from
      # @param to [String, nil] to
      # @param person_id [Integer, nil] person id
      # @return [Hash] response data
      def report(from: nil, to: nil, person_id: nil)
        http_get("/reports/timesheet.json", params: compact_params(from: from, to: to, person_id: person_id)).json
      end
    end
  end
end
