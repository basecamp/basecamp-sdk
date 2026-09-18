# frozen_string_literal: true

# Tests for the LineupService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Method names: create(), update(), delete() (not create_marker, update_marker, delete_marker)
# - No client-side validation (API validates)

require "test_helper"

class LineupServiceTest < Minitest::Test
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

    result = @account.lineup.list_lineup_markers.to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal 1069479400, result[0]["id"]
    assert_equal "Product Launch", result[0]["name"]
    assert_equal "2024-03-01", result[0]["date"]
  end

  def test_list_lineup_markers_empty
    stub_get("/12345/lineup/markers.json", response_body: [])

    result = @account.lineup.list_lineup_markers.to_a

    assert_kind_of Array, result
    assert_equal 0, result.length
  end

  def test_create
    stub_post("/12345/lineup/markers.json", response_body: "", status: 201)

    result = @account.lineup.create(
      name: "Launch Day",
      date: "2024-03-01"
    )

    assert_nil result
  end

  def test_update
    stub_request(:put, "https://3.basecampapi.com/12345/lineup/markers/1")
      .to_return(status: 204, body: "")

    result = @account.lineup.update(marker_id: 1, name: "Updated Launch")

    assert_nil result
  end

  def test_delete
    stub_request(:delete, "https://3.basecampapi.com/12345/lineup/markers/1")
      .to_return(status: 204, body: "")

    result = @account.lineup.delete(marker_id: 1)

    assert_nil result
  end
end
