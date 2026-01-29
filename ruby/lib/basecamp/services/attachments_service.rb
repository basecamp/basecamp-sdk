# frozen_string_literal: true

require "uri"

module Basecamp
  module Services
    # Service for attachment operations.
    #
    # Attachments are used to upload files that can be embedded in rich text
    # content like messages, comments, and documents. After uploading, you
    # receive an attachable_sgid that can be used to embed the file in HTML.
    #
    # @example Upload a file
    #   attachment = account.attachments.create(
    #     filename: "report.pdf",
    #     content_type: "application/pdf",
    #     data: file_content
    #   )
    #   # Use in HTML: <bc-attachment sgid="#{attachment["attachable_sgid"]}"></bc-attachment>
    class AttachmentsService < BaseService
      # Creates an attachment by uploading a file.
      # Returns an attachable_sgid for embedding the file in rich text content.
      #
      # @param filename [String] filename for the uploaded file
      # @param content_type [String] MIME content type (e.g., "image/png", "application/pdf")
      # @param data [String] file data
      # @return [Hash] attachment response with attachable_sgid
      def create(filename:, content_type:, data:)
        http_post_raw("/attachments.json?name=#{URI.encode_www_form_component(filename)}", body: data, content_type: content_type).json
      end
    end
  end
end
