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

    # The most digits a signed 64-bit value can carry, so a longer run is out of
    # range without converting it.
    MAX_DIGITS = MAX.to_s.length

    # The magnitude ParseUint accumulates into before ParseInt applies its own
    # bound: the scan refuses at UNSIGNED 64-bit, not at MAX (strconv/atoi.go,
    # ParseUint's loop). See {parse_int}, where that one boundary is the whole
    # difference between a syntax refusal and a range refusal.
    U64_MAX = (2**64) - 1

    PLUS = "+".ord
    MINUS = "-".ord
    ZERO = "0".ord
    private_constant :PLUS, :MINUS, :ZERO

    module_function

    # A decimal string as an Integer, :overflow when it is out of range, or
    # :not_decimal when it is not one.
    #
    # BOUNDED LEXICALLY BEFORE CONVERTING, which is the whole point. Every
    # caller used to run to_i and then range-check the result, so a digit run
    # from a response body or from rich text built an arbitrarily large Integer
    # first — 5,000,000 digits measured at 2.5 seconds, and a body may be 50 MB.
    # An int64 is at most #{MAX_DIGITS} digits, so anything longer is out of
    # range and can be rejected by LENGTH. Leading zeros are stripped before
    # that test, since they carry no magnitude.
    #
    # Five call sites had this shape and a review found three; the other two
    # were the caller-argument reader and the sgid decoder, which is the one fed
    # by rich text other people wrote.
    #
    # LEXICAL FIRST, THEN BOUNDED — which is a different rule from the one
    # {parse_int} implements, and deliberately so: this one asks "is the whole
    # string a decimal?" before it asks "does it fit?", so junk anywhere makes
    # it :not_decimal however large the digits are. The sites that read a
    # caller's argument, a header count, a Retry-After and an sgid's id want
    # exactly that. The two sites that read a PERSON id off the wire do not,
    # because Go's scan decides in the other order; they call {parse_int}.
    #
    # @param value [String]
    # @param signed [Boolean] whether a leading "+" or "-" is allowed
    def bounded_decimal(value, signed: true)
      digits = value.b
      return :not_decimal unless digits.match?(signed ? /\A[-+]?\d+\z/n : /\A\d+\z/n)

      magnitude = digits.sub(/\A[-+]/, "").sub(/\A0+(?=\d)/, "")
      return :overflow if magnitude.length > MAX_DIGITS

      parsed = digits.to_i
      parsed.between?(MIN, MAX) ? parsed : :overflow
    end

    # <tt>strconv.ParseInt(s, 10, 64)</tt>, scan order included: the Integer the
    # string spells, :syntax when Go reports ErrSyntax, or :range when it
    # reports ErrRange.
    #
    # The two refusals are kept APART because the reference does two different
    # things with them, and no single "is this a number?" predicate can tell
    # them apart: ErrSyntax is the "basecamp" sentinel and reads 0
    # (go/pkg/types/flexible_int64.go:46), ErrRange fails the read
    # (flexible_int64.go:43). Which one a string earns depends on where in it
    # the first disqualifying byte sits, so the answer is a property of the
    # SCAN, not of the string's shape.
    #
    # The grammar: one optional ASCII "+" or "-", then one or more ASCII
    # digits, and nothing else. No surrounding whitespace (Go trims none, so
    # " 7" is ErrSyntax and reads 0), no "_" separator (only base 0 allows one),
    # and ASCII digits alone — a fullwidth "７" or an Arabic-Indic "٧" is not a
    # digit. Ruby happens to agree on that last one, since both /\d/ and
    # Integer() are ASCII-only here, but the rule is Go's rather than Ruby's:
    # a \p{Nd}-aware rewrite would accept ids the reference calls sentinels,
    # which is how the other ports of this scan drifted.
    #
    # THE SUBTLETY THAT COSTS THE HAND-ROLLED LOOP: ParseInt delegates the
    # magnitude to ParseUint, which checks it INSIDE the scan and returns
    # ErrRange the instant the accumulator would overflow uint64 — before it
    # ever reaches the rest of the string. The first disqualifying byte wins,
    # and the boundary it wins at is u64, not int64. So one digit decides which
    # refusal a malformed id earns:
    #
    #   "18446744073709551615x"  digits still fit u64, the scan reaches the
    #                            "x"                      -> :syntax, reads 0
    #   "18446744073709551616x"  the overflow fires first -> :range, fails
    #
    # Testing the whole string for well-formedness first — which is what
    # {bounded_decimal} does, correctly, for its own callers — gets that pair
    # backwards and hands a malformed oversized id to the reader as the
    # "basecamp" system actor. Measured against the reference on both rows.
    #
    # NOT the rule the sgid's person id gets. That one walks the bytes and
    # refuses anything outside 0..9 BEFORE parsing (go/pkg/basecamp/mentions.go:
    # 252-256), so it rejects the leading "+" this one accepts, and
    # {Basecamp::Mentions.parse_global_id} carries that walk because the
    # reference has that shape at THAT site. Two co-resident rules, deliberately
    # different: do not hoist either into the other, in either direction —
    # unifying them here would accept "+77" as a mentioned person again, and
    # unifying them there would refuse a "+7" the people read accepts.
    #
    # Bounded by construction, so it needs no length gate: the accumulator
    # passes u64 within 20 digits and every other byte ends the scan, so no
    # input builds a large Integer however long it is. Read by BYTES for the
    # same reason {bounded_decimal} is — a String carrying invalid UTF-8 makes
    # the regexp engine raise, and +getbyte+ neither raises nor copies the
    # string, which matters on a body that may be 50 MB.
    #
    # @param value [String]
    # @return [Integer, Symbol] the value, :syntax, or :range
    def parse_int(value)
      index = 0
      negative = false
      case value.getbyte(0)
      when PLUS then index = 1
      when MINUS then index, negative = 1, true
      end
      # An empty digit run, after a sign or without one, is ErrSyntax.
      return :syntax if index == value.bytesize

      # ParseUint's loop, byte for byte.
      magnitude = 0
      while index < value.bytesize
        digit = value.getbyte(index) - ZERO
        return :syntax unless digit.between?(0, 9)

        magnitude = (magnitude * 10) + digit
        return :range if magnitude > U64_MAX

        index += 1
      end

      # ParseInt's own bound, applied to what ParseUint returned: a negative may
      # carry 2**63, which is MIN, and a positive may carry MAX.
      if negative
        magnitude > -MIN ? :range : -magnitude
      else
        magnitude > MAX ? :range : magnitude
      end
    end

    # @param value [Object] the id as the caller supplied it
    # @param name [String] the argument's name, for the error message
    # @return [Integer]
    # @raise [Basecamp::UsageError] when the value is not an integer id
    def integer(value, name)
      id = value if value.is_a?(Integer)
      # Matched on BYTES: a String carrying invalid UTF-8 makes the regexp
      # engine raise ArgumentError, and an id that is not a number is a usage
      # error naming the argument, not an exception out of a public method.
      if id.nil? && value.is_a?(String)
        parsed = bounded_decimal(value, signed: false)
        raise UsageError.new("#{name} is out of range: #{value.inspect}") if parsed == :overflow

        id = parsed unless parsed == :not_decimal
      end
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
    # * a string whose digits overflow is a range error, so it fails the read,
    #   and so is one whose digits overflow BEFORE the junk that follows them
    #   ("18446744073709551616x"), because ParseUint refuses the magnitude
    #   inside the scan — the same junk one digit earlier reads 0;
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

      # Go's scan rather than a lexical test, because the reference's two
      # refusals do not partition the string the way a regexp does: an
      # oversized digit run followed by junk is a RANGE error there and fails
      # the read, while the same junk one digit earlier is a syntax error and
      # reads 0. See {parse_int}.
      parsed = parse_int(value)
      return 0 if parsed == :syntax
      return nil if parsed == :range

      parsed
    end
  end
end
