# frozen_string_literal: true

# Tests for the ReportsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Timesheet methods moved to TimesheetsService
# - assigned_todos renamed to assigned
# - assignable_people moved to PeopleService.list_assignable

require "test_helper"

class ReportsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_progress
    events = [
      { "id" => 1, "action" => "created", "recording_type" => "Todo" },
      { "id" => 2, "action" => "completed", "recording_type" => "Todo" }
    ]
    stub_get("/12345/reports/progress.json", response_body: events)

    result = @account.reports.progress.to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal "created", result[0]["action"]
  end

  def test_upcoming
    upcoming = {
      "entries" => [
        { "id" => 1, "summary" => "Meeting", "starts_at" => "2024-01-20T10:00:00Z" }
      ]
    }
    stub_get("/12345/reports/schedules/upcoming.json", response_body: upcoming)

    result = @account.reports.upcoming

    assert_kind_of Hash, result
    assert_equal 1, result["entries"].length
  end

  def test_upcoming_with_date_range
    upcoming = { "entries" => [] }
    stub_request(:get, "https://3.basecampapi.com/12345/reports/schedules/upcoming.json")
      .with(query: { window_starts_on: "2024-01-01", window_ends_on: "2024-01-31" })
      .to_return(status: 200, body: upcoming.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.reports.upcoming(window_starts_on: "2024-01-01", window_ends_on: "2024-01-31")

    assert_kind_of Hash, result
  end

  def test_assigned
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

    result = @account.reports.assigned(person_id: 456)

    assert_kind_of Hash, result
    assert_equal "Jane Doe", result["person"]["name"]
    assert_equal "project", result["grouped_by"]
    assert_equal 1, result["todos"].length
    assert_equal "Task for Jane", result["todos"][0]["content"]
  end

  def test_assigned_with_group_by
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

    result = @account.reports.assigned(person_id: 456, group_by: "date")

    assert_equal "date", result["grouped_by"]
  end

  def test_overdue
    response = {
      "overdue_todos" => [
        { "id" => 1, "content" => "Overdue task", "due_on" => "2024-01-01" }
      ]
    }
    stub_get("/12345/reports/todos/overdue.json", response_body: response)

    result = @account.reports.overdue

    assert_kind_of Hash, result
  end

  def test_person_progress
    response = {
      "person" => { "id" => 456, "name" => "Jane Doe" },
      "events" => [
        { "id" => 1, "action" => "created" }
      ]
    }
    stub_request(:get, %r{https://3\.basecampapi\.com/12345/reports/users/progress/456$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.reports.person_progress(person_id: 456)

    assert_kind_of Hash, result
    assert_equal "Jane Doe", result["person"]["name"]
  end

  # Note: Timesheet methods (timesheet, project_timesheet, recording_timesheet) moved to TimesheetsService
  # Note: assignable_people moved to PeopleService.list_assignable
end
