# frozen_string_literal: true

module Basecamp
  module Services
    # Service for People operations
    #
    # @generated from OpenAPI spec
    class PeopleService < BaseService

      # List all account users who can be pinged
      # @return [Enumerator<Hash>] paginated results
      def list_pingable()
        wrap_paginated(service: "people", operation: "list_pingable", is_mutation: false) do
          paginate("/circles/people.json")
        end
      end

      # Get the current authenticated user's profile
      # @return [Hash] response data
      def my_profile()
        with_operation(service: "people", operation: "my_profile", is_mutation: false) do
          http_get("/my/profile.json").json
        end
      end

      # List all people visible to the current user
      # @return [Enumerator<Hash>] paginated results
      def list()
        wrap_paginated(service: "people", operation: "list", is_mutation: false) do
          paginate("/people.json")
        end
      end

      # Get a person by ID
      # @param person_id [Integer] person id ID
      # @return [Hash] response data
      def get(person_id:)
        with_operation(service: "people", operation: "get", is_mutation: false, resource_id: person_id) do
          http_get("/people/#{person_id}").json
        end
      end

      # List all active people on a project
      # @param project_id [Integer] project id ID
      # @return [Enumerator<Hash>] paginated results
      def list_for_project(project_id:)
        wrap_paginated(service: "people", operation: "list_for_project", is_mutation: false, project_id: project_id) do
          paginate("/projects/#{project_id}/people.json")
        end
      end

      # Update project access (grant/revoke/create people)
      # @param project_id [Integer] project id ID
      # @param grant [Array, nil] grant
      # @param revoke [Array, nil] revoke
      # @param create [Array, nil] create
      # @return [Hash] response data
      def update_project_access(project_id:, grant: nil, revoke: nil, create: nil)
        with_operation(service: "people", operation: "update_project_access", is_mutation: true, project_id: project_id) do
          http_put("/projects/#{project_id}/people/users.json", body: compact_params(grant: grant, revoke: revoke, create: create)).json
        end
      end

      # List people who can be assigned todos
      # @return [Hash] response data
      def list_assignable()
        with_operation(service: "people", operation: "list_assignable", is_mutation: false) do
          http_get("/reports/todos/assigned.json").json
        end
      end
    end
  end
end
