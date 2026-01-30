# frozen_string_literal: true

module Basecamp
  module Services
    # Service for event operations.
    #
    # Events are activity records that track changes to recordings.
    # An event is created any time a recording is modified (created,
    # updated, completed, etc.).
    #
    # @example List events for a recording
    #   account.events.list(project_id: 123, recording_id: 456).each do |event|
    #     puts "#{event["action"]} by #{event["creator"]["name"]} at #{event["created_at"]}"
    #   end
    class EventsService < BaseService
      # Lists all events for a recording.
      # Events track all changes made to a recording over time.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @return [Enumerator<Hash>] events
      def list(project_id:, recording_id:)
        paginate(bucket_path(project_id, "/recordings/#{recording_id}/events.json"))
      end
    end
  end
end
