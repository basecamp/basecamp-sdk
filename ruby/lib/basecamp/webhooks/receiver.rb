# frozen_string_literal: true

require "json"
require_relative "event"
require_relative "verify"

module Basecamp
  module Webhooks
    class VerificationError < StandardError; end

    # Receives and routes webhook events from Basecamp.
    # Framework-agnostic: works with raw body strings and a header accessor.
    class Receiver
      DEFAULT_SIGNATURE_HEADER = "X-Basecamp-Signature"
      DEFAULT_DEDUP_WINDOW_SIZE = 1000

      def initialize(secret: nil, signature_header: DEFAULT_SIGNATURE_HEADER, dedup_window_size: DEFAULT_DEDUP_WINDOW_SIZE)
        @secret = secret
        @signature_header = signature_header
        @dedup_window_size = dedup_window_size
        @handlers = {}
        @any_handlers = []
        @middleware = []
        @dedup_set = {}
        @dedup_order = []
        @mutex = Mutex.new
      end

      # Register a handler for a specific event kind pattern.
      # Supports glob patterns: "todo.*" matches "todo_created", etc.
      def on(pattern, &handler)
        @handlers[pattern] ||= []
        @handlers[pattern] << handler
        self
      end

      # Register a handler for all events.
      def on_any(&handler)
        @any_handlers << handler
        self
      end

      # Add middleware to the processing chain.
      # Middleware receives (event, next_proc) and must call next_proc.call to continue.
      def use(&middleware)
        @middleware << middleware
        self
      end

      # Process a raw webhook request.
      # Returns the parsed Event.
      # Raises VerificationError if signature is invalid.
      def handle_request(raw_body:, headers:)
        # Verify signature
        if @secret && !@secret.empty?
          signature = extract_header(headers, @signature_header)
          unless Verify.valid?(payload: raw_body, signature: signature, secret: @secret)
            raise VerificationError, "invalid webhook signature"
          end
        end

        # Parse event
        hash = JSON.parse(raw_body)
        event = Event.new(hash)

        # Dedup check
        return event if duplicate?(event.id)

        # Build middleware chain
        run_handlers = -> { dispatch_handlers(event) }
        chain = @middleware.reverse.reduce(run_handlers) do |next_fn, mw|
          -> { mw.call(event, next_fn) }
        end

        chain.call
        event
      end

      private

      def extract_header(headers, name)
        if headers.respond_to?(:call)
          headers.call(name)
        elsif headers.respond_to?(:[])
          # Try exact match first, then case-insensitive
          headers[name] || headers[name.downcase] || headers[name.upcase]
        end
      end

      def duplicate?(event_id)
        return false if @dedup_window_size <= 0 || event_id.nil?

        @mutex.synchronize do
          return true if @dedup_set.key?(event_id)

          if @dedup_order.size >= @dedup_window_size
            oldest = @dedup_order.shift
            @dedup_set.delete(oldest)
          end

          @dedup_set[event_id] = true
          @dedup_order << event_id
          false
        end
      end

      def dispatch_handlers(event)
        matched = []

        @handlers.each do |pattern, handlers|
          matched.concat(handlers) if match_pattern?(pattern, event.kind)
        end

        matched.concat(@any_handlers)

        matched.each { |handler| handler.call(event) }
      end

      def match_pattern?(pattern, value)
        return false if value.nil?
        return true if pattern == value

        # Convert glob pattern to regex
        regex_str = pattern.split("*", -1).map { |part| Regexp.escape(part) }.join(".*")
        Regexp.new("\\A#{regex_str}\\z").match?(value)
      end
    end
  end
end
