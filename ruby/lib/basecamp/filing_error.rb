# frozen_string_literal: true

module Basecamp
  # A move or copy that {Services::RecordingsExtensions#move_and_wait} or
  # +#copy_and_wait+ could not complete (SPEC.md Appendix F, Filing Waits).
  # {#kind} says how far it got; only a {FilingRefusedError} is safe to try
  # again. Like Go's, these are not {Basecamp::Error}s: the failure that caused
  # one is its +cause+, so a retryable error inside one that may have filed
  # cannot read as permission to send the start again.
  class FilingError < StandardError
    # @return [String] the fixture vocabulary's name for this outcome
    def kind
      self.class::KIND
    end
  end
end
