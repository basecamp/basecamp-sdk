# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Schedules operations
    #
    # @generated from OpenAPI spec
    class SchedulesService < BaseService

      # Get a single schedule entry by id.
      # @param project_id [Integer] project id ID
      # @param entry_id [Integer] entry id ID
      # @return [Hash] response data
      def get_entry(project_id:, entry_id:)
        with_operation(service: "schedules", operation: "get_entry", is_mutation: false, project_id: project_id, resource_id: entry_id) do
          http_get(bucket_path(project_id, "/schedule_entries/#{entry_id}")).json
        end
      end

      # Update an existing schedule entry
      # @param project_id [Integer] project id ID
      # @param entry_id [Integer] entry id ID
      # @param summary [String, nil] summary
      # @param starts_at [String, nil] starts at (RFC3339 (e.g., 2024-12-15T09:00:00Z))
      # @param ends_at [String, nil] ends at (RFC3339 (e.g., 2024-12-15T09:00:00Z))
      # @param description [String, nil] description
      # @param participant_ids [Array, nil] participant ids
      # @param all_day [Boolean, nil] all day
      # @param notify [Boolean, nil] notify
      # @return [Hash] response data
      def update_entry(project_id:, entry_id:, summary: nil, starts_at: nil, ends_at: nil, description: nil, participant_ids: nil, all_day: nil, notify: nil)
        with_operation(service: "schedules", operation: "update_entry", is_mutation: true, project_id: project_id, resource_id: entry_id) do
          http_put(bucket_path(project_id, "/schedule_entries/#{entry_id}"), body: compact_params(summary: summary, starts_at: starts_at, ends_at: ends_at, description: description, participant_ids: participant_ids, all_day: all_day, notify: notify)).json
        end
      end

      # Get a specific occurrence of a recurring schedule entry
      # @param project_id [Integer] project id ID
      # @param entry_id [Integer] entry id ID
      # @param date [String] date ID
      # @return [Hash] response data
      def get_entry_occurrence(project_id:, entry_id:, date:)
        with_operation(service: "schedules", operation: "get_entry_occurrence", is_mutation: false, project_id: project_id, resource_id: date) do
          http_get(bucket_path(project_id, "/schedule_entries/#{entry_id}/occurrences/#{date}")).json
        end
      end

      # Get a schedule
      # @param project_id [Integer] project id ID
      # @param schedule_id [Integer] schedule id ID
      # @return [Hash] response data
      def get(project_id:, schedule_id:)
        with_operation(service: "schedules", operation: "get", is_mutation: false, project_id: project_id, resource_id: schedule_id) do
          http_get(bucket_path(project_id, "/schedules/#{schedule_id}")).json
        end
      end

      # Update schedule settings
      # @param project_id [Integer] project id ID
      # @param schedule_id [Integer] schedule id ID
      # @param include_due_assignments [Boolean] include due assignments
      # @return [Hash] response data
      def update_settings(project_id:, schedule_id:, include_due_assignments:)
        with_operation(service: "schedules", operation: "update_settings", is_mutation: true, project_id: project_id, resource_id: schedule_id) do
          http_put(bucket_path(project_id, "/schedules/#{schedule_id}"), body: compact_params(include_due_assignments: include_due_assignments)).json
        end
      end

      # List entries on a schedule
      # @param project_id [Integer] project id ID
      # @param schedule_id [Integer] schedule id ID
      # @param status [String, nil] active|archived|trashed
      # @return [Enumerator<Hash>] paginated results
      def list_entries(project_id:, schedule_id:, status: nil)
        wrap_paginated(service: "schedules", operation: "list_entries", is_mutation: false, project_id: project_id, resource_id: schedule_id) do
          params = compact_params(status: status)
          paginate(bucket_path(project_id, "/schedules/#{schedule_id}/entries.json"), params: params)
        end
      end

      # Create a new schedule entry
      # @param project_id [Integer] project id ID
      # @param schedule_id [Integer] schedule id ID
      # @param summary [String] summary
      # @param starts_at [String] starts at (RFC3339 (e.g., 2024-12-15T09:00:00Z))
      # @param ends_at [String] ends at (RFC3339 (e.g., 2024-12-15T09:00:00Z))
      # @param description [String, nil] description
      # @param participant_ids [Array, nil] participant ids
      # @param all_day [Boolean, nil] all day
      # @param notify [Boolean, nil] notify
      # @return [Hash] response data
      def create_entry(project_id:, schedule_id:, summary:, starts_at:, ends_at:, description: nil, participant_ids: nil, all_day: nil, notify: nil)
        with_operation(service: "schedules", operation: "create_entry", is_mutation: true, project_id: project_id, resource_id: schedule_id) do
          http_post(bucket_path(project_id, "/schedules/#{schedule_id}/entries.json"), body: compact_params(summary: summary, starts_at: starts_at, ends_at: ends_at, description: description, participant_ids: participant_ids, all_day: all_day, notify: notify)).json
        end
      end
    end
  end
end
