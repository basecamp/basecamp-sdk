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
    # The range a signed 64-bit id can carry, which is what the API's own ids
    # are. Shared with {Basecamp::Mentions}, which bounds a person id decoded
    # out of an sgid the same way.
    MAX = (2**63) - 1
    MIN = -(2**63)

    module_function

    # @param value [Object] the id as the caller supplied it
    # @param name [String] the argument's name, for the error message
    # @return [Integer]
    # @raise [Basecamp::UsageError] when the value is not an integer id
    def integer(value, name)
      id = value if value.is_a?(Integer)
      # Matched on BYTES: a String carrying invalid UTF-8 makes the regexp
      # engine raise ArgumentError, and an id that is not a number is a usage
      # error naming the argument, not an exception out of a public method.
      digits = value.b if value.is_a?(String)
      id ||= digits.to_i if digits&.match?(/\A\d+\z/n)
      raise UsageError.new("#{name} must be an integer, got #{value.inspect}") if id.nil?
      raise UsageError.new("#{name} is out of range: #{value.inspect}") unless id.between?(MIN, MAX)

      id
    end

    # Reads an id off a value the API sent. An Integer passes through; a string
    # of digits is read as one; ABSENT reads as 0; anything else is MALFORMED
    # and returns nil.
    #
    # The three-way answer is the point, and an earlier version got it wrong by
    # collapsing malformed into absent. It exists because +to_i+ is not defined
    # on every JSON shape — <tt>{"bucket": {"id": []}}</tt> is valid JSON and
    # turned a projection into a NoMethodError — but the reference decodes into
    # a typed integer, so a payload like that fails the READ there. Reading it
    # as absent instead let it through silently, and in the one place that
    # matters it disarmed a check: a bucket id of 0 is not compared, so a
    # recording from another bucket would have been returned. Loud was bad;
    # silent was worse.
    #
    # @param value [Object] as it arrived on the wire
    # @return [Integer, nil] the id, 0 when absent, nil when malformed
    def from_wire(value)
      return value if value.is_a?(Integer)
      return 0 if value.nil?
      return value.to_i if value.is_a?(String) && value.b.match?(/\A\d+\z/n)

      nil
    end
  end
end
