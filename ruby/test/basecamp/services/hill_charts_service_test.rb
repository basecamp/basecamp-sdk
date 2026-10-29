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

  def test_get_not_found
    stub_get("/12345/todosets/999/hill.json", response_body: "", status: 404)

    assert_raises(Basecamp::NotFoundError) do
      @account.hill_charts.get(todoset_id: 999)
    end
  end

  def test_update_settings_not_found
    stub_put("/12345/todosets/999/hills/settings.json", response_body: "", status: 404)

    assert_raises(Basecamp::NotFoundError) do
      @account.hill_charts.update_settings(todoset_id: 999, tracked: [ 1 ])
    end
  end

  def test_update_settings_sends_body
    stub_request(:put, %r{https://3\.basecampapi\.com/12345/todosets/\d+/hills/settings\.json})
      .with(body: hash_including("tracked" => [ 1069479573 ], "untracked" => [ 1069479511 ]))
      .to_return(status: 200, body: { "enabled" => true, "stale" => false, "dots" => [] }.to_json,
                 headers: { "Content-Type" => "application/json" })

    @account.hill_charts.update_settings(
      todoset_id: 42,
      tracked: [ 1069479573 ],
      untracked: [ 1069479511 ]
    )

    assert_requested(:put, %r{todosets/42/hills/settings\.json},
      body: hash_including("tracked" => [ 1069479573 ], "untracked" => [ 1069479511 ]))
  end

  def test_get_carries_versions_url
    stub_get("/12345/todosets/1069479829/hill.json", response_body: load_fixture("hill_charts/get.json"))

    result = @account.hill_charts.get(todoset_id: 1069479829)
    assert_equal "https://3.basecampapi.com/195539477/buckets/2085958505/todosets/1069479829/hills/versions.json",
      result["versions_url"]
  end

  def test_list_versions
    stub_request(:get, "https://3.basecampapi.com/12345/todosets/1069479829/hills/versions.json")
      .to_return(status: 200, body: load_fixture("hill_charts/versions.json").to_json,
                 headers: { "Content-Type" => "application/json", "X-Total-Count" => "1" })

    versions = @account.hill_charts.list_versions(todoset_id: 1069479829).to_a
    assert_equal 1, versions.length
    version = versions.first
    assert_equal 1069480194, version["id"]
    assert_equal "Hill::Version", version["type"]
    assert_includes version["description"], "Research is wrapping up."
    assert_equal [], version["description_attachments"]
    assert_equal 1, version["dots"].length
    assert_equal 1069479863, version["dots"][0]["id"]
    assert_equal 40, version["dots"][0]["position"]
  end

  def test_list_versions_follows_link_header
    first = "https://3.basecampapi.com/12345/todosets/1/hills/versions.json"
    stub_request(:get, first)
      .to_return(status: 200, body: load_fixture("hill_charts/versions.json").to_json,
                 headers: { "Content-Type" => "application/json", "Link" => "<#{first}?page=2>; rel=\"next\"" })
    stub_request(:get, "#{first}?page=2")
      .to_return(status: 200, body: load_fixture("hill_charts/versions.json").to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_equal 2, @account.hill_charts.list_versions(todoset_id: 1).to_a.length
  end

  def test_list_versions_not_found
    stub_get("/12345/todosets/999/hills/versions.json", response_body: "", status: 404)

    assert_raises(Basecamp::NotFoundError) do
      @account.hill_charts.list_versions(todoset_id: 999).to_a
    end
  end

  def test_get_version
    stub_get("/12345/todosets/1069479829/hills/versions/1069480194", response_body: load_fixture("hill_charts/version.json"))

    version = @account.hill_charts.get_version(todoset_id: 1069479829, version_id: 1069480194)
    assert_equal 1069480194, version["id"]
    assert_equal "Victor Cooper", version["creator"]["name"]
    assert_equal 40, version["dots"][0]["position"]
  end

  def test_get_version_with_a_todolist_gone
    stub_get("/12345/todosets/1069479829/hills/versions/1069480194",
      response_body: load_fixture("hill_charts/version_todolist_gone.json"))

    version = @account.hill_charts.get_version(todoset_id: 1069479829, version_id: 1069480194)
    gone = version["dots"][1]
    assert gone.key?("id")
    assert_nil gone["id"]
    assert_not gone.key?("url")
    assert_equal "Old research list", gone["label"]
  end

  def test_get_version_not_found
    stub_get("/12345/todosets/1/hills/versions/999", response_body: "", status: 404)

    assert_raises(Basecamp::NotFoundError) do
      @account.hill_charts.get_version(todoset_id: 1, version_id: 999)
    end
  end
end
