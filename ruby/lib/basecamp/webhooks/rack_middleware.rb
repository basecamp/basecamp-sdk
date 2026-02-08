# frozen_string_literal: true

require_relative "receiver"

module Basecamp
  module Webhooks
    # Rack middleware that intercepts POST requests to a configurable path
    # and dispatches them to a WebhookReceiver for processing.
    class RackMiddleware
      DEFAULT_PATH = "/webhooks/basecamp"

      def initialize(app, receiver:, path: DEFAULT_PATH)
        @app = app
        @receiver = receiver
        @path = path
      end

      def call(env)
        unless env["PATH_INFO"] == @path
          return @app.call(env)
        end

        unless env["REQUEST_METHOD"] == "POST"
          return [ 405, { "Content-Type" => "text/plain" }, [ "Method Not Allowed" ] ]
        end

        body = env["rack.input"].read
        env["rack.input"].rewind

        headers = lambda { |name|
          # Rack normalizes headers to HTTP_UPPER_CASE format
          rack_key = "HTTP_#{name.upcase.tr('-', '_')}"
          env[rack_key]
        }

        begin
          @receiver.handle_request(raw_body: body, headers: headers)
          [ 200, { "Content-Type" => "text/plain" }, [ "OK" ] ]
        rescue VerificationError
          [ 401, { "Content-Type" => "text/plain" }, [ "Unauthorized" ] ]
        rescue JSON::ParserError
          [ 400, { "Content-Type" => "text/plain" }, [ "Bad Request" ] ]
        rescue StandardError
          [ 500, { "Content-Type" => "text/plain" }, [ "Internal Server Error" ] ]
        end
      end
    end
  end
end
