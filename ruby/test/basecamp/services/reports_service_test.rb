# frozen_string_literal: true

require "test_helper"

class ReportsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_timesheet_entry(id: 1)
    {
      "id" => id,
      "duration" => 3600,
      "started_at" => "2024-01-15T09:00:00Z",
      "ended_at" => "2024-01-15T10:00:00Z",
      "creator" => { "id" => 1, "name" => "Test User" }
    }
  end

  def test_timesheet
    response = { "entries" => [ sample_timesheet_entry, sample_timesheet_entry(id: 2) ] }
    stub_get("/12345/reports/timesheet.json", response_body: response)

    result = @account.reports.timesheet

    assert_kind_of Hash, result
    assert_equal 2, result["entries"].length
    assert_equal 3600, result["entries"][0]["duration"]
  end

  def test_timesheet_with_filters
    response = { "entries" => [ sample_timesheet_entry ] }
    stub_request(:get, "https://3.basecampapi.com/12345/reports/timesheet.json")
      .with(query: { from: "2024-01-01", to: "2024-01-31", person_id: "1" })
      .to_return(status: 200, body: response.to_json)

    result = @account.reports.timesheet(from: "2024-01-01", to: "2024-01-31", person_id: 1)

    assert_equal 1, result["entries"].length
  end

  def test_project_timesheet
    response = { "entries" => [ sample_timesheet_entry ] }
    stub_get("/12345/buckets/100/timesheet.json", response_body: response)

    result = @account.reports.project_timesheet(project_id: 100)

    assert_equal 1, result["entries"].length
  end

  def test_project_timesheet_with_filters
    response = { "entries" => [ sample_timesheet_entry ] }
    stub_request(:get, "https://3.basecampapi.com/12345/buckets/100/timesheet.json")
      .with(query: { from: "2024-01-01", to: "2024-01-31" })
      .to_return(status: 200, body: response.to_json)

    result = @account.reports.project_timesheet(project_id: 100, from: "2024-01-01", to: "2024-01-31")

    assert_equal 1, result["entries"].length
  end

  def test_recording_timesheet
    response = { "entries" => [ sample_timesheet_entry ] }
    stub_get("/12345/buckets/100/recordings/456/timesheet.json", response_body: response)

    result = @account.reports.recording_timesheet(project_id: 100, recording_id: 456)

    assert_equal 1, result["entries"].length
  end

  def test_assignable_people
    people = [
      { "id" => 1, "name" => "Jane Doe" },
      { "id" => 2, "name" => "John Smith" }
    ]
    stub_get("/12345/reports/todos/assigned.json", response_body: people)

    result = @account.reports.assignable_people.to_a

    assert_equal 2, result.length
    assert_equal "Jane Doe", result[0]["name"]
  end

  def test_assigned_todos
    response = {
      "person" => { "id" => 456, "name" => "Jane Doe" },
      "grouped_by" => "project",
      "todos" => [
        { "id" => 1, "content" => "Task for Jane" }
      ]
    }
    # Note: no .json extension on this endpoint
    stub_request(:get, %r{https://3\.basecampapi\.com/12345/reports/todos/assigned/456$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.reports.assigned_todos(person_id: 456)

    assert_kind_of Hash, result
    assert_equal "Jane Doe", result["person"]["name"]
    assert_equal "project", result["grouped_by"]
    assert_equal 1, result["todos"].length
    assert_equal "Task for Jane", result["todos"][0]["content"]
  end

  def test_assigned_todos_with_group_by
    response = {
      "person" => { "id" => 456, "name" => "Jane Doe" },
      "grouped_by" => "date",
      "todos" => [
        { "id" => 1, "content" => "Task for Jane" }
      ]
    }
    stub_request(:get, "https://3.basecampapi.com/12345/reports/todos/assigned/456")
      .with(query: { group_by: "date" })
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.reports.assigned_todos(person_id: 456, group_by: "date")

    assert_equal "date", result["grouped_by"]
  end
end
