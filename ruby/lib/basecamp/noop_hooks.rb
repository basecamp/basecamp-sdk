# frozen_string_literal: true

module Basecamp
  # No-op implementation of Hooks.
  # Used as the default when no hooks are configured.
  class NoopHooks
    include Hooks
  end
end
