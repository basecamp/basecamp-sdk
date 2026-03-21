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

  def test_assignments
    response = {
      "priorities" => [
        {
          "id" => 9007199254741623,
          "app_url" => "https://3.basecamp.com/195539477/buckets/2085958504/todos/9007199254741623",
          "content" => "Program the flux capacitor",
          "due_on" => "2026-03-15",
          "bucket" => {
            "id" => 2085958504,
            "name" => "The Leto Laptop",
            "app_url" => "https://3.basecamp.com/195539477/buckets/2085958504"
          },
          "completed" => false,
          "type" => "todo",
          "assignees" => [
            { "id" => 1049715913, "name" => "Victor Cooper" }
          ],
          "comments_count" => 0,
          "has_description" => false,
          "priority_recording_id" => 9007199254741700,
          "parent" => {
            "id" => 9007199254741601,
            "title" => "Development tasks",
            "app_url" => "https://3.basecamp.com/195539477/buckets/2085958504/todolists/9007199254741601"
          },
          "children" => []
        }
      ],
      "non_priorities" => []
    }
    stub_get("/12345/my/assignments.json", response_body: response)

    result = @account.reports.assignments

    assert_kind_of Hash, result
    assert_equal 1, result["priorities"].length
    assert_equal 9007199254741700, result["priorities"][0]["priority_recording_id"]
    assert_equal [], result["non_priorities"]
  end

  def test_completed_assignments
    response = [
      {
        "id" => 9007199254741623,
        "app_url" => "https://3.basecamp.com/195539477/buckets/2085958504/todos/9007199254741623",
        "content" => "Program the flux capacitor",
        "due_on" => "2026-03-15",
        "bucket" => {
          "id" => 2085958504,
          "name" => "The Leto Laptop",
          "app_url" => "https://3.basecamp.com/195539477/buckets/2085958504"
        },
        "completed" => true,
        "type" => "todo",
        "assignees" => [
          { "id" => 1049715913, "name" => "Victor Cooper" }
        ],
        "comments_count" => 0,
        "has_description" => false,
        "parent" => {
          "id" => 9007199254741601,
          "title" => "Development tasks",
          "app_url" => "https://3.basecamp.com/195539477/buckets/2085958504/todolists/9007199254741601"
        },
        "children" => []
      }
    ]
    stub_get("/12345/my/assignments/completed.json", response_body: response)

    result = @account.reports.completed_assignments

    assert_kind_of Array, result
    assert_equal true, result[0]["completed"]
  end

  def test_due_assignments
    response = [
      {
        "id" => 9007199254741623,
        "app_url" => "https://3.basecamp.com/195539477/buckets/2085958504/todos/9007199254741623",
        "content" => "Program the flux capacitor",
        "due_on" => "2026-03-22",
        "bucket" => {
          "id" => 2085958504,
          "name" => "The Leto Laptop",
          "app_url" => "https://3.basecamp.com/195539477/buckets/2085958504"
        },
        "completed" => false,
        "type" => "todo",
        "assignees" => [
          { "id" => 1049715913, "name" => "Victor Cooper" }
        ],
        "comments_count" => 0,
        "has_description" => false,
        "parent" => {
          "id" => 9007199254741601,
          "title" => "Development tasks",
          "app_url" => "https://3.basecamp.com/195539477/buckets/2085958504/todolists/9007199254741601"
        },
        "children" => []
      }
    ]

    stub_request(:get, "https://3.basecampapi.com/12345/my/assignments/due.json")
      .with(query: { scope: "due_tomorrow" })
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.reports.due_assignments(scope: "due_tomorrow")

    assert_kind_of Array, result
    assert_equal "2026-03-22", result[0]["due_on"]
  end

  def test_due_assignments_invalid_scope
    error_body = {
      "error" => "Invalid scope 'invalid'. Valid options: overdue, due_today, due_tomorrow, due_later_this_week, due_next_week, due_later"
    }

    stub_request(:get, "https://3.basecampapi.com/12345/my/assignments/due.json")
      .with(query: { scope: "invalid" })
      .to_return(status: 400, body: error_body.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::ValidationError) do
      @account.reports.due_assignments(scope: "invalid")
    end

    assert_includes error.message, "Invalid scope 'invalid'"
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
