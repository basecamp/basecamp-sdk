# frozen_string_literal: true

module Basecamp
  module Services
    # Service for CloudFiles operations
    #
    # @generated from OpenAPI spec
    class CloudFilesService < BaseService

      # Create a new cloud file in a vault.
      # @param bucket_id [Integer] bucket id ID
      # @param vault_id [Integer] vault id ID
      # @param url [String] url
      # @param service [String] Short identifier for the external service — "dropbox", "google_doc",
      #   "figma", "other", … Derived from the CloudFile::Service subclass name, so it
      #   is always present. `other` accepts any well-formed HTTPS URL.
      # @param title [String, nil] title
      # @param description [String, nil] description
      # @param subscriptions [Array, nil] subscriptions
      # @param visible_to_clients [Boolean, nil] Whether the cloud file is visible to the project's clients. Applies only
      #   when creating directly in the tool's vault — an item created inside a
      #   folder inherits the folder's visibility and ignores this. A client caller
      #   always creates client-visible records regardless of what is sent.
      # @param notify [String, nil] Who to notify: "default", "everyone", or "custom" (the people in subscriptions).
      #
      #   Omit both this and subscriptions to notify nobody. A subscriptions array sent
      #   without notify is read as "custom".
      # @return [Hash] response data
      def create_cloud_file(bucket_id:, vault_id:, url:, service:, title: nil, description: nil, subscriptions: nil, visible_to_clients: nil, notify: nil)
        with_operation(service: "cloudfiles", operation: "create_cloud_file", is_mutation: true, project_id: bucket_id, resource_id: vault_id) do
          http_post("/buckets/#{bucket_id}/vaults/#{vault_id}/cloud_files.json", body: compact_params(url: url, service: service, title: title, description: description, subscriptions: subscriptions, visible_to_clients: visible_to_clients, notify: notify)).json(operation: "CreateCloudFile")
        end
      end

      # Get a single cloud file by id
      # @param cloud_file_id [Integer] cloud file id ID
      # @return [Hash] response data
      def get_cloud_file(cloud_file_id:)
        with_operation(service: "cloudfiles", operation: "get_cloud_file", is_mutation: false, resource_id: cloud_file_id) do
          http_get("/cloud_files/#{cloud_file_id}", operation: "GetCloudFile").json(operation: "GetCloudFile")
        end
      end

      # Replace a cloud file with a new complete representation.
      # @param cloud_file_id [Integer] cloud file id ID
      # @param url [String] url
      # @param service [String] Short identifier for the external service — "dropbox", "google_doc",
      #   "figma", "other", … Derived from the CloudFile::Service subclass name, so it
      #   is always present. `other` accepts any well-formed HTTPS URL.
      # @param title [String, nil] title
      # @param description [String, nil] description
      # @param subscriptions [Array, nil] subscriptions
      # @param notify [String, nil] Who to notify: "default", "everyone", or "custom" (the people in subscriptions).
      #
      #   Applies only to a drafted cloud file, whose subscribers it recomputes; a
      #   published one ignores it. Omit both this and subscriptions to keep a draft's
      #   current subscribers. A subscriptions array sent without notify is read as
      #   "custom".
      # @return [Hash] response data
      def update_cloud_file(cloud_file_id:, url:, service:, title: nil, description: nil, subscriptions: nil, notify: nil)
        with_operation(service: "cloudfiles", operation: "update_cloud_file", is_mutation: true, resource_id: cloud_file_id) do
          http_put("/cloud_files/#{cloud_file_id}", body: compact_params(url: url, service: service, title: title, description: description, subscriptions: subscriptions, notify: notify)).json(operation: "UpdateCloudFile")
        end
      end
    end
  end
end
