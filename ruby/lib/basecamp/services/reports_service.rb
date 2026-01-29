# frozen_string_literal: true

module Basecamp
  module Services
    # Service for report operations.
    #
    # Provides access to various report types including timesheet and assigned todos reports.
    #
    # @example Get account-wide timesheet
    #   result = account.reports.timesheet
    #   result["entries"].each { |e| puts e["hours"] }
    #
    # @example Get list of people with assigned todos
    #   people = account.reports.assignable_people.to_a
    #
    # @example Get todos assigned to a specific person
    #   result = account.reports.assigned_todos(person_id: 123)
    #   result["todos"].each { |todo| puts todo["content"] }
    class ReportsService < BaseService
      # Returns the account-wide timesheet report.
      # This includes time entries across all projects in the account.
      #
      # @param from [String, nil] filter entries on or after this date (ISO 8601)
      # @param to [String, nil] filter entries on or before this date (ISO 8601)
      # @param person_id [Integer, nil] filter entries by a specific person
      # @return [Hash] timesheet report object with "entries" array
      def timesheet(from: nil, to: nil, person_id: nil)
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
      def project_timesheet(project_id:, from: nil, to: nil, person_id: nil)
        params = compact_params(from: from, to: to, person_id: person_id)
        response = http_get(bucket_path(project_id, "/timesheet.json"), params: params)
        response.json
      end

      # Returns the timesheet report for a specific recording within a project.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID (e.g., a todo or message)
      # @param from [String, nil] filter entries on or after this date (ISO 8601)
      # @param to [String, nil] filter entries on or before this date (ISO 8601)
      # @param person_id [Integer, nil] filter entries by a specific person
      # @return [Hash] timesheet report object with "entries" array
      def recording_timesheet(project_id:, recording_id:, from: nil, to: nil, person_id: nil)
        params = compact_params(from: from, to: to, person_id: person_id)
        response = http_get(bucket_path(project_id, "/recordings/#{recording_id}/timesheet.json"), params: params)
        response.json
      end

      # Returns the list of people who have assigned todos.
      # Use assigned_todos(person_id:) to get the actual todos for a specific person.
      #
      # @return [Enumerator<Hash>] list of Person objects
      def assignable_people
        paginate("/reports/todos/assigned.json")
      end

      # Returns all todos assigned to a specific person.
      #
      # @param person_id [Integer, String] person ID
      # @param group_by [String, nil] grouping method: "bucket" or "date"
      # @return [Hash] object with "person", "grouped_by", and "todos" keys
      def assigned_todos(person_id:, group_by: nil)
        params = compact_params(group_by: group_by)
        response = http_get("/reports/todos/assigned/#{person_id}", params: params)
        response.json
      end
    end
  end
end
