# frozen_string_literal: true

require "test_helper"

class TimelineServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_event(id: 1, action: "created")
    {
      "id" => id,
      "action" => action,
      "recording_type" => "Todo",
      "created_at" => "2024-01-15T10:00:00Z"
    }
  end

  def test_progress
    response = {
      "events" => [
        sample_event(id: 1, action: "created"),
        sample_event(id: 2, action: "updated")
      ]
    }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/reports/progress\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.timeline.progress.to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal "created", result[0]["action"]
  end

  def test_progress_pagination
    page1 = { "events" => [ sample_event(id: 1) ] }
    page2 = { "events" => [ sample_event(id: 2) ] }

    stub_request(:get, "https://3.basecampapi.com/12345/reports/progress.json")
      .to_return(
        status: 200,
        body: page1.to_json,
        headers: {
          "Content-Type" => "application/json",
          "Link" => '<https://3.basecampapi.com/12345/reports/progress.json?page=2>; rel="next"'
        }
      )

    stub_request(:get, "https://3.basecampapi.com/12345/reports/progress.json?page=2")
      .to_return(status: 200, body: page2.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.timeline.progress.to_a

    assert_equal 2, result.length
    assert_equal 1, result[0]["id"]
    assert_equal 2, result[1]["id"]
  end

  def test_project_timeline
    response = {
      "events" => [
        sample_event(id: 1, action: "updated")
      ]
    }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/buckets/100/timeline\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.timeline.project_timeline(project_id: 100).to_a

    assert_kind_of Array, result
    assert_equal 1, result.length
    assert_equal "updated", result[0]["action"]
  end

  def test_person_progress
    response = {
      "person" => { "id" => 456, "name" => "Jane Doe" },
      "events" => [
        sample_event(id: 1, action: "completed")
      ]
    }

    # Note: no .json extension on this endpoint
    stub_request(:get, %r{https://3\.basecampapi\.com/12345/reports/users/progress/456$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.timeline.person_progress(person_id: 456)

    assert_kind_of Hash, result
    assert_equal "Jane Doe", result["person"]["name"]
    assert_equal 1, result["events"].length
    assert_equal "completed", result["events"][0]["action"]
  end

  def test_person_progress_events
    response = {
      "person" => { "id" => 456, "name" => "Jane Doe" },
      "events" => [
        sample_event(id: 1, action: "completed")
      ]
    }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/reports/users/progress/456$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.timeline.person_progress_events(person_id: 456).to_a

    assert_kind_of Array, result
    assert_equal 1, result.length
    assert_equal "completed", result[0]["action"]
  end

  def test_person_progress_events_pagination
    page1 = { "person" => { "id" => 456 }, "events" => [ sample_event(id: 1) ] }
    page2 = { "person" => { "id" => 456 }, "events" => [ sample_event(id: 2) ] }

    stub_request(:get, "https://3.basecampapi.com/12345/reports/users/progress/456")
      .to_return(
        status: 200,
        body: page1.to_json,
        headers: {
          "Content-Type" => "application/json",
          "Link" => '<https://3.basecampapi.com/12345/reports/users/progress/456?page=2>; rel="next"'
        }
      )

    stub_request(:get, "https://3.basecampapi.com/12345/reports/users/progress/456?page=2")
      .to_return(status: 200, body: page2.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.timeline.person_progress_events(person_id: 456).to_a

    assert_equal 2, result.length
    assert_equal 1, result[0]["id"]
    assert_equal 2, result[1]["id"]
  end
end
