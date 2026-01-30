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
        paginate("/circles/people.json")
      end

      # Get the current authenticated user's profile
      # @return [Hash] response data
      def my_profile()
        http_get("/my/profile.json").json
      end

      # List all people visible to the current user
      # @return [Enumerator<Hash>] paginated results
      def list()
        paginate("/people.json")
      end

      # Get a person by ID
      # @param person_id [Integer] person id ID
      # @return [Hash] response data
      def get(person_id:)
        http_get("/people/#{person_id}").json
      end

      # List all active people on a project
      # @param project_id [Integer] project id ID
      # @return [Enumerator<Hash>] paginated results
      def list_for_project(project_id:)
        paginate("/projects/#{project_id}/people.json")
      end

      # Update project access (grant/revoke/create people)
      # @param project_id [Integer] project id ID
      # @param grant [Array, nil] grant
      # @param revoke [Array, nil] revoke
      # @param create [Array, nil] create
      # @return [Hash] response data
      def update_project_access(project_id:, grant: nil, revoke: nil, create: nil)
        http_put("/projects/#{project_id}/people/users.json", body: compact_params(grant: grant, revoke: revoke, create: create)).json
      end

      # List people who can be assigned todos
      # @return [Hash] response data
      def list_assignable()
        http_get("/reports/todos/assigned.json").json
      end
    end
  end
end
