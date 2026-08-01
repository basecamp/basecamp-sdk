# frozen_string_literal: true

# Go type spellings the Ruby generator needs to reason about, extracted so the
# generator and its tests can share one definition — the test used to `load`
# the whole generator script, which defined every one of its helpers on the
# test process.
module GoTypeSpellings
  module_function

  # Spellings that mean "full timestamp" and therefore get Time coercion in
  # Ruby. Matched after stripping a leading `*`: the Go optional-pointer policy
  # (SPEC.md §10) means a schema may carry either `time.Time` or `*time.Time`
  # for the same wire contract, and an exact-string match silently degrades the
  # pointer spelling to a raw String (#537).
  #
  # types.FlexibleTime is deliberately NOT here: it also accepts date-only
  # values, and Ruby has passed those through as strings since it was
  # introduced. Adding it is a behavior change, not a spelling fix.
  TIMESTAMP_GO_TYPES = [ 'time.Time' ].freeze

  def timestamp_go_type?(go_type)
    return false unless go_type.is_a?(String)

    TIMESTAMP_GO_TYPES.include?(go_type.delete_prefix('*'))
  end
end
