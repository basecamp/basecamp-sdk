# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Embeds operations
    #
    # @generated from OpenAPI spec
    class EmbedsService < BaseService

      # Create an embed for a URL, or return the one Basecamp already has for it.
      # @param url [String] The URL of the content to embed.
      # @return [Hash] response data
      def create(url:)
        with_operation(service: "embeds", operation: "create", is_mutation: true) do
          http_post("/embeds.json", body: compact_params(url: url)).json(operation: "CreateEmbed")
        end
      end

      # Get an embed by its sgid.
      # @param sgid [String] An embed's signed global id, an opaque string.
      # @return [Hash] response data
      def get(sgid:)
        with_operation(service: "embeds", operation: "get", is_mutation: false) do
          http_get("/embeds/#{path_segment(sgid)}", operation: "GetEmbed").json(operation: "GetEmbed")
        end
      end
    end
  end
end
