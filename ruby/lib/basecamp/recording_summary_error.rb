# frozen_string_literal: true

module Basecamp
  # Base class for the identities +RecordingsExtensions#summarize+ raises.
  #
  # These are the composite's OWN outcomes, not HTTP answers: "this pointer
  # names no recording type", "this chat line is under no Campfire you can
  # currently see", "discovery could not be carried to a conclusion", "the read
  # came back from another bucket". A consumer matches the CLASS — or {#kind},
  # the same vocabulary the conformance fixture pins — rather than parsing the
  # message or reading a status off it.
  #
  # +code+ stays inside SPEC section 6's closed taxonomy, which is the
  # HTTP-status mapping and has no room for composite identities. Each subclass
  # documents the canonical code it chose and why; the code is the coarse
  # category a CLI exits on, the class is the identity.
  class RecordingSummaryError < Error
    # The composite identity, in the conformance fixture's vocabulary
    # (+no_recording_type+, +unknown_recording_type+, +recording_unresolved+,
    # +campfire_discovery_incomplete+, +bucket_mismatch+).
    # @return [String]
    attr_reader :kind

    # @param kind [String] the composite identity
    # @param message [String]
    # @param code [String] a canonical {ErrorCode}
    # @param hint [String, nil]
    def initialize(kind:, message:, code:, hint: nil)
      super(code: code, message: message, hint: hint)
      @kind = kind
    end
  end
end
