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

  # The upcoming-schedule report renders BC3's reduced calendar partials
  # (app/views/api/schedules/calendar/), and its top-level key set is
  # schedule_entries / recurring_schedule_entry_occurrences / assignables.
  #
  # This test used to stub `{"entries" => [...]}` — not a key BC3 has ever sent
  # — and assert only that a Hash came back. Ruby is lenient, so it passed
  # against a contract that made Swift throw on every populated window (#635).
  # Reading the shared fixture keeps the body honest: it is validated against
  # the generated schema by `make check-fixture-coverage`.
  def test_upcoming
    upcoming = load_fixture("schedules/upcoming.json")
    stub_request(:get, "https://3.basecampapi.com/12345/reports/schedules/upcoming.json")
      .with(query: { window_starts_on: "2026-06-01", window_ends_on: "2026-06-30" })
      .to_return(status: 200, body: upcoming.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.reports.upcoming(window_starts_on: "2026-06-01", window_ends_on: "2026-06-30")

    assert_kind_of Hash, result
    assert_equal %w[schedule_entries recurring_schedule_entry_occurrences assignables].sort, result.keys.sort

    entry = result["schedule_entries"].first
    assert_equal "Team Meeting", entry["summary"]
    # Emitted only by the calendar partial, and the flag that separates the two
    # entry arrays.
    assert_not entry["recurring"]
    # id + name only: the calendar partial writes json.(recording.bucket, :id, :name).
    assert_equal %w[id name].sort, entry["bucket"].keys.sort

    occurrence = result["recurring_schedule_entry_occurrences"].first
    assert occurrence["recurring"]
    assert occurrence["all_day"]
    assert_equal "2026-06-08", occurrence["starts_at"]

    todo, card = result["assignables"]
    # BC3 spells the item text `content`, never `title`.
    assert_equal "Ship the hardware", todo["content"]
    assert_not todo.key?("title")
    assert_equal "todo", todo["type"]
    assert_equal "Steve Marsh", todo.dig("completion", "creator", "name")
    # The partial's one conditional key: absent on an incomplete item.
    assert_not card.key?("completion")
    assert_nil card["starts_on"]
  end

  # Both bounds are required — BC3 reads them with params.require and answers a
  # bodiless 400 without them — so the generated method takes them as required
  # keywords rather than options.
  def test_upcoming_requires_both_window_bounds
    assert_raises(ArgumentError) { @account.reports.upcoming(window_starts_on: "2026-06-01") }
    assert_raises(ArgumentError) { @account.reports.upcoming }
  end

  def test_upcoming_empty_window
    upcoming = { "schedule_entries" => [], "recurring_schedule_entry_occurrences" => [], "assignables" => [] }
    stub_request(:get, "https://3.basecampapi.com/12345/reports/schedules/upcoming.json")
      .with(query: { window_starts_on: "2026-01-01", window_ends_on: "2026-01-31" })
      .to_return(status: 200, body: upcoming.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.reports.upcoming(window_starts_on: "2026-01-01", window_ends_on: "2026-01-31")

    assert_equal [], result["schedule_entries"]
    assert_equal [], result["assignables"]
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
