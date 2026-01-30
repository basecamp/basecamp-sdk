# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Schedules operations
    #
    # @generated from OpenAPI spec
    class SchedulesService < BaseService

      # Get a single schedule entry by id
      def get_entry(project_id:, entry_id:)
        http_get(bucket_path(project_id, "/schedule_entries/#{entry_id}")).json
      end

      # Update an existing schedule entry
      def update_entry(project_id:, entry_id:, **body)
        http_put(bucket_path(project_id, "/schedule_entries/#{entry_id}"), body: body).json
      end

      # Get a specific occurrence of a recurring schedule entry
      def get_entry_occurrence(project_id:, entry_id:, date:)
        http_get(bucket_path(project_id, "/schedule_entries/#{entry_id}/occurrences/#{date}")).json
      end

      # Get a schedule
      def get(project_id:, schedule_id:)
        http_get(bucket_path(project_id, "/schedules/#{schedule_id}")).json
      end

      # Update schedule settings
      def update_settings(project_id:, schedule_id:, **body)
        http_put(bucket_path(project_id, "/schedules/#{schedule_id}"), body: body).json
      end

      # List entries on a schedule
      def list_entries(project_id:, schedule_id:, status: nil)
        params = compact_params(status: status)
        paginate(bucket_path(project_id, "/schedules/#{schedule_id}/entries.json"), params: params)
      end

      # Create a new schedule entry
      def create_entry(project_id:, schedule_id:, **body)
        http_post(bucket_path(project_id, "/schedules/#{schedule_id}/entries.json"), body: body).json
      end
    end
  end
end
