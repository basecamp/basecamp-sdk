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
        @dedup_seen = {}
        @dedup_pending = {}
        @dedup_order = []
        @mutex = Mutex.new
      end

      # Register a handler for a specific event kind pattern.
      # Supports glob patterns: "todo_*" matches "todo_created", etc.
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

        # Atomic dedup: claim before handlers, commit on success, release on error
        return event unless claim(event.id)

        begin
          # Build middleware chain
          run_handlers = -> { dispatch_handlers(event) }
          chain = @middleware.reverse.reduce(run_handlers) do |next_fn, mw|
            -> { mw.call(event, next_fn) }
          end

          chain.call

          # Promote from pending to seen on success
          commit_seen(event.id)
        rescue => e
          # Release claim so retries can re-attempt
          release_claim(event.id)
          raise e
        end

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

      # Returns true if the event was claimed (caller should process it).
      # Returns false if already seen or in-flight.
      def claim(event_id)
        return true if @dedup_window_size <= 0 || event_id.nil?

        @mutex.synchronize do
          return false if @dedup_seen.key?(event_id) || @dedup_pending.key?(event_id)
          @dedup_pending[event_id] = true
          true
        end
      end

      # Promote from pending to seen after successful handling.
      def commit_seen(event_id)
        return if @dedup_window_size <= 0 || event_id.nil?

        @mutex.synchronize do
          @dedup_pending.delete(event_id)

          if @dedup_order.size >= @dedup_window_size
            oldest = @dedup_order.shift
            @dedup_seen.delete(oldest)
          end

          @dedup_seen[event_id] = true
          @dedup_order << event_id
        end
      end

      # Release claim so retries can re-attempt.
      def release_claim(event_id)
        return if event_id.nil?

        @mutex.synchronize do
          @dedup_pending.delete(event_id)
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
