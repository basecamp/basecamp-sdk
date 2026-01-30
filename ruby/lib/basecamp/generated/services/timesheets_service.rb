# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Timesheets operations
    #
    # @generated from OpenAPI spec
    class TimesheetsService < BaseService

      # Get timesheet for a specific recording
      def for_recording(project_id:, recording_id:, from: nil, to: nil, person_id: nil)
        http_get(bucket_path(project_id, "/recordings/#{recording_id}/timesheet.json"), params: compact_params(from: from, to: to, person_id: person_id)).json
      end

      # Get timesheet for a specific project
      def for_project(project_id:, from: nil, to: nil, person_id: nil)
        http_get(bucket_path(project_id, "/timesheet.json"), params: compact_params(from: from, to: to, person_id: person_id)).json
      end

      # Get account-wide timesheet report
      def report(from: nil, to: nil, person_id: nil)
        http_get("/reports/timesheet.json", params: compact_params(from: from, to: to, person_id: person_id)).json
      end
    end
  end
end
