# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Schedules operations
    #
    # @generated from OpenAPI spec
    class SchedulesService < BaseService

      # Get a single schedule entry by id.
      # @param entry_id [Integer] entry id ID
      # @return [Hash] response data
      def get_entry(entry_id:)
        http_get("/schedule_entries/#{entry_id}").json
      end

      # Update an existing schedule entry
      # @param entry_id [Integer] entry id ID
      # @param summary [String, nil] summary
      # @param starts_at [String, nil] starts at (RFC3339 (e.g., 2024-12-15T09:00:00Z))
      # @param ends_at [String, nil] ends at (RFC3339 (e.g., 2024-12-15T09:00:00Z))
      # @param description [String, nil] description
      # @param participant_ids [Array, nil] participant ids
      # @param all_day [Boolean, nil] all day
      # @param notify [Boolean, nil] notify
      # @return [Hash] response data
      def update_entry(entry_id:, summary: nil, starts_at: nil, ends_at: nil, description: nil, participant_ids: nil, all_day: nil, notify: nil)
        http_put("/schedule_entries/#{entry_id}", body: compact_params(summary: summary, starts_at: starts_at, ends_at: ends_at, description: description, participant_ids: participant_ids, all_day: all_day, notify: notify)).json
      end

      # Get a specific occurrence of a recurring schedule entry
      # @param entry_id [Integer] entry id ID
      # @param date [String] date ID
      # @return [Hash] response data
      def get_entry_occurrence(entry_id:, date:)
        http_get("/schedule_entries/#{entry_id}/occurrences/#{date}").json
      end

      # Get a schedule
      # @param schedule_id [Integer] schedule id ID
      # @return [Hash] response data
      def get(schedule_id:)
        http_get("/schedules/#{schedule_id}").json
      end

      # Update schedule settings
      # @param schedule_id [Integer] schedule id ID
      # @param include_due_assignments [Boolean] include due assignments
      # @return [Hash] response data
      def update_settings(schedule_id:, include_due_assignments:)
        http_put("/schedules/#{schedule_id}", body: compact_params(include_due_assignments: include_due_assignments)).json
      end

      # List entries on a schedule
      # @param schedule_id [Integer] schedule id ID
      # @param status [String, nil] active|archived|trashed
      # @return [Enumerator<Hash>] paginated results
      def list_entries(schedule_id:, status: nil)
        params = compact_params(status: status)
        paginate("/schedules/#{schedule_id}/entries.json", params: params)
      end

      # Create a new schedule entry
      # @param schedule_id [Integer] schedule id ID
      # @param summary [String] summary
      # @param starts_at [String] starts at (RFC3339 (e.g., 2024-12-15T09:00:00Z))
      # @param ends_at [String] ends at (RFC3339 (e.g., 2024-12-15T09:00:00Z))
      # @param description [String, nil] description
      # @param participant_ids [Array, nil] participant ids
      # @param all_day [Boolean, nil] all day
      # @param notify [Boolean, nil] notify
      # @return [Hash] response data
      def create_entry(schedule_id:, summary:, starts_at:, ends_at:, description: nil, participant_ids: nil, all_day: nil, notify: nil)
        http_post("/schedules/#{schedule_id}/entries.json", body: compact_params(summary: summary, starts_at: starts_at, ends_at: ends_at, description: description, participant_ids: participant_ids, all_day: all_day, notify: notify)).json
      end
    end
  end
end
