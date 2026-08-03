# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Timesheets operations
    #
    # @generated from OpenAPI spec
    class TimesheetsService < BaseService

      # Get timesheet for a specific project
      # @param project_id [Integer] project id ID
      # @param from [String, nil] from
      # @param to [String, nil] to
      # @param person_id [Integer, nil] person id
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def for_project(project_id:, from: nil, to: nil, person_id: nil, page: nil, max_items: nil)
        wrap_paginated(service: "timesheets", operation: "for_project", is_mutation: false, project_id: project_id) do
          params = compact_query_params(from: from, to: to, person_id: person_id, page: page)
          paginate("/projects/#{project_id}/timesheet.json", params: params, operation: "GetProjectTimesheet", max_items: max_items)
        end
      end

      # Get timesheet for a specific recording
      # @param recording_id [Integer] recording id ID
      # @param from [String, nil] from
      # @param to [String, nil] to
      # @param person_id [Integer, nil] person id
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def for_recording(recording_id:, from: nil, to: nil, person_id: nil, page: nil, max_items: nil)
        wrap_paginated(service: "timesheets", operation: "for_recording", is_mutation: false, resource_id: recording_id) do
          params = compact_query_params(from: from, to: to, person_id: person_id, page: page)
          paginate("/recordings/#{recording_id}/timesheet.json", params: params, operation: "GetRecordingTimesheet", max_items: max_items)
        end
      end

      # Create a timesheet entry on a recording
      # @param recording_id [Integer] recording id ID
      # @param date [String] date
      # @param hours [String] hours
      # @param description [String, nil] description
      # @param person_id [Integer, nil] person id
      # @return [Hash] response data
      def create(recording_id:, date:, hours:, description: nil, person_id: nil)
        with_operation(service: "timesheets", operation: "create", is_mutation: true, resource_id: recording_id) do
          http_post("/recordings/#{recording_id}/timesheet/entries.json", body: compact_params(date: date, hours: hours, description: description, person_id: person_id)).json
        end
      end

      # Get account-wide timesheet report
      # @param from [String, nil] from
      # @param to [String, nil] to
      # @param person_id [Integer, nil] person id
      # @return [Array<Hash>] response data
      def report(from: nil, to: nil, person_id: nil)
        with_operation(service: "timesheets", operation: "report", is_mutation: false) do
          http_get("/reports/timesheet.json", params: compact_query_params(from: from, to: to, person_id: person_id), operation: "GetTimesheetReport").json
        end
      end

      # Get a single timesheet entry
      # @param entry_id [Integer] entry id ID
      # @return [Hash] response data
      def get(entry_id:)
        with_operation(service: "timesheets", operation: "get", is_mutation: false, resource_id: entry_id) do
          http_get("/timesheet_entries/#{entry_id}", operation: "GetTimesheetEntry").json
        end
      end

      # Update a timesheet entry
      # @param entry_id [Integer] entry id ID
      # @param date [String, nil] date
      # @param hours [String, nil] hours
      # @param description [String, nil] description
      # @param person_id [Integer, nil] person id
      # @return [Hash] response data
      def update(entry_id:, date: nil, hours: nil, description: nil, person_id: nil)
        with_operation(service: "timesheets", operation: "update", is_mutation: true, resource_id: entry_id) do
          http_put("/timesheet_entries/#{entry_id}", body: compact_params(date: date, hours: hours, description: description, person_id: person_id)).json
        end
      end

      # Permanently delete a timesheet entry; answers 403 when the caller may not archive or trash it.
      # @param entry_id [Integer] entry id ID
      # @return [void]
      def destroy(entry_id:)
        with_operation(service: "timesheets", operation: "destroy", is_mutation: true, resource_id: entry_id) do
          http_delete("/timesheet_entries/#{entry_id}")
          nil
        end
      end
    end
  end
end
