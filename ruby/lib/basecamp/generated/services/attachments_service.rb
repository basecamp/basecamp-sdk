# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Attachments operations
    #
    # @generated from OpenAPI spec
    class AttachmentsService < BaseService

      # Create an attachment (upload a file for embedding)
      def create(data:, content_type:, name:)
        http_post_raw("/attachments.json?name=#{URI.encode_www_form_component(name.to_s)}", body: data, content_type: content_type).json
      end
    end
  end
end
