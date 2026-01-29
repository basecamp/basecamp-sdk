# frozen_string_literal: true

module Basecamp
  module Services
    # Service for schedule (calendar) operations.
    #
    # Schedules are calendars within projects that contain schedule entries (events).
    # Each project has one schedule that can optionally show todo due dates.
    #
    # @example Get a schedule
    #   schedule = account.schedules.get(project_id: 123, schedule_id: 456)
    #
    # @example List entries
    #   account.schedules.list_entries(project_id: 123, schedule_id: 456).each do |entry|
    #     puts "#{entry["summary"]} - #{entry["starts_at"]}"
    #   end
    #
    # @example Create an entry
    #   entry = account.schedules.create_entry(
    #     project_id: 123,
    #     schedule_id: 456,
    #     summary: "Team Meeting",
    #     starts_at: "2024-12-15T09:00:00Z",
    #     ends_at: "2024-12-15T10:00:00Z"
    #   )
    class SchedulesService < BaseService
      # Gets a specific schedule.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param schedule_id [Integer, String] schedule ID
      # @return [Hash] schedule data
      def get(project_id:, schedule_id:)
        http_get(bucket_path(project_id, "/schedules/#{schedule_id}")).json
      end

      # Lists all entries on a schedule.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param schedule_id [Integer, String] schedule ID
      # @return [Enumerator<Hash>] schedule entries
      def list_entries(project_id:, schedule_id:)
        paginate(bucket_path(project_id, "/schedules/#{schedule_id}/entries.json"))
      end

      # Gets a specific schedule entry.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param entry_id [Integer, String] schedule entry ID
      # @return [Hash] schedule entry data
      def get_entry(project_id:, entry_id:)
        http_get(bucket_path(project_id, "/schedule_entries/#{entry_id}")).json
      end

      # Creates a new entry on a schedule.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param schedule_id [Integer, String] schedule ID
      # @param summary [String] event title
      # @param starts_at [String] start time in RFC3339 format
      # @param ends_at [String] end time in RFC3339 format
      # @param description [String, nil] event description in HTML
      # @param participant_ids [Array<Integer>, nil] person IDs to assign
      # @param all_day [Boolean, nil] whether this is an all-day event
      # @param notify [Boolean, nil] whether to notify participants
      # @return [Hash] created entry
      def create_entry(
        project_id:,
        schedule_id:,
        summary:,
        starts_at:,
        ends_at:,
        description: nil,
        participant_ids: nil,
        all_day: nil,
        notify: nil
      )
        body = compact_params(
          summary: summary,
          starts_at: starts_at,
          ends_at: ends_at,
          description: description,
          participant_ids: participant_ids,
          all_day: all_day,
          notify: notify
        )
        http_post(bucket_path(project_id, "/schedules/#{schedule_id}/entries.json"), body: body).json
      end

      # Updates an existing schedule entry.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param entry_id [Integer, String] schedule entry ID
      # @param summary [String, nil] new title
      # @param starts_at [String, nil] new start time in RFC3339 format
      # @param ends_at [String, nil] new end time in RFC3339 format
      # @param description [String, nil] new description in HTML
      # @param participant_ids [Array<Integer>, nil] new participant IDs
      # @param all_day [Boolean, nil] whether this is an all-day event
      # @param notify [Boolean, nil] whether to notify participants
      # @return [Hash] updated entry
      def update_entry(
        project_id:,
        entry_id:,
        summary: nil,
        starts_at: nil,
        ends_at: nil,
        description: nil,
        participant_ids: nil,
        all_day: nil,
        notify: nil
      )
        body = compact_params(
          summary: summary,
          starts_at: starts_at,
          ends_at: ends_at,
          description: description,
          participant_ids: participant_ids,
          all_day: all_day,
          notify: notify
        )
        http_put(bucket_path(project_id, "/schedule_entries/#{entry_id}"), body: body).json
      end

      # Gets a specific occurrence of a recurring schedule entry.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param entry_id [Integer, String] schedule entry ID
      # @param date [String] occurrence date in YYYY-MM-DD format
      # @return [Hash] schedule entry occurrence
      def get_entry_occurrence(project_id:, entry_id:, date:)
        http_get(bucket_path(project_id, "/schedule_entries/#{entry_id}/occurrences/#{date}")).json
      end

      # Updates schedule settings.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param schedule_id [Integer, String] schedule ID
      # @param include_due_assignments [Boolean] whether to show todo due dates
      # @return [Hash] updated schedule
      def update_settings(project_id:, schedule_id:, include_due_assignments:)
        body = { include_due_assignments: include_due_assignments }
        http_put(bucket_path(project_id, "/schedules/#{schedule_id}"), body: body).json
      end

      # Moves a schedule entry to the trash.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param entry_id [Integer, String] schedule entry ID
      # @return [void]
      def trash_entry(project_id:, entry_id:)
        http_put(bucket_path(project_id, "/recordings/#{entry_id}/status/trashed.json"))
        nil
      end
    end
  end
end
