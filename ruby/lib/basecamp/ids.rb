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

    # Reads an id off a value the API sent: the Integer itself, 0 when the value
    # is absent, and nil when it is anything else.
    #
    # Nothing is coerced, and a string of digits is NOT an id. At every field
    # this reads — a bucket id, a dock item's id, a listed Campfire's id and its
    # bucket's id — the reference holds a plain 64-bit integer, so a JSON
    # string, float, boolean, array or object there is a DECODE error that fails
    # the read. This tier has no decoder, which is precisely why the check has
    # to be explicit (the same reason {Basecamp::Services::MergeSafe} exists).
    # An earlier version accepted digit strings on an argument about
    # deduplicating ids across two sources; that argument was written in a
    # comment and was never true of these fields.
    #
    # NOT "every id in the API", which an earlier version of this paragraph
    # claimed. A PERSON's id is the one exception in the whole generated model:
    # it is decoded flexibly, so the reference takes <tt>"7"</tt> as 7 and
    # <tt>"basecamp"</tt> — the sentinel it serves for system-generated
    # entities — as 0, neither of them an error. Nothing routed through here
    # reads a person id today. Anything that starts to must not reach for this
    # method, because it would refuse a body the reference accepts.
    #
    # The caller turns nil into a malformed-response error. It is not raised
    # here because the message belongs to the field, not to this reader.
    #
    # @param value [Object] as it arrived on the wire
    # @return [Integer, nil] the id, 0 when absent, nil when malformed
    def from_wire(value)
      return value if value.is_a?(Integer)
      return 0 if value.nil?

      nil
    end
  end
end
