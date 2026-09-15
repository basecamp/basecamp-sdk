# frozen_string_literal: true

module Basecamp
  # Reads a documented integer id argument.
  #
  # The composites take ids the caller holds — a bucket, a recording, the people
  # to mention — and put them straight into a read. Coercing with +to_i+ would
  # turn "12oops" and 12.9 into 12 and go and fetch THAT record, which is a
  # wrong answer wearing the shape of a right one. The generated services do not
  # coerce either: they interpolate what they are given, so a malformed id fails
  # as a bad path rather than as a different record.
  #
  # An Integer passes through. A string of DIGITS is accepted, since an id read
  # out of JSON or off a command line is a reasonable thing to hold. Anything
  # else — a float, a partly numeric string, nil — is a usage error, named so
  # the caller can see which argument it was.
  #
  # "A string of digits" is matched as such, not handed to +Integer()+, whose
  # grammar is Ruby's integer literal: it reads "1_2" as 12, accepts a sign and
  # surrounding whitespace, and would put this method back to fetching a record
  # the caller did not ask for. The value is bounded like the API's own ids, so
  # a number too large to be one is refused rather than sent.
  module Ids
    # The largest id the API can carry, matching a signed 64-bit integer.
    MAX = (2**63) - 1

    module_function

    # @param value [Object] the id as the caller supplied it
    # @param name [String] the argument's name, for the error message
    # @return [Integer]
    # @raise [Basecamp::UsageError] when the value is not an integer id
    def integer(value, name)
      id = value if value.is_a?(Integer)
      id ||= value.to_i if value.is_a?(String) && value.match?(/\A\d+\z/)
      raise UsageError.new("#{name} must be an integer, got #{value.inspect}") if id.nil?
      raise UsageError.new("#{name} is out of range: #{value.inspect}") if id.abs > MAX

      id
    end
  end
end
