# frozen_string_literal: true

module Basecamp
  module Services
    # Service for timesheet report operations.
    #
    # Provides access to timesheet reports at account, project, and recording levels.
    # Supports filtering by date range and person.
    #
    # @example Get account-wide timesheet
    #   result = account.timesheet.report
    #   result["entries"].each { |e| puts e["hours"] }
    #
    # @example Get project timesheet with filters
    #   result = account.timesheet.project_report(
    #     project_id: 123,
    #     from: "2024-01-01",
    #     to: "2024-01-31"
    #   )
    #
    # @example Get timesheet for a specific todo
    #   result = account.timesheet.recording_report(
    #     project_id: 123,
    #     recording_id: 789
    #   )
    class TimesheetService < BaseService
      # Returns the account-wide timesheet report.
      # This includes time entries across all projects in the account.
      #
      # @param from [String, nil] filter entries on or after this date (ISO 8601, e.g., "2024-01-01")
      # @param to [String, nil] filter entries on or before this date (ISO 8601, e.g., "2024-01-31")
      # @param person_id [Integer, nil] filter entries by a specific person
      # @return [Hash] timesheet report object with "entries" array
      def report(from: nil, to: nil, person_id: nil)
        params = compact_params(from: from, to: to, person_id: person_id)
        response = http_get("/reports/timesheet.json", params: params)
        response.json
      end

      # Returns the timesheet report for a specific project.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param from [String, nil] filter entries on or after this date (ISO 8601)
      # @param to [String, nil] filter entries on or before this date (ISO 8601)
      # @param person_id [Integer, nil] filter entries by a specific person
      # @return [Hash] timesheet report object with "entries" array
      def project_report(project_id:, from: nil, to: nil, person_id: nil)
        params = compact_params(from: from, to: to, person_id: person_id)
        response = http_get(bucket_path(project_id, "/timesheet.json"), params: params)
        response.json
      end

      # Returns the timesheet report for a specific recording within a project.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID (e.g., a todo)
      # @param from [String, nil] filter entries on or after this date (ISO 8601)
      # @param to [String, nil] filter entries on or before this date (ISO 8601)
      # @param person_id [Integer, nil] filter entries by a specific person
      # @return [Hash] timesheet report object with "entries" array
      def recording_report(project_id:, recording_id:, from: nil, to: nil, person_id: nil)
        params = compact_params(from: from, to: to, person_id: person_id)
        response = http_get(bucket_path(project_id, "/recordings/#{recording_id}/timesheet.json"), params: params)
        response.json
      end
    end
  end
end
