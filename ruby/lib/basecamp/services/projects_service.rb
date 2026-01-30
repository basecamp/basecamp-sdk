# frozen_string_literal: true

module Basecamp
  module Services
    # Service for project (Basecamp) operations.
    #
    # @example List all projects
    #   account.projects.list.each do |project|
    #     puts "#{project["name"]} (#{project["id"]})"
    #   end
    #
    # @example Get a specific project
    #   project = account.projects.get(123)
    #   puts project["name"]
    #
    # @example Create a project
    #   project = account.projects.create(name: "My Project", description: "A new project")
    class ProjectsService < BaseService
      # Lists all projects in the account.
      #
      # @param status [String, nil] filter by status ("active", "archived", "trashed")
      # @return [Enumerator<Hash>] projects
      def list(status: nil)
        params = compact_params(status: status)
        paginate("/projects.json", params: params)
      end

      # Gets a specific project.
      #
      # @param project_id [Integer, String] project ID
      # @return [Hash] project data
      def get(project_id)
        http_get("/projects/#{project_id}.json").json
      end

      # Creates a new project.
      #
      # @param name [String] project name
      # @param description [String, nil] project description
      # @return [Hash] created project
      def create(name:, description: nil)
        body = compact_params(name: name, description: description)
        http_post("/projects.json", body: body).json
      end

      # Updates a project.
      #
      # @param project_id [Integer, String] project ID
      # @param name [String, nil] new name
      # @param description [String, nil] new description
      # @return [Hash] updated project
      def update(project_id, name: nil, description: nil)
        body = compact_params(name: name, description: description)
        http_put("/projects/#{project_id}.json", body: body).json
      end

      # Trashes a project.
      #
      # @param project_id [Integer, String] project ID
      # @return [void]
      def trash(project_id)
        http_delete("/projects/#{project_id}.json")
        nil
      end
    end
  end
end
