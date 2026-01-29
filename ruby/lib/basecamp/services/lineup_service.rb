# frozen_string_literal: true

module Basecamp
  module Services
    # Service for lineup operations.
    #
    # The Lineup is Basecamp's visual timeline tool for tracking
    # project schedules and milestones.
    #
    # @example Create a marker
    #   marker = account.lineup.create_marker(
    #     title: "Launch Day",
    #     starts_on: "2024-03-01",
    #     ends_on: "2024-03-01",
    #     color: "green"
    #   )
    class LineupService < BaseService
      # Creates a new marker on the lineup.
      #
      # @param title [String] marker title
      # @param starts_on [String] start date (YYYY-MM-DD)
      # @param ends_on [String] end date (YYYY-MM-DD)
      # @param color [String, nil] marker color (white, red, orange, yellow, green, blue, aqua, purple, gray, pink, brown)
      # @param description [String, nil] description in HTML
      # @return [Hash] created marker
      def create_marker(title:, starts_on:, ends_on:, color: nil, description: nil)
        body = compact_params(
          title: title,
          starts_on: starts_on,
          ends_on: ends_on,
          color: color,
          description: description
        )
        http_post("/lineup/markers.json", body: body).json
      end

      # Updates an existing marker.
      #
      # @param marker_id [Integer, String] marker ID
      # @param title [String, nil] new title
      # @param starts_on [String, nil] new start date (YYYY-MM-DD)
      # @param ends_on [String, nil] new end date (YYYY-MM-DD)
      # @param color [String, nil] new color
      # @param description [String, nil] new description
      # @return [Hash] updated marker
      def update_marker(marker_id:, title: nil, starts_on: nil, ends_on: nil, color: nil, description: nil)
        body = compact_params(
          title: title,
          starts_on: starts_on,
          ends_on: ends_on,
          color: color,
          description: description
        )
        http_put("/lineup/markers/#{marker_id}", body: body).json
      end

      # Deletes a marker.
      #
      # @param marker_id [Integer, String] marker ID
      # @return [void]
      def delete_marker(marker_id:)
        http_delete("/lineup/markers/#{marker_id}")
        nil
      end
    end
  end
end
