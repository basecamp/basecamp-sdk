# frozen_string_literal: true

module Basecamp
  module Oauth
    # RFC 8628 device authorization response (the device/user code pair the user
    # approves out of band).
    #
    # @attr device_code [String] The device verification code polled at the token
    #   endpoint.
    # @attr user_code [String] The code the user enters at the verification URI.
    # @attr verification_uri [String] Where the user goes to approve the request.
    # @attr verification_uri_complete [String, nil] The verification URI with the
    #   user code pre-filled (optional; absent when the server omits it).
    # @attr expires_in [Integer] Lifetime of the device/user codes in seconds.
    # @attr interval [Integer] Minimum seconds to wait between token polls
    #   (defaults to 5 when the server omits it).
    DeviceAuthorization = Data.define(
      :device_code,
      :user_code,
      :verification_uri,
      :verification_uri_complete,
      :expires_in,
      :interval
    ) do
      def initialize(
        device_code:,
        user_code:,
        verification_uri:,
        expires_in:,
        interval:,
        verification_uri_complete: nil
      )
        super
      end
    end
  end
end
