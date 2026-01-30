# frozen_string_literal: true

require_relative "../errors"

module Basecamp
  module Services
    # Base service class for Basecamp API services.
    #
    # Provides shared functionality for all service classes including:
    # - HTTP method delegation (http_get, http_post, etc.)
    # - Path building helpers
    # - Pagination support
    #
    # @example
    #   class TodosService < BaseService
    #     def list(project_id:, todolist_id:)
    #       paginate(bucket_path(project_id, "/todolists/#{todolist_id}/todos.json"))
    #     end
    #   end
    class BaseService
      # @return [String] the account ID for API requests
      attr_reader :account_id

      # @param client [Object] the parent client (AccountClient or Client)
      def initialize(client)
        @client = client
        @account_id = client.account_id
      end

      protected

      # @return [HTTP] the HTTP client for direct access
      def http
        @client.http
      end

      # Helper to remove nil values from a hash.
      # @param hash [Hash] the input hash
      # @return [Hash] hash with nil values removed
      def compact_params(**kwargs)
        kwargs.compact
      end

      # Build a bucket (project) path.
      # @param project_id [Integer, String] the project/bucket ID
      # @param path [String] the path suffix
      # @return [String] the full bucket path
      def bucket_path(project_id, path)
        "/buckets/#{project_id}#{path}"
      end

      # Delegate HTTP methods to the client with http_ prefix to avoid conflicts
      # with service method names (e.g., service.get vs http_get)
      # @!method http_get(path, params: {})
      #   @see AccountClient#get
      # @!method http_post(path, body: nil)
      #   @see AccountClient#post
      # @!method http_put(path, body: nil)
      #   @see AccountClient#put
      # @!method http_delete(path)
      #   @see AccountClient#delete
      # @!method http_post_raw(path, body:, content_type:)
      #   @see AccountClient#post_raw
      # @!method paginate(path, params: {}, &block)
      #   @see AccountClient#paginate
      %i[get post put delete post_raw].each do |method|
        define_method(:"http_#{method}") do |*args, **kwargs, &block|
          @client.public_send(method, *args, **kwargs, &block)
        end
      end

      # Paginate doesn't conflict with service methods, keep as-is
      def paginate(...)
        @client.paginate(...)
      end

      # Paginate extracting items from a specific key (for object responses)
      def paginate_key(...)
        @client.paginate_key(...)
      end
    end
  end
end
