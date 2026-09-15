# frozen_string_literal: true

require "test_helper"

class NormalizePersonIdsTest < Minitest::Test
  def test_sentinel_creator_id_normalized
    data = {
      "creator" => {
        "id" => "basecamp",
        "name" => "Basecamp",
        "personable_type" => "LocalPerson"
      }
    }
    Basecamp::Http.normalize_person_ids(data)

    assert_equal 0, data["creator"]["id"]
    assert_equal "basecamp", data["creator"]["system_label"]
  end

  def test_numeric_string_creator_id_coerced
    data = {
      "creator" => {
        "id" => "99999",
        "name" => "Real Person",
        "personable_type" => "User"
      }
    }
    Basecamp::Http.normalize_person_ids(data)

    assert_equal 99999, data["creator"]["id"]
    assert_nil data["creator"]["system_label"]
  end

  def test_integer_creator_id_unchanged
    data = {
      "creator" => {
        "id" => 12345,
        "name" => "Normal",
        "personable_type" => "User"
      }
    }
    Basecamp::Http.normalize_person_ids(data)

    assert_equal 12345, data["creator"]["id"]
    assert_nil data["creator"]["system_label"]
  end

  def test_nested_person_in_array
    data = [
      {
        "creator" => {
          "id" => "campfire",
          "name" => "Campfire",
          "personable_type" => "LocalPerson"
        }
      }
    ]
    Basecamp::Http.normalize_person_ids(data)

    assert_equal 0, data[0]["creator"]["id"]
    assert_equal "campfire", data[0]["creator"]["system_label"]
  end

  def test_the_grammar_is_signed_decimal_not_rubys_integer_literal
    # Integer() detects a base from the literal and tolerates surrounding space
    # and underscores; the reference uses ParseInt(s, 10, 64). Measured, the two
    # disagree on six shapes — and "010" is the one that matters most, because
    # it is not a refusal against an acceptance but two different PEOPLE: base
    # ten gives 10 and Ruby's octal detection gave 8, either of which a caller
    # could then mention.
    #
    # This runs before either composite sees the value, so their strict decoding
    # could never have seen the original.
    { "1_2" => 0, "0x10" => 0, "0b11" => 0, " 7" => 0, "7 " => 0, "1e3" => 0,
      "010" => 10, "+7" => 7, "-7" => 7 * -1, "7" => 7 }.each do |wire, id|
      data = { "personable_type" => "User", "id" => wire }
      Basecamp::Http.normalize_person_ids(data)

      assert_equal id, data["id"], "a person id of #{wire.inspect}"
    end
  end

  def test_a_sentinel_keeps_its_original_text_and_an_overflow_is_left_alone
    data = { "personable_type" => "User", "id" => "0x10" }
    Basecamp::Http.normalize_person_ids(data)

    assert_equal "0x10", data["system_label"], "the original text is preserved"

    # Out of int64 range: the reference leaves it a string for the decoder to
    # refuse rather than substituting a sentinel, because it is a real number
    # that does not fit rather than a label.
    overflow = { "personable_type" => "User", "id" => (2**63).to_s }
    Basecamp::Http.normalize_person_ids(overflow)

    assert_equal (2**63).to_s, overflow["id"]
    assert_not overflow.key?("system_label")
  end
end
