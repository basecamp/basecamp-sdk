# frozen_string_literal: true

require "uri"

module Basecamp
  module Services
    # Service for Attachments operations
    #
    # @generated from OpenAPI spec
    class AttachmentsService < BaseService

      # Create an attachment (upload a file for embedding)
      # @param data [String] Binary file data to upload
      # @param content_type [String] MIME type of the file (e.g., "application/pdf", "image/png")
      # @param name [String] name
      # @return [Hash] response data
      def create(data:, content_type:, name:)
        with_operation(service: "attachments", operation: "create", is_mutation: true) do
          http_post_raw("/attachments.json?name=#{URI.encode_www_form_component(name.to_s)}", body: data, content_type: content_type).json
        end
      end
    end
  end
end
