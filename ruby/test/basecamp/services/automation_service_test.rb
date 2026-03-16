# frozen_string_literal: true

# Tests for the AutomationService (generated from OpenAPI spec)

require "test_helper"

class AutomationServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list_lineup_markers
    markers = [
      {
        id: 1069479400,
        name: "Product Launch",
        date: "2024-03-01",
        created_at: "2024-02-15T10:30:00.000Z",
        updated_at: "2024-02-15T10:30:00.000Z"
      },
      {
        id: 1069479401,
        name: "Quarterly Review",
        date: "2024-06-15",
        created_at: "2024-03-01T09:00:00.000Z",
        updated_at: "2024-03-01T09:00:00.000Z"
      }
    ]

    stub_get("/12345/lineup/markers.json", response_body: markers)

    result = @account.automation.list_lineup_markers.to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal 1069479400, result[0]["id"]
    assert_equal "Product Launch", result[0]["name"]
    assert_equal "2024-03-01", result[0]["date"]
  end

  def test_list_lineup_markers_empty
    stub_get("/12345/lineup/markers.json", response_body: [])

    result = @account.automation.list_lineup_markers.to_a

    assert_kind_of Array, result
    assert_equal 0, result.length
  end
end
