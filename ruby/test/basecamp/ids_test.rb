# frozen_string_literal: true

require "test_helper"

# Tests for Basecamp::Ids — the three id readers the composites decode with.
#
# This file had no test at all until a sweep for "which changed lib files have
# no test of their own" found it, prompted by the same gap in Basecamp::Text.
# Every rule in it was reachable only through a composite, so four separate
# mutations survived the whole suite: removing the int64 bound from from_wire,
# turning a non-numeric person id into a decode failure instead of 0, dropping
# the overflow check, and refusing a leading "+".
#
# Three of those four were added in the commit that introduced them, verified
# by an ad-hoc script and not by anything that runs again.
#
# The expected values are the REFERENCE's. That sentence used to end there, and
# it was wrong at one row — person_from_wire(nil) was asserted as 0 when the
# reference fails the read — so it now says how to re-run the measurement
# instead of asking to be believed:
#
#   go/pkg/types/flexible_int64.go is the decoder for a person id;
#   every other id in go/pkg/generated/client.gen.go is a plain int64.
#   json.Unmarshal(`{"id":null}`) into a struct whose field is
#   types.FlexibleInt64 returns an error: its number path decodes null into an
#   empty json.Number and Int64() then fails. The same input into a plain int64
#   field returns 0 with no error. An ABSENT key is 0 for both.
#
# The property that decides is what the type's decoder DOES with a null, not
# whether it has one. time.Time also carries an UnmarshalJSON and explicitly
# accepts null — a hundred fields in this model are non-pointer time.Time — so
# "carries a hook" is the wrong test and a sentence here once said it.
#
# The asymmetry is still a property of the TYPE rather than of the name, so a
# port unifying null-handling across id fields is wrong for one of the two
# kinds whichever way it chooses.
#
# A header claiming "measured against the reference" is itself a claim, and the
# most load-bearing one in a file of expectations.
class IdsTest < Minitest::Test
  include TestHelper

  MAX = (2**63) - 1
  MIN = -(2**63)

  # --- Ids.integer: the caller's own argument ------------------------------

  def test_integer_takes_an_integer_or_a_string_of_digits
    assert_equal 12, Basecamp::Ids.integer(12, "id")
    assert_equal 12, Basecamp::Ids.integer("12", "id")
    assert_equal 0, Basecamp::Ids.integer(0, "id")
    assert_equal(-5, Basecamp::Ids.integer(-5, "id"))
  end

  def test_integer_refuses_anything_it_would_have_to_coerce
    # Coercing would fetch a different record — a wrong answer wearing the shape
    # of a right one. "1_2" is Ruby's integer-literal grammar, not a string of
    # digits, and Integer() would read it as 12.
    [ "12oops", 12.9, nil, "", [ 12 ], "1_2", " 12 ", "+12", "-12", {},
      true, (2**63).to_s, "0x10" ].each do |value|
      assert_raises(Basecamp::UsageError, "an argument of #{value.inspect}") do
        Basecamp::Ids.integer(value, "id")
      end
    end
  end

  def test_integer_is_bounded_like_the_apis_own_ids
    assert_equal MAX, Basecamp::Ids.integer(MAX, "id")
    assert_equal MIN, Basecamp::Ids.integer(MIN, "id")
    assert_raises(Basecamp::UsageError) { Basecamp::Ids.integer(MAX + 1, "id") }
    assert_raises(Basecamp::UsageError) { Basecamp::Ids.integer(MIN - 1, "id") }
  end

  # --- Ids.from_wire: every id the reference types as a plain int64 ---------

  def test_from_wire_takes_an_integer_and_reads_absent_as_zero
    assert_equal 12, Basecamp::Ids.from_wire(12)
    assert_equal 0, Basecamp::Ids.from_wire(nil)
    assert_equal 0, Basecamp::Ids.from_wire(0)
    assert_equal(-5, Basecamp::Ids.from_wire(-5))
  end

  def test_from_wire_refuses_everything_the_reference_cannot_decode
    # A string is NOT an id here, whatever it spells: these fields are plain
    # int64 in the reference, so a JSON string is a decode failure.
    [ "12", "", "abc", 12.5, 12.0, true, false, [], [ 12 ], {}, { "id" => 1 } ].each do |value|
      assert_nil Basecamp::Ids.from_wire(value), "a wire value of #{value.inspect}"
    end
  end

  def test_from_wire_is_bounded_like_the_reference
    # 2**63 fails the read there and was returned verbatim here. The argument
    # reader in the same file has been bounded since its first commit; this one
    # was not, and nothing noticed.
    assert_equal MAX, Basecamp::Ids.from_wire(MAX)
    assert_equal MIN, Basecamp::Ids.from_wire(MIN)
    assert_nil Basecamp::Ids.from_wire(MAX + 1)
    assert_nil Basecamp::Ids.from_wire(MIN - 1)
  end

  # --- Ids.person_from_wire: the one flexibly-decoded id -------------------

  def test_person_from_wire_takes_an_integer_or_a_digit_string
    # Person.Id is the single field in the generated model typed as the
    # flexible decoder, which hands a string to ParseInt — so "7" is 7 there,
    # and a leading sign is accepted because ParseInt accepts one.
    assert_equal 7, Basecamp::Ids.person_from_wire(7)
    assert_equal 7, Basecamp::Ids.person_from_wire("7")
    assert_equal 7, Basecamp::Ids.person_from_wire("+7")
    assert_equal(-7, Basecamp::Ids.person_from_wire("-7"))
    assert_equal 7, Basecamp::Ids.person_from_wire("007")
  end

  def test_a_null_person_id_fails_the_read_where_an_absent_one_is_zero
    # THE one field where absent and null differ. encoding/json calls the
    # flexible decoder's own UnmarshalJSON for a null, its number path leaves
    # the buffer empty, and ParseInt("") fails — so {"id": null} fails the read
    # while a missing "id" is the zero value. A plain int64 has no
    # UnmarshalJSON, so json handles its null itself and both are 0.
    #
    # This row is here because the first version of this file asserted the
    # opposite, which made the wrong rule harder to see rather than easier: a
    # test is a claim with more authority than a comment, and it was wrong.
    assert_nil Basecamp::Ids.person_from_wire(nil)
    assert_equal 0, Basecamp::Ids.from_wire(nil)
  end

  def test_a_non_numeric_person_id_is_zero_rather_than_a_failure
    # "basecamp" is the sentinel the API serves for system-generated entities,
    # and the reference reads it as 0 with NO error. Treating it as malformed
    # kept a creator the reference drops.
    [ "basecamp", "", "abc", "1_2", " 7", "7 ", "7\n", "\n7", "7\t", "0x10", "７", "+", "-",
      "١٢" ].each do |value|
      assert_equal 0, Basecamp::Ids.person_from_wire(value), "a person id of #{value.inspect}"
    end
  end

  def test_a_person_id_that_overflows_fails_the_read
    # The decoder distinguishes a numeric overflow, which it reports, from a
    # non-numeric sentinel, which it zeroes.
    assert_equal MAX, Basecamp::Ids.person_from_wire(MAX)
    assert_equal MAX, Basecamp::Ids.person_from_wire(MAX.to_s)
    assert_nil Basecamp::Ids.person_from_wire(MAX + 1)
    assert_nil Basecamp::Ids.person_from_wire((MAX + 1).to_s)
    assert_nil Basecamp::Ids.person_from_wire((MIN - 1).to_s)
  end

  def test_the_measured_go_corpus_at_the_flexible_reader
    # All 74 rows of GoPersonIds::CORPUS through the real reader. The reader
    # cannot tell a value of 0 from a sentinel — the reference returns 0 for
    # both, which is exactly why the sentinel is safe — so :label expects 0
    # here and the three-way verdict is pinned at parse_int below.
    GoPersonIds::CORPUS.each do |wire, expected|
      want =
        case expected
        when :label then 0
        when :refuse then nil
        else expected.last
        end

      assert_equal want.inspect, Basecamp::Ids.person_from_wire(wire).inspect,
                   "person_from_wire(#{wire.inspect})"
    end
  end

  # --- Ids.parse_int: strconv.ParseInt(s, 10, 64), scan order included -----

  def test_the_measured_go_corpus_at_the_scan
    # The same 74 rows against the scan itself, where the three verdicts are
    # still distinct: a value, ErrSyntax (:syntax, which the two person-id
    # sites read as the "basecamp" sentinel) and ErrRange (:range, which they
    # refuse). Both sites route through this one function, so this is the row
    # that fails first if the rule drifts.
    GoPersonIds::CORPUS.each do |wire, expected|
      want =
        case expected
        when :label then :syntax
        when :refuse then :range
        else expected.last
        end

      assert_equal want.inspect, Basecamp::Ids.parse_int(wire).inspect, "parse_int(#{wire.inspect})"
    end
  end

  def test_which_refusal_comes_first_is_decided_by_the_scan_and_not_by_the_shape
    # ParseUint checks the magnitude INSIDE its loop and returns ErrRange the
    # instant the accumulator would overflow uint64 — before the scan reaches
    # the junk. So one digit separates a sentinel from a failed read, and the
    # boundary is u64, not int64. A port that tests the whole string for
    # well-formedness first gets this pair backwards, which is the defect this
    # method exists to fix: an oversized malformed id silently became the
    # "basecamp" system actor.
    assert_equal :syntax, Basecamp::Ids.parse_int("18446744073709551615x")
    assert_equal :range, Basecamp::Ids.parse_int("18446744073709551616x")

    # And a lexical rule cannot express that, which is why both rules live
    # here rather than one: bounded_decimal answers the SAME thing for both,
    # correctly for its own callers, and neither may be hoisted into the other.
    assert_equal :not_decimal, Basecamp::Ids.bounded_decimal("18446744073709551615x")
    assert_equal :not_decimal, Basecamp::Ids.bounded_decimal("18446744073709551616x")
  end

  def test_the_scan_bounds_itself_without_a_length_gate
    # No length gate here and none needed: the accumulator passes uint64 within
    # twenty digits, so the scan answers without ever building the number. On
    # the clock, because that is the only observable difference — every answer
    # is identical with and without it (see the same argument at
    # bounded_decimal's gate, which needs one because it converts the run).
    digits = "9" * 5_000_000

    elapsed = Process.clock_gettime(Process::CLOCK_MONOTONIC)
    assert_equal :range, Basecamp::Ids.parse_int(digits)
    assert_equal :syntax, Basecamp::Ids.parse_int("x#{digits}")
    elapsed = Process.clock_gettime(Process::CLOCK_MONOTONIC) - elapsed

    assert_operator elapsed, :<, 1.0,
      "five million digits took #{elapsed.round(3)}s — the scan is accumulating past uint64 " \
      "instead of refusing at it"
  end

  def test_an_empty_digit_run_is_a_syntax_error_however_it_is_spelled
    # Unobservable through either consumer, since both read a syntax refusal as
    # 0 and an empty run would accumulate 0 anyway — but it is what ParseInt
    # does, and it matters the moment a third caller tells the two apart.
    assert_equal :syntax, Basecamp::Ids.parse_int("")
    assert_equal :syntax, Basecamp::Ids.parse_int("+")
    assert_equal :syntax, Basecamp::Ids.parse_int("-")
  end

  def test_the_scan_reads_bytes_rather_than_characters
    # A String carrying invalid UTF-8 reaches this from a response body, and
    # the regexp engine raises ArgumentError on one. The scan compares bytes,
    # so a broken encoding is just a non-digit byte: a syntax refusal, not an
    # exception out of a public method.
    assert_equal :syntax, Basecamp::Ids.parse_int("7\xC3".dup.force_encoding("UTF-8"))
    assert_equal 7, Basecamp::Ids.parse_int("7".dup.force_encoding("ASCII-8BIT"))
  end

  def test_a_person_id_of_another_type_is_still_a_decode_failure
    # Flexible is not lenient: the decoder takes a number or a string and
    # nothing else.
    [ 7.0, 7.5, true, false, [], [ 7 ], {}, { "id" => 7 } ].each do |value|
      assert_nil Basecamp::Ids.person_from_wire(value), "a person id of #{value.inspect}"
    end
  end

  def test_a_digit_run_is_bounded_before_it_is_converted
    # Every reader used to run to_i and then range-check the result, so a digit
    # run from a response body or from rich text built an arbitrarily large
    # Integer first — 5,000,000 digits measured at 2.5 seconds each, on a path
    # where a body may be 50 MB and five readers may see the same value.
    #
    # An int64 is at most 19 digits, so a longer magnitude is out of range by
    # LENGTH. Asserted on the answers rather than on a clock, because a timing
    # assertion is flaky and would not catch a bound that is fast and wrong.
    assert_equal 19, Basecamp::Ids::MAX_DIGITS

    # The boundary itself still converts, either side of it still answers.
    assert_equal MAX, Basecamp::Ids.bounded_decimal(MAX.to_s)
    assert_equal MIN, Basecamp::Ids.bounded_decimal(MIN.to_s)
    assert_equal :overflow, Basecamp::Ids.bounded_decimal((MAX + 1).to_s)
    assert_equal :overflow, Basecamp::Ids.bounded_decimal((MIN - 1).to_s)

    # LEADING ZEROS carry no magnitude, so a long run of them is still in range
    # — a length check that did not strip them first would refuse this.
    assert_equal 7, Basecamp::Ids.bounded_decimal("#{"0" * 100}7")
    assert_equal 0, Basecamp::Ids.bounded_decimal("0" * 100)
    assert_equal MAX, Basecamp::Ids.bounded_decimal("#{"0" * 50}#{MAX}")

    # And the three verdicts stay distinct, because two callers need to tell
    # "not a number" from "too big": one zeroes the first and refuses the
    # second, the other raises different errors for each.
    assert_equal :not_decimal, Basecamp::Ids.bounded_decimal("basecamp")
    assert_equal :not_decimal, Basecamp::Ids.bounded_decimal("+7", signed: false)
    assert_equal 7, Basecamp::Ids.bounded_decimal("+7")

    # A run far past the bound answers without building the number.
    assert_equal :overflow, Basecamp::Ids.bounded_decimal("9" * 1_000_000)
  end

  def test_the_length_gate_is_what_makes_the_bound_cheap
    # The gate has to be asserted on the CLOCK, because it is the only
    # observable difference it makes: every answer is identical with and
    # without it, so deleting it left the whole suite green — including the
    # row above, which passes either way. The mechanism the bound exists for
    # was verified by nothing.
    #
    # The previous version of this file justified omitting a timing assertion
    # on the grounds that it would be flaky and "would not catch a bound that
    # is fast and wrong". The second clause is right and is why the answer
    # assertions above stay. The first is measurably false here: gated is
    # 0.06s at five million digits and ungated is 2.58s, a 41x margin. A
    # threshold an order of magnitude inside that is not a flaky test.
    digits = "9" * 5_000_000

    elapsed = Process.clock_gettime(Process::CLOCK_MONOTONIC)
    assert_equal :overflow, Basecamp::Ids.bounded_decimal(digits)
    elapsed = Process.clock_gettime(Process::CLOCK_MONOTONIC) - elapsed

    assert_operator elapsed, :<, 1.0,
      "five million digits took #{elapsed.round(3)}s — the length gate is not short-circuiting, " \
      "which means the value is being converted before it is bounded"
  end

  def test_the_two_wire_readers_disagree_only_where_the_reference_does
    # The whole reason both exist. A digit string is an id for a person and a
    # decode failure everywhere else, and that asymmetry is the reference's.
    assert_nil Basecamp::Ids.from_wire("7")
    assert_equal 7, Basecamp::Ids.person_from_wire("7")

    assert_nil Basecamp::Ids.from_wire("basecamp")
    assert_equal 0, Basecamp::Ids.person_from_wire("basecamp")

    # Every value where they DISAGREE, spelled out. The first version of this
    # row listed mostly values that are nil on both sides, so five of its seven
    # assertions were "nil" == "nil" — satisfied by any mutant breaking both
    # readers identically, and exercising none of the disagreement its own name
    # claims to check.
    {
      "7" => [ nil, 7 ], "007" => [ nil, 7 ], "+7" => [ nil, 7 ], "-7" => [ nil, -7 ],
      "basecamp" => [ nil, 0 ], "" => [ nil, 0 ], "abc" => [ nil, 0 ],
      MAX.to_s => [ nil, MAX ], nil => [ 0, nil ]
    }.each do |value, (strict, flexible)|
      assert_equal strict.inspect, Basecamp::Ids.from_wire(value).inspect, "from_wire(#{value.inspect})"
      assert_equal flexible.inspect, Basecamp::Ids.person_from_wire(value).inspect,
                   "person_from_wire(#{value.inspect})"
      assert_not_equal strict.inspect, flexible.inspect, "#{value.inspect} must be a disagreement"
    end

    # And they agree on every value neither can read, which is the rest of it.
    [ 7, 0, -5, 7.0, true, [], {}, MAX + 1, MIN - 1 ].each do |value|
      assert_equal Basecamp::Ids.from_wire(value).inspect,
                   Basecamp::Ids.person_from_wire(value).inspect,
                   "both readers on #{value.inspect}"
    end
  end
end
