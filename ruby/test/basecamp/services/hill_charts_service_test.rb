# frozen_string_literal: true

require "test_helper"

class HillChartsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_get
    response = {
      "enabled" => true,
      "stale" => false,
      "updated_at" => "2026-03-11T06:38:12.167Z",
      "app_versions_url" => "https://3.basecamp.com/12345/buckets/100/todosets/42/hill/versions",
      "dots" => [
        {
          "id" => 1069479424,
          "label" => "Background and research",
          "color" => "blue",
          "position" => 0
        }
      ]
    }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todosets/\d+/hill\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.hill_charts.get(todoset_id: 42)
    assert_equal true, result["enabled"]
    assert_equal false, result["stale"]
    assert_equal 1, result["dots"].length
    assert_equal "Background and research", result["dots"][0]["label"]
    assert_equal "https://3.basecamp.com/12345/buckets/100/todosets/42/hill/versions", result["app_versions_url"]
  end

  def test_update_settings
    response = {
      "enabled" => true,
      "stale" => false,
      "updated_at" => "2026-03-11T07:00:00.000Z",
      "dots" => [
        {
          "id" => 1069479424,
          "label" => "Background and research",
          "color" => "blue",
          "position" => 0
        },
        {
          "id" => 1069479573,
          "label" => "Design mockups",
          "color" => "green",
          "position" => 42
        }
      ]
    }

    stub_request(:put, %r{https://3\.basecampapi\.com/12345/todosets/\d+/hills/settings\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.hill_charts.update_settings(
      todoset_id: 42,
      tracked: [ 1069479573 ],
      untracked: [ 1069479511 ]
    )
    assert_equal true, result["enabled"]
    assert_equal 2, result["dots"].length
    assert_equal "Design mockups", result["dots"][1]["label"]
    assert_equal 42, result["dots"][1]["position"]
  end
end
