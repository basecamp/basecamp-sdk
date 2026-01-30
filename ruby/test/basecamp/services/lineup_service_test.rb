# frozen_string_literal: true

require "test_helper"

class LineupServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_create_marker
    marker = { "id" => 1, "title" => "Launch Day", "starts_on" => "2024-03-01", "ends_on" => "2024-03-01" }
    stub_post("/12345/lineup/markers.json", response_body: marker)

    result = @account.lineup.create_marker(
      title: "Launch Day",
      starts_on: "2024-03-01",
      ends_on: "2024-03-01"
    )

    assert_equal "Launch Day", result["title"]
    assert_equal "2024-03-01", result["starts_on"]
  end

  def test_create_marker_with_color_and_description
    marker = { "id" => 2, "title" => "Milestone", "color" => "green", "description" => "<p>Big day!</p>" }
    stub_post("/12345/lineup/markers.json", response_body: marker)

    result = @account.lineup.create_marker(
      title: "Milestone",
      starts_on: "2024-04-01",
      ends_on: "2024-04-01",
      color: "green",
      description: "<p>Big day!</p>"
    )

    assert_equal "green", result["color"]
  end

  def test_update_marker
    updated_marker = { "id" => 1, "title" => "Updated Launch", "color" => "blue" }
    stub_request(:put, "https://3.basecampapi.com/12345/lineup/markers/1")
      .to_return(status: 200, body: updated_marker.to_json)

    result = @account.lineup.update_marker(marker_id: 1, title: "Updated Launch", color: "blue")

    assert_equal "Updated Launch", result["title"]
    assert_equal "blue", result["color"]
  end

  def test_delete_marker
    stub_request(:delete, "https://3.basecampapi.com/12345/lineup/markers/1")
      .to_return(status: 204, body: "")

    result = @account.lineup.delete_marker(marker_id: 1)

    assert_nil result
  end
end
