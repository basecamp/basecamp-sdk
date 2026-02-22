# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Timesheets operations
    #
    # @generated from OpenAPI spec
    class TimesheetsService < BaseService

      # Get timesheet for a specific recording
      # @param recording_id [Integer] recording id ID
      # @param from [String, nil] from
      # @param to [String, nil] to
      # @param person_id [Integer, nil] person id
      # @return [Hash] response data
      def for_recording(recording_id:, from: nil, to: nil, person_id: nil)
        with_operation(service: "timesheets", operation: "for_recording", is_mutation: false, resource_id: recording_id) do
          http_get("/recordings/#{recording_id}/timesheet.json", params: compact_params(from: from, to: to, person_id: person_id)).json
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
      # @return [Hash] response data
      def report(from: nil, to: nil, person_id: nil)
        with_operation(service: "timesheets", operation: "report", is_mutation: false) do
          http_get("/reports/timesheet.json", params: compact_params(from: from, to: to, person_id: person_id)).json
        end
      end

      # Get timesheet for a specific project
      # @param from [String, nil] from
      # @param to [String, nil] to
      # @param person_id [Integer, nil] person id
      # @return [Hash] response data
      def for_project(from: nil, to: nil, person_id: nil)
        with_operation(service: "timesheets", operation: "for_project", is_mutation: false) do
          http_get("/timesheet.json", params: compact_params(from: from, to: to, person_id: person_id)).json
        end
      end

      # Get a single timesheet entry
      # @param entry_id [Integer] entry id ID
      # @return [Hash] response data
      def get(entry_id:)
        with_operation(service: "timesheets", operation: "get", is_mutation: false, resource_id: entry_id) do
          http_get("/timesheet/entries/#{entry_id}").json
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
          http_put("/timesheet/entries/#{entry_id}", body: compact_params(date: date, hours: hours, description: description, person_id: person_id)).json
        end
      end
    end
  end
end
