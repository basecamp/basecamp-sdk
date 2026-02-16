# frozen_string_literal: true

require "cgi/escape"
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
        @hooks = client.hooks
      end

      protected

      # Wraps a service operation with hooks for observability.
      # @param service [String] service name (e.g., "projects")
      # @param operation [String] operation name (e.g., "list")
      # @param resource_type [String, nil] resource type (e.g., "Project")
      # @param is_mutation [Boolean] whether this is a write operation
      # @param project_id [Integer, String, nil] project/bucket ID
      # @param resource_id [Integer, String, nil] resource ID
      # @yield the operation to execute
      # @return the result of the block
      def with_operation(service:, operation:, resource_type: nil, is_mutation: false, project_id: nil, resource_id: nil)
        info = OperationInfo.new(
          service: service, operation: operation, resource_type: resource_type,
          is_mutation: is_mutation, project_id: project_id, resource_id: resource_id
        )
        start = Process.clock_gettime(Process::CLOCK_MONOTONIC)
        safe_hook { @hooks.on_operation_start(info) }
        result = yield
        duration = ((Process.clock_gettime(Process::CLOCK_MONOTONIC) - start) * 1000).round
        safe_hook { @hooks.on_operation_end(info, OperationResult.new(duration_ms: duration, error: nil)) }
        result
      rescue => e
        duration = ((Process.clock_gettime(Process::CLOCK_MONOTONIC) - start) * 1000).round
        safe_hook { @hooks.on_operation_end(info, OperationResult.new(duration_ms: duration, error: e)) }
        raise
      end

      # Wraps a lazy Enumerator so operation hooks fire around actual iteration,
      # not at enumerator creation time. Hooks fire when the consumer begins
      # iterating (.each, .to_a, .first, etc.) and end fires via ensure when
      # iteration completes, errors, or is cut short by break/take.
      def wrap_paginated(service:, operation:, is_mutation: false, project_id: nil, resource_id: nil)
        info = OperationInfo.new(
          service: service, operation: operation,
          is_mutation: is_mutation, project_id: project_id, resource_id: resource_id
        )
        enum = yield

        hooks = @hooks
        Enumerator.new do |yielder|
          start = Process.clock_gettime(Process::CLOCK_MONOTONIC)
          error = nil
          begin
            safe_hook { hooks.on_operation_start(info) }
            enum.each { |item| yielder.yield(item) }
          rescue => e
            error = e
            raise
          ensure
            duration = ((Process.clock_gettime(Process::CLOCK_MONOTONIC) - start) * 1000).round
            safe_hook { hooks.on_operation_end(info, OperationResult.new(duration_ms: duration, error: error)) }
          end
        end
      end

      # Invoke a hook callback, swallowing exceptions so hooks never break SDK behavior.
      def safe_hook
        yield
      rescue => e
        warn "Basecamp hook error: #{e.class}: #{e.message}"
      end

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
