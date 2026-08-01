# frozen_string_literal: true

require "cgi/escape"
require "securerandom"

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

      # Wraps a paginated operation with hooks, preserving the pagination
      # metadata carried by the inner ListEnumerator.
      # Fires on_operation_start eagerly (paginate fetches page 1 inside the
      # yield, so the start hook must precede it); on_operation_end fires via
      # ensure when the first traversal completes, errors, or is cut short by
      # break/take — exactly once, so re-enumerating never emits an
      # on_operation_end without a matching start.
      def wrap_paginated(service:, operation:, is_mutation: false, project_id: nil, resource_id: nil)
        info = OperationInfo.new(
          service: service, operation: operation,
          is_mutation: is_mutation, project_id: project_id, resource_id: resource_id
        )
        hooks = @hooks
        start = Process.clock_gettime(Process::CLOCK_MONOTONIC)
        safe_hook { hooks.on_operation_start(info) }

        begin
          enum = yield  # paginate fetches page 1 here
        rescue => e
          duration = ((Process.clock_gettime(Process::CLOCK_MONOTONIC) - start) * 1000).round
          safe_hook { hooks.on_operation_end(info, OperationResult.new(duration_ms: duration, error: e)) }
          raise
        end

        ended = false
        ListEnumerator.new(enum.meta) do |yielder|
          error = nil
          begin
            enum.each { |item| yielder.yield(item) }
          rescue => e
            error = e
            raise
          ensure
            unless ended
              ended = true
              duration = ((Process.clock_gettime(Process::CLOCK_MONOTONIC) - start) * 1000).round
              safe_hook { hooks.on_operation_end(info, OperationResult.new(duration_ms: duration, error: error)) }
            end
          end
        end
      end

      # Wraps a wrapped-paginated operation with hooks.
      # Fires on_operation_start eagerly (before page 1 fetch), and
      # on_operation_end exactly once, when the first traversal of the events
      # Enumerator completes/errors/breaks.
      def wrap_paginated_wrapped(key:, service:, operation:, is_mutation: false, project_id: nil, resource_id: nil)
        info = OperationInfo.new(
          service: service, operation: operation,
          is_mutation: is_mutation, project_id: project_id, resource_id: resource_id
        )
        hooks = @hooks
        start = Process.clock_gettime(Process::CLOCK_MONOTONIC)
        safe_hook { hooks.on_operation_start(info) }

        begin
          result = yield  # paginate_wrapped fetches page 1 here
        rescue => e
          duration = ((Process.clock_gettime(Process::CLOCK_MONOTONIC) - start) * 1000).round
          safe_hook { hooks.on_operation_end(info, OperationResult.new(duration_ms: duration, error: e)) }
          raise
        end

        inner_enum = result[key]
        ended = false
        wrapped_enum = ListEnumerator.new(inner_enum.meta) do |yielder|
          error = nil
          begin
            inner_enum.each { |item| yielder.yield(item) }
          rescue => e
            error = e
            raise
          ensure
            # Exactly once: on_operation_start fired once at call time, so
            # re-enumerating must not emit unmatched on_operation_end events.
            unless ended
              ended = true
              duration = ((Process.clock_gettime(Process::CLOCK_MONOTONIC) - start) * 1000).round
              safe_hook { hooks.on_operation_end(info, OperationResult.new(duration_ms: duration, error: error)) }
            end
          end
        end

        result.merge(key => wrapped_enum)
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

      # Like compact_params, but also drops empty arrays. For query filters an
      # empty array means "no filter" and must be omitted entirely: Faraday's
      # nested encoder would otherwise emit a bare `bucket_ids[]`, which Rails
      # normalizes into a bogus `[0]` filter instead of the unfiltered request
      # the caller intended. Not used for request bodies, where an explicit
      # empty array (e.g. clearing a list) can be meaningful.
      #
      # @param hash [Hash] the input hash
      # @return [Hash] hash with nil values and empty arrays removed
      def compact_query_params(**kwargs)
        kwargs.reject { |_, value| value.nil? || value == [] }
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
      %i[get post put delete post_raw put_raw].each do |method|
        define_method(:"http_#{method}") do |*args, **kwargs, &block|
          @client.public_send(method, *args, **kwargs, &block)
        end
      end

      # Upload a file as multipart/form-data.
      # @param path [String] API path
      # @param io [IO, String] file data
      # @param filename [String] display filename
      # @param content_type [String] MIME type
      # @param field [String] form field name
      def http_put_multipart(path, io:, filename:, content_type:, field: "file")
        boundary = "BasecampSDK#{SecureRandom.hex(16)}"
        body = build_multipart_body(boundary: boundary, field: field, io: io, \
          filename: filename, content_type: content_type)
        http_put_raw(path, body: body, content_type: "multipart/form-data; boundary=#{boundary}")
        nil
      end

      private

      def build_multipart_body(boundary:, field:, io:, filename:, content_type:)
        data = io.respond_to?(:read) ? io.read : io.to_s
        safe_filename = filename.tr("\r\n", "").gsub("\\", "\\\\").gsub('"', '\\"')
        safe_content_type = content_type.tr("\r\n", "")
        body = "".b
        body << "--#{boundary}\r\n"
        body << "Content-Disposition: form-data; name=\"#{field}\"; filename=\"#{safe_filename}\"\r\n"
        body << "Content-Type: #{safe_content_type}\r\n"
        body << "\r\n"
        body << data
        body << "\r\n"
        body << "--#{boundary}--\r\n"
        body.force_encoding(Encoding::BINARY)
      end

      # Paginate doesn't conflict with service methods, keep as-is
      def paginate(...)
        @client.paginate(...)
      end

      # Paginate extracting items from a specific key (for object responses)
      def paginate_key(...)
        @client.paginate_key(...)
      end

      # Paginate a wrapped response extracting items from a specific key
      def paginate_wrapped(...)
        @client.paginate_wrapped(...)
      end
    end
  end
end
