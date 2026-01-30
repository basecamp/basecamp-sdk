# frozen_string_literal: true

module Basecamp
  module Services
    # Service for People operations
    #
    # @generated from OpenAPI spec
    class PeopleService < BaseService

      # List all account users who can be pinged
      def list_pingable()
        paginate("/circles/people.json")
      end

      # Get the current authenticated user's profile
      def my_profile()
        http_get("/my/profile.json").json
      end

      # List all people visible to the current user
      def list()
        paginate("/people.json")
      end

      # Get a person by ID
      def get(person_id:)
        http_get("/people/#{person_id}").json
      end

      # List all active people on a project
      def list_for_project(project_id:)
        paginate("/projects/#{project_id}/people.json")
      end

      # Update project access (grant/revoke/create people)
      def update_project_access(project_id:, **body)
        http_put("/projects/#{project_id}/people/users.json", body: body).json
      end

      # List people who can be assigned todos
      def list_assignable()
        http_get("/reports/todos/assigned.json").json
      end
    end
  end
end
