# frozen_string_literal: true

module Basecamp
  module Services
    # Service for people operations.
    #
    # People are the users in your Basecamp account.
    #
    # @example List all people
    #   account.people.list.each do |person|
    #     puts "#{person["name"]} <#{person["email_address"]}>"
    #   end
    #
    # @example Get current user's profile
    #   me = account.people.me
    #   puts "Logged in as #{me["name"]}"
    class PeopleService < BaseService
      # Lists all people in the account.
      #
      # @return [Enumerator<Hash>] people
      def list
        paginate("/people.json")
      end

      # Gets a specific person.
      #
      # @param person_id [Integer, String] person ID
      # @return [Hash] person data
      def get(person_id:)
        http_get("/people/#{person_id}").json
      end

      # Gets the current user's profile.
      #
      # @return [Hash] current user's profile
      def me
        http_get("/my/profile.json").json
      end

      # Lists all people who can be pinged (mentioned).
      #
      # @return [Enumerator<Hash>] pingable people
      def list_pingable
        paginate("/circles/people.json")
      end

      # Lists all people in a project.
      #
      # @param project_id [Integer, String] project ID
      # @return [Enumerator<Hash>] project members
      def list_project_people(project_id:)
        paginate("/projects/#{project_id}/people.json")
      end

      # Updates project access for users.
      #
      # @param project_id [Integer, String] project ID
      # @param grant [Array<Integer>, nil] user IDs to grant access
      # @param revoke [Array<Integer>, nil] user IDs to revoke access
      # @param create [Array<Hash>, nil] new users to create and grant access
      # @return [void]
      def update_project_access(project_id:, grant: nil, revoke: nil, create: nil)
        body = compact_params(
          grant: grant,
          revoke: revoke,
          create: create
        )
        http_put("/projects/#{project_id}/people/users.json", body: body)
        nil
      end
    end
  end
end
