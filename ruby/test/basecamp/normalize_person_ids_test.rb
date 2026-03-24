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
end
