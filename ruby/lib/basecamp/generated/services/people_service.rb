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

      # Get the current user's preferences
      # @return [Hash] response data
      def get_my_preferences()
        with_operation(service: "people", operation: "get_my_preferences", is_mutation: false) do
          http_get("/my/preferences.json").json
        end
      end

      # Update the current user's preferences
      # @param person [String] person
      # @return [Hash] response data
      def update_my_preferences(person:)
        with_operation(service: "people", operation: "update_my_preferences", is_mutation: true) do
          http_put("/my/preferences.json", body: compact_params(person: person)).json
        end
      end

      # Get the current authenticated user's profile
      # @return [Hash] response data
      def my_profile()
        with_operation(service: "people", operation: "my_profile", is_mutation: false) do
          http_get("/my/profile.json").json
        end
      end

      # Update the current authenticated user's profile (returns 204 No Content)
      # @param name [String, nil] name
      # @param email_address [String, nil] email address
      # @param title [String, nil] title
      # @param bio [String, nil] bio
      # @param location [String, nil] location
      # @param time_zone_name [String, nil] time zone name
      # @param first_week_day [String, nil] first week day
      # @param time_format [String, nil] time format
      # @return [void]
      def update_my_profile(name: nil, email_address: nil, title: nil, bio: nil, location: nil, time_zone_name: nil, first_week_day: nil, time_format: nil)
        with_operation(service: "people", operation: "update_my_profile", is_mutation: true) do
          http_put("/my/profile.json", body: compact_params(name: name, email_address: email_address, title: title, bio: bio, location: location, time_zone_name: time_zone_name, first_week_day: first_week_day, time_format: time_format))
          nil
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

      # Get the out of office status for a person
      # @param person_id [Integer] person id ID
      # @return [Hash] response data
      def get_out_of_office(person_id:)
        with_operation(service: "people", operation: "get_out_of_office", is_mutation: false, resource_id: person_id) do
          http_get("/people/#{person_id}/out_of_office.json").json
        end
      end

      # Enable or replace out of office for a person.
      # @param person_id [Integer] person id ID
      # @param out_of_office [String] out of office
      # @return [Hash] response data
      def enable_out_of_office(person_id:, out_of_office:)
        with_operation(service: "people", operation: "enable_out_of_office", is_mutation: true, resource_id: person_id) do
          http_post("/people/#{person_id}/out_of_office.json", body: compact_params(out_of_office: out_of_office)).json
        end
      end

      # Disable out of office for a person.
      # @param person_id [Integer] person id ID
      # @return [void]
      def disable_out_of_office(person_id:)
        with_operation(service: "people", operation: "disable_out_of_office", is_mutation: true, resource_id: person_id) do
          http_delete("/people/#{person_id}/out_of_office.json")
          nil
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
