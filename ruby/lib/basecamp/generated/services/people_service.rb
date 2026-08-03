# frozen_string_literal: true

module Basecamp
  module Services
    # Service for People operations
    #
    # @generated from OpenAPI spec
    class PeopleService < BaseService

      # List all account users who can be pinged
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list_pingable(max_items: nil)
        wrap_paginated(service: "people", operation: "list_pingable", is_mutation: false) do
          paginate("/circles/people.json", operation: "ListPingablePeople", max_items: max_items)
        end
      end

      # Get the current user's preferences
      # @return [Hash] response data
      def get_my_preferences()
        with_operation(service: "people", operation: "get_my_preferences", is_mutation: false) do
          http_get("/my/preferences.json", operation: "GetMyPreferences").json
        end
      end

      # Update the current user's preferences.
      # @param person [Hash] person
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
          http_get("/my/profile.json", operation: "GetMyProfile").json
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
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(page: nil, max_items: nil)
        wrap_paginated(service: "people", operation: "list", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/people.json", params: params, operation: "ListPeople", max_items: max_items)
        end
      end

      # Get a person by ID
      # @param person_id [Integer] person id ID
      # @return [Hash] response data
      def get(person_id:)
        with_operation(service: "people", operation: "get", is_mutation: false, resource_id: person_id) do
          http_get("/people/#{person_id}", operation: "GetPerson").json
        end
      end

      # Get the out of office status for a person
      # @param person_id [Integer] person id ID
      # @return [Hash] response data
      def get_out_of_office(person_id:)
        with_operation(service: "people", operation: "get_out_of_office", is_mutation: false, resource_id: person_id) do
          http_get("/people/#{person_id}/out_of_office.json", operation: "GetOutOfOffice").json
        end
      end

      # Enable or replace out of office for a person.
      # @param person_id [Integer] person id ID
      # @param out_of_office [Hash] out of office
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
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list_for_project(project_id:, page: nil, max_items: nil)
        wrap_paginated(service: "people", operation: "list_for_project", is_mutation: false, project_id: project_id) do
          params = compact_query_params(page: page)
          paginate("/projects/#{project_id}/people.json", params: params, operation: "ListProjectPeople", max_items: max_items)
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
      # @return [Array<Hash>] response data
      def list_assignable()
        with_operation(service: "people", operation: "list_assignable", is_mutation: false) do
          http_get("/reports/todos/assigned.json", operation: "ListAssignablePeople").json
        end
      end
    end
  end
end
