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
      # BOUNDED, like the argument reader above. The fields this serves are
      # plain 64-bit integers in the reference, so a number outside that range
      # is a decode failure there — 2**63 failed the read and was returned
      # verbatim here, which the doc above already claimed was impossible.
      return value if value.is_a?(Integer) && value.between?(MIN, MAX)
      return 0 if value.nil?

      nil
    end

    # A PERSON's id, which the reference decodes flexibly rather than strictly.
    #
    # +Person.Id+ is the single field in the generated model typed as the
    # flexible decoder, so the rules differ from {from_wire} in exactly the way
    # that method's own doc warns about:
    #
    # * an Integer in range, or an absent value, behaves as everywhere else;
    # * a STRING of digits is the integer it spells, since the decoder hands it
    #   to ParseInt — which takes a leading sign;
    # * any OTHER string is 0 and not an error, because that is the sentinel the
    #   API serves for system-generated entities ("basecamp");
    # * a string whose digits overflow is a range error, so it fails the read;
    # * anything else — a float, a boolean, an array, an object — is a decode
    #   failure, as it is for every id.
    #
    # @param value [Object] as it arrived on the wire
    # @return [Integer, nil] the id, 0 for absent or a non-numeric sentinel,
    #   nil when the reference could not have decoded it
    def person_from_wire(value)
      return value if value.is_a?(Integer) && value.between?(MIN, MAX)
      # NULL IS NOT ABSENT here, and that is the one place these two readers
      # differ on nil. encoding/json calls the flexible decoder's own
      # UnmarshalJSON for a null, its number path leaves the buffer empty, and
      # ParseInt("") fails — so {"id": null} FAILS THE READ while a missing
      # "id" is the zero value with no error. A plain int64 field has no
      # UnmarshalJSON, so json handles its null itself and both are 0, which is
      # why {from_wire} may treat them alike and this may not. Measured through
      # the real decode path; an earlier version of this method returned 0 here
      # and a test pinned that, which made the wrong rule harder to see rather
      # than easier.
      #
      # The caller distinguishes them: an absent key never reaches this.
      return nil if value.nil?
      return nil unless value.is_a?(String)

      digits = value.b
      return 0 unless digits.match?(/\A[-+]?\d+\z/n)

      parsed = digits.to_i
      parsed.between?(MIN, MAX) ? parsed : nil
    end
  end
end
