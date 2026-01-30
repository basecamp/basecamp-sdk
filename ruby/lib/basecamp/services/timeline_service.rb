# frozen_string_literal: true

module Basecamp
  module Services
    # Service for timeline and progress report operations.
    #
    # Provides access to activity feeds showing recent activity across the account,
    # within specific projects, or for specific people.
    #
    # @example Get account-wide progress report
    #   account.timeline.progress.each { |event| puts event["action"] }
    #
    # @example Get project timeline
    #   account.timeline.project_timeline(project_id: 123).each { |event| puts event }
    #
    # @example Get a person's progress (returns hash with person info)
    #   result = account.timeline.person_progress(person_id: 456)
    #   puts result["person"]["name"]
    #   result["events"].each { |event| puts event["action"] }
    class TimelineService < BaseService
      # Returns the account-wide progress report.
      # This shows recent activity across all projects.
      # Results are paginated via Link header.
      #
      # @return [Enumerator<Hash>] timeline events
      def progress
        paginate_key("/reports/progress.json", key: "events")
      end

      # Returns the activity timeline for a specific project.
      # Results are paginated via Link header.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @return [Enumerator<Hash>] timeline events
      def project_timeline(project_id:)
        paginate_key(bucket_path(project_id, "/timeline.json"), key: "events")
      end

      # Returns the progress report for a specific person, including person metadata.
      #
      # @note Returns first page only to preserve person metadata.
      #       Use {#person_progress_events} for full paginated event stream.
      # @param person_id [Integer, String] person ID
      # @return [Hash] object with "person" and "events" keys
      def person_progress(person_id:)
        response = http_get("/reports/users/progress/#{person_id}")
        response.json
      end

      # Returns all progress events for a specific person.
      # Results are paginated via Link header.
      #
      # @note Does not include person metadata. Use {#person_progress} if you need
      #       the person object along with the first page of events.
      # @param person_id [Integer, String] person ID
      # @return [Enumerator<Hash>] timeline events
      def person_progress_events(person_id:)
        paginate_key("/reports/users/progress/#{person_id}", key: "events")
      end
    end
  end
end
