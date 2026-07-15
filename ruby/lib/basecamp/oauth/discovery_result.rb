# frozen_string_literal: true

module Basecamp
  module Oauth
    # Result of {Oauth.discover_from_resource}: either a *selected* AS config, or
    # a *soft* fallback to Launchpad. Hard failures are raised as
    # {DiscoverySelectionError}, never represented here — so no consumer can
    # convert a hard failure into a Launchpad request.
    #
    # @attr kind [Symbol] +:selected+ or +:fallback+
    # @attr config [Config, nil] the selected AS config (when +:selected+)
    # @attr issuer [String, nil] the selected issuer (when +:selected+)
    # @attr reason [String, nil] the soft fallback reason (when +:fallback+):
    #   +"resource_discovery_failed"+ or +"no_as_advertised"+
    DiscoveryResult = Data.define(:kind, :config, :issuer, :reason) do
      def self.selected(config)
        new(kind: :selected, config: config, issuer: config.issuer, reason: nil)
      end

      def self.fallback(reason)
        new(kind: :fallback, config: nil, issuer: nil, reason: reason)
      end

      def selected?
        kind == :selected
      end

      def fallback?
        kind == :fallback
      end
    end
  end
end
