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

  def test_my_assignments
    response = {
      "priorities" => [
        {
          "id" => 1,
          "content" => "Priority assignment",
          "completed" => false,
          "type" => "Todo",
          "comments_count" => 2,
          "has_description" => true,
          "children" => [
            {
              "id" => 2,
              "content" => "Nested assignment",
              "completed" => true,
              "type" => "Todo",
              "comments_count" => 0,
              "has_description" => false
            }
          ]
        }
      ],
      "non_priorities" => [
        {
          "id" => 3,
          "content" => "Backlog assignment",
          "completed" => true,
          "type" => "Todo",
          "comments_count" => 1,
          "has_description" => false
        }
      ]
    }
    stub_get("/12345/my/assignments.json", response_body: response)

    result = @account.reports.my_assignments

    assert_kind_of Hash, result
    assert_equal 1, result["priorities"].length
    assert_equal "Priority assignment", result["priorities"][0]["content"]
    assert_equal "Nested assignment", result["priorities"][0]["children"][0]["content"]
    assert_equal true, result["non_priorities"][0]["completed"]
  end

  def test_my_assignments_unauthorized
    stub_get("/12345/my/assignments.json", response_body: "", status: 401)

    assert_raises(Basecamp::AuthError) do
      @account.reports.my_assignments
    end
  end

  def test_my_assignments_completed
    response = [
      {
        "id" => 10,
        "content" => "Completed assignment",
        "completed" => true,
        "type" => "Todo",
        "comments_count" => 4,
        "has_description" => true
      }
    ]
    stub_get("/12345/my/assignments/completed.json", response_body: response)

    result = @account.reports.my_assignments_completed

    assert_kind_of Array, result
    assert_equal 1, result.length
    assert_equal "Completed assignment", result[0]["content"]
    assert_equal true, result[0]["completed"]
  end

  def test_my_assignments_completed_forbidden
    stub_get("/12345/my/assignments/completed.json", response_body: "", status: 403)

    assert_raises(Basecamp::ForbiddenError) do
      @account.reports.my_assignments_completed
    end
  end

  def test_my_assignments_due
    response = [
      {
        "id" => 20,
        "content" => "Due assignment",
        "due_on" => "2024-04-03",
        "completed" => false,
        "type" => "Todo",
        "comments_count" => 0,
        "has_description" => false
      }
    ]
    stub_request(:get, "https://3.basecampapi.com/12345/my/assignments/due.json")
      .with(query: { scope: "due_today" })
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.reports.my_assignments_due(scope: "due_today")

    assert_kind_of Array, result
    assert_equal 1, result.length
    assert_equal "2024-04-03", result[0]["due_on"]
  end

  def test_my_assignments_due_rate_limited
    stub_request(:get, "https://3.basecampapi.com/12345/my/assignments/due.json")
      .with(query: { scope: "due_today" })
      .to_return(status: 429, body: "", headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::RateLimitError) do
      @account.reports.my_assignments_due(scope: "due_today")
    end
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
    stub_request(:get, %r{https://3\.basecampapi\.com/12345/reports/users/progress/456\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.reports.person_progress(person_id: 456)

    assert_kind_of Hash, result
    assert_equal "Jane Doe", result["person"]["name"]
  end

  def test_person_progress_multi_page_wrapped
    page1 = {
      "person" => { "id" => 456, "name" => "Jane Doe" },
      "events" => [
        { "id" => 1, "action" => "created" },
        { "id" => 2, "action" => "completed" }
      ]
    }
    page2 = {
      "person" => { "id" => 456, "name" => "Jane Doe" },
      "events" => [
        { "id" => 3, "action" => "updated" }
      ]
    }

    page2_url = "https://3.basecampapi.com/12345/reports/users/progress/456.json?page=2"

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/reports/users/progress/456\.json$})
      .to_return(
        status: 200,
        body: page1.to_json,
        headers: {
          "Content-Type" => "application/json",
          "X-Total-Count" => "3",
          "Link" => "<#{page2_url}>; rel=\"next\""
        }
      )

    stub_request(:get, page2_url)
      .to_return(
        status: 200,
        body: page2.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    result = @account.reports.person_progress(person_id: 456)

    # Wrapper field (person) preserved from page 1
    assert_equal "Jane Doe", result["person"]["name"]

    # Events accumulated across both pages via lazy Enumerator
    all_events = result["events"].to_a
    assert_equal 3, all_events.length
    assert_equal "created", all_events[0]["action"]
    assert_equal "completed", all_events[1]["action"]
    assert_equal "updated", all_events[2]["action"]
  end

  # Note: Timesheet methods (timesheet, project_timesheet, recording_timesheet) moved to TimesheetsService
  # Note: assignable_people moved to PeopleService.list_assignable
end
