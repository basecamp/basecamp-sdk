# frozen_string_literal: true

module Basecamp
  module Oauth
    # RFC 9728 protected-resource metadata (hop 1 of resource-first discovery).
    #
    # +authorization_servers+ preserves "key absent" and "present but empty
    # <tt>[]</tt>" distinctly: BC5 omits the key while dark, per RFC 9728 §3.2.
    # Both select Launchpad, but the distinction is meaningful to callers
    # inspecting metadata directly (absent => +nil+, empty => +[]+).
    #
    # @attr resource [String] The resource identifier; equals the requested
    #   resource origin by code-point.
    # @attr authorization_servers [Array<String>, nil] Advertised authorization
    #   servers; +nil+ when the key is absent, +[]+ when present but empty.
    ProtectedResourceMetadata = Data.define(:resource, :authorization_servers) do
      def initialize(resource:, authorization_servers: nil)
        super
      end
    end
  end
end
