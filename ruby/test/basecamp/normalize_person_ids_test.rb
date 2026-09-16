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
    Basecamp::Http.normalize_person_ids(data, embedded_people: true)

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
    Basecamp::Http.normalize_person_ids(data, embedded_people: true)

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
    Basecamp::Http.normalize_person_ids(data, embedded_people: true)

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
    Basecamp::Http.normalize_person_ids(data, embedded_people: true)

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
      Basecamp::Http.normalize_person_ids(data, embedded_people: true)

      assert_equal id, data["id"], "a person id of #{wire.inspect}"
    end
  end

  def test_the_measured_go_corpus_at_the_normalizer
    # All 74 rows of GoPersonIds::CORPUS through the real normalizer, which is
    # the pre-decode half of the same rule Basecamp::Ids.person_from_wire reads
    # with: both call Ids.parse_int, so a drift in either shows up in both
    # files. The three outcomes are spelled as the normalizer writes them —
    # a number with no system_label, id 0 with the raw text kept, or the string
    # left exactly as it arrived so the reader is the one that refuses it.
    assert_corpus_at("a tagged person") do |wire|
      [ { "personable_type" => "User", "id" => wire }, [] ]
    end
  end

  # ---------------------------------------------------------------------
  # The people the reference finds by POSITION rather than by type tag:
  # normalizeEmbeddedPersonIds (go/pkg/basecamp/normalize.go:83). These three
  # shapes carry NO "personable_type" at all, which is the case BC3 actually
  # serves for embedded people and the case this SDK used to miss entirely:
  # measured before the second pass existed, 62 of the 74 rows differed from
  # the reference in each of them, and the twelve that matched did so only
  # because "leave the string" is also what doing nothing looks like.
  # ---------------------------------------------------------------------

  # ---------------------------------------------------------------------
  # WHERE the positional pass runs, which is a correction. It used to run on
  # every response body. Go calls normalizeEmbeddedPeopleJSON from exactly two
  # places -- decodeGaugePayload (gauges.go:170) and the notification decoders
  # (my_notifications.go:171, 281, 296) -- and "creator" / "participants" are
  # not unique to those wrappers: on UpcomingScheduleEntry they hold
  # UpcomingSchedulePerson, a plain int64 id in the reference, so a string there
  # is a decode error in Go. Running everywhere turned it into the system actor.
  # ---------------------------------------------------------------------

  def test_the_positional_pass_does_not_run_off_the_reference_surfaces
    # THE NEGATIVE TEST. An upcoming-schedule entry, which the reference never
    # normalizes: its creator and participants keep the strings they arrived
    # with, rather than becoming person 0 with a system_label -- the SYSTEM
    # ACTOR, for a body Go refuses to decode at all.
    data = {
      "schedule_entries" => [
        { "id" => 1,
          "creator" => { "id" => "basecamp", "name" => "Basecamp" },
          "participants" => [ { "id" => "007", "name" => "Padded" } ] }
      ]
    }
    Basecamp::Http.normalize_person_ids(data)

    entry = data["schedule_entries"][0]
    assert_equal "basecamp", entry["creator"]["id"]
    assert_not entry["creator"].key?("system_label"),
           "a strict site must not be handed the system actor's label"
    assert_equal "007", entry["participants"][0]["id"]
  end

  def test_the_personable_type_pass_still_runs_everywhere
    # The other half: narrowing the positional pass must not narrow the tagged
    # one, which predates this work and whose reach is unchanged. An object that
    # declares personable_type IS the Person projection.
    data = { "report" => { "actor" => { "id" => "007", "personable_type" => "User" } } }
    Basecamp::Http.normalize_person_ids(data)

    assert_equal 7, data["report"]["actor"]["id"]
  end

  def test_the_reference_surfaces_are_exactly_gauges_and_notifications
    base = "https://3.basecampapi.com/999"
    %w[/my/readings.json /my/readings/bubble_ups.json /gauge_needles/5
       /projects/1/gauge/needles.json /reports/gauges.json].each do |path|
      assert Basecamp::Http.embedded_people_url?("#{base}#{path}?page=2"), path
    end
    %w[/reports/schedules/upcoming.json /my/assignments.json
       /schedule_entries/9.json /todos/42.json /my/out_of_office.json].each do |path|
      assert_not Basecamp::Http.embedded_people_url?("#{base}#{path}"), path
    end
  end

  def test_the_measured_go_corpus_at_an_untagged_creator
    assert_corpus_at("an untagged creator") do |wire|
      [ { "creator" => { "id" => wire, "name" => "Ann" } }, [ "creator" ] ]
    end
  end

  def test_the_measured_go_corpus_at_an_untagged_participant
    assert_corpus_at("an untagged participant") do |wire|
      [ { "participants" => [ { "id" => wire, "name" => "Ann" } ] }, [ "participants", 0 ] ]
    end
  end

  def test_the_measured_go_corpus_at_a_creator_nested_in_a_collection
    # "At any depth" is the reference's rule, so the pass has to recurse rather
    # than look at the top level of the body: a schedule entry's creator sits
    # under a bucket, inside an array, inside the document.
    assert_corpus_at("a creator three levels down") do |wire|
      [ { "recordings" => [ { "bucket" => { "creator" => { "id" => wire } } } ] },
        [ "recordings", 0, "bucket", "creator" ] ]
    end
  end

  def test_the_positional_pass_finds_only_creator_and_participants
    # The reference names exactly two keys, so this must not become "any key
    # holding something person-shaped". A todo's assignees are people too, but
    # the reference reaches them through "personable_type" or not at all, and
    # coercing them here would accept ids it leaves as strings.
    data = {
      "assignees" => [ { "id" => "7", "name" => "Ann" } ],
      "person" => { "id" => "7" },
      "creator" => { "id" => "7" },
      "participants" => [ { "id" => "8" } ]
    }
    Basecamp::Http.normalize_person_ids(data, embedded_people: true)

    assert_equal "7", data["assignees"][0]["id"], "an assignee is not found by position"
    assert_equal "7", data["person"]["id"], "a \"person\" key is not one of the two"
    assert_equal 7, data["creator"]["id"]
    assert_equal 8, data["participants"][0]["id"]
  end

  def test_the_positional_pass_skips_what_is_not_person_shaped
    # Matched key for key with normalize.go:86-95: a "creator" that is not an
    # object and a "participants" that is not an array are left alone, as are
    # non-object elements inside one.
    data = { "creator" => "basecamp", "participants" => "none" }
    other = { "creator" => [ { "id" => "7" } ], "participants" => [ "7", nil, 7, { "id" => "9" } ] }
    Basecamp::Http.normalize_person_ids(data, embedded_people: true)
    Basecamp::Http.normalize_person_ids(other, embedded_people: true)

    assert_equal({ "creator" => "basecamp", "participants" => "none" }, data)
    assert_equal [ { "id" => "7" } ], other["creator"], "an array under \"creator\" is not a person"
    assert_equal [ "7", nil, 7, { "id" => 9 } ], other["participants"]
  end

  def test_normalizing_twice_changes_nothing
    # This walk does in one pass what the reference does in two, which is only
    # sound because coercing an id is idempotent: an object both passes reach
    # must come out the same however many times it is visited. Asserted here
    # rather than reasoned about, since it is the whole argument for the shape
    # of the method.
    GoPersonIds::CORPUS.each do |wire, _expected|
      once = { "personable_type" => "User", "id" => wire,
               "creator" => { "id" => wire }, "participants" => [ { "id" => wire } ] }
      Basecamp::Http.normalize_person_ids(once, embedded_people: true)
      twice = Marshal.load(Marshal.dump(once))
      Basecamp::Http.normalize_person_ids(twice, embedded_people: true)

      assert_equal once, twice, "a second normalization of #{wire.inspect}"
    end
  end

  def test_an_oversized_malformed_id_is_refused_rather_than_made_the_system_actor
    # The scan-order pair, at this site. ParseUint refuses the magnitude inside
    # the loop, so it never reaches the "x": one digit decides whether a
    # malformed id is the "basecamp" system actor or a failed read. This site
    # read BOTH as the system actor until it stopped testing the string's shape
    # and started walking it the way the reference does.
    sentinel = { "personable_type" => "User", "id" => "18446744073709551615x" }
    refused = { "personable_type" => "User", "id" => "18446744073709551616x" }
    Basecamp::Http.normalize_person_ids(sentinel, embedded_people: true)
    Basecamp::Http.normalize_person_ids(refused, embedded_people: true)

    assert_equal 0, sentinel["id"]
    assert_equal "18446744073709551615x", sentinel["system_label"]

    assert_equal "18446744073709551616x", refused["id"]
    assert_not refused.key?("system_label")
  end

  def test_a_sentinel_keeps_its_original_text_and_an_overflow_is_left_alone
    data = { "personable_type" => "User", "id" => "0x10" }
    Basecamp::Http.normalize_person_ids(data, embedded_people: true)

    assert_equal "0x10", data["system_label"], "the original text is preserved"

    # Out of int64 range: the reference leaves it a string for the decoder to
    # refuse rather than substituting a sentinel, because it is a real number
    # that does not fit rather than a label.
    overflow = { "personable_type" => "User", "id" => (2**63).to_s }
    Basecamp::Http.normalize_person_ids(overflow, embedded_people: true)

    assert_equal (2**63).to_s, overflow["id"]
    assert_not overflow.key?("system_label")
  end

  private

    # Runs all 74 measured rows through the real normalizer in one document
    # shape, and asserts the reference's verdict on the person the block points
    # at. The three outcomes are spelled as the normalizer writes them: a number
    # with no system_label, id 0 with the raw text kept, or the string left
    # exactly as it arrived so the READER is the one that refuses it.
    def assert_corpus_at(shape)
      GoPersonIds::CORPUS.each do |wire, expected|
        document, path = yield(wire)
        Basecamp::Http.normalize_person_ids(document, embedded_people: true)
        person = path.empty? ? document : document.dig(*path)
        where = "#{wire.inspect} as #{shape}"

        case expected
        when :label
          assert_equal 0, person["id"], where
          assert_equal wire, person["system_label"], "the sentinel text of #{where}"
        when :refuse
          assert_equal wire, person["id"], "#{where} is left for the reader"
          assert_not person.key?("system_label"), "#{where} is a number, not a sentinel"
        else
          assert_equal expected.last, person["id"], where
          assert_not person.key?("system_label"), "#{where} is a number"
        end
      end
    end
end
