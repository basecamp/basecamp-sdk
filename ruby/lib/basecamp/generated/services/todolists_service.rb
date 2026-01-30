# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Todolists operations
    #
    # @generated from OpenAPI spec
    class TodolistsService < BaseService

      # Get a single todolist or todolist group by id
      def get(project_id:, id:)
        http_get(bucket_path(project_id, "/todolists/#{id}")).json
      end

      # Update an existing todolist or todolist group
      def update(project_id:, id:, **body)
        http_put(bucket_path(project_id, "/todolists/#{id}"), body: body).json
      end

      # List todolists in a todoset
      def list(project_id:, todoset_id:, status: nil)
        params = compact_params(status: status)
        paginate(bucket_path(project_id, "/todosets/#{todoset_id}/todolists.json"), params: params)
      end

      # Create a new todolist in a todoset
      def create(project_id:, todoset_id:, **body)
        http_post(bucket_path(project_id, "/todosets/#{todoset_id}/todolists.json"), body: body).json
      end
    end
  end
end
