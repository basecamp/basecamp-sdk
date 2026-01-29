# frozen_string_literal: true

module Basecamp
  module Services
    # Service for todoset operations.
    #
    # Each project has one todoset which is the container for all todolists.
    #
    # @example Get a todoset
    #   todoset = account.todosets.get(project_id: 123, todoset_id: 456)
    #   puts "#{todoset["name"]} - #{todoset["todolists_count"]} lists"
    class TodosetsService < BaseService
      # Gets a specific todoset.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todoset_id [Integer, String] todoset ID
      # @return [Hash] todoset data
      def get(project_id:, todoset_id:)
        http_get(bucket_path(project_id, "/todosets/#{todoset_id}")).json
      end
    end
  end
end
