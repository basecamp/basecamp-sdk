# frozen_string_literal: true

# Tests for the SchedulesService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - No trash_entry() - use recordings.trash() instead
# - No client-side validation (API validates)

require "test_helper"

class SchedulesServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_schedule(id: 456)
    {
      "id" => id,
      "title" => "Schedule",
      "include_due_assignments" => true,
      "entries_count" => 5
    }
  end

  def sample_entry(id: nil, summary: nil)
    fixture = load_fixture("schedules/entry_get.json")
    fixture.merge "id" => id || fixture["id"], "summary" => summary || fixture["summary"]
  end

  def test_get
    stub_get("/12345/schedules/456", response_body: sample_schedule)

    result = @account.schedules.get(schedule_id: 456)

    assert_equal 456, result["id"]
    assert_equal "Schedule", result["title"]
  end

  def test_list_entries
    entries = [ sample_entry, sample_entry(id: 790, summary: "Another Event") ]
    stub_get("/12345/schedules/456/entries.json", response_body: entries)

    result = @account.schedules.list_entries(schedule_id: 456).to_a

    assert_equal 2, result.length
    assert_equal "Project Kickoff Meeting", result[0]["summary"]
  end

  def test_get_entry
    stub_get("/12345/schedule_entries/789", response_body: sample_entry(id: 789))

    result = @account.schedules.get_entry(entry_id: 789)

    assert_equal 789, result["id"]
    assert_equal "Project Kickoff Meeting", result["summary"]
  end

  def test_create_entry
    new_entry = sample_entry(id: 999, summary: "New Event")
    stub_post("/12345/schedules/456/entries.json", response_body: new_entry)

    result = @account.schedules.create_entry(
      schedule_id: 456,
      summary: "New Event",
      starts_at: "2024-12-20T14:00:00Z",
      ends_at: "2024-12-20T15:00:00Z"
    )

    assert_equal 999, result["id"]
    assert_equal "New Event", result["summary"]
  end

  def test_create_entry_with_all_options
    new_entry = sample_entry(id: 1000, summary: "Full Event")
    stub_post("/12345/schedules/456/entries.json", response_body: new_entry)

    result = @account.schedules.create_entry(
      schedule_id: 456,
      summary: "Full Event",
      starts_at: "2024-12-25T00:00:00Z",
      ends_at: "2024-12-25T23:59:59Z",
      description: "<p>Holiday party!</p>",
      participant_ids: [ 1, 2, 3 ],
      all_day: true,
      notify: true
    )

    assert_equal "Full Event", result["summary"]
  end

  def test_create_entry_with_subscriptions
    new_entry = sample_entry(id: 1001, summary: "Quiet Event")
    stub_post("/12345/schedules/456/entries.json", response_body: new_entry)

    @account.schedules.create_entry(
      schedule_id: 456,
      summary: "Quiet Event",
      starts_at: "2024-12-20T14:00:00Z",
      ends_at: "2024-12-20T15:00:00Z",
      subscriptions: [ 111, 222 ]
    )

    assert_requested(:post, "#{BASE_URL}/12345/schedules/456/entries.json",
      body: hash_including("subscriptions" => [ 111, 222 ]))
  end

  def test_update_entry
    updated_entry = sample_entry(summary: "Updated Meeting")
    stub_put("/12345/schedule_entries/789", response_body: updated_entry)

    result = @account.schedules.update_entry(
      entry_id: 789,
      summary: "Updated Meeting"
    )

    assert_equal "Updated Meeting", result["summary"]
  end

  def test_get_entry_occurrence
    occurrence = sample_entry.merge("occurrence_date" => "2024-12-22")
    stub_get("/12345/schedule_entries/789/occurrences/2024-12-22", response_body: occurrence)

    result = @account.schedules.get_entry_occurrence(entry_id: 789, date: "2024-12-22")

    assert_equal "2024-12-22", result["occurrence_date"]
  end

  # The occurrence path ends in `{date}` (a string), not an `Id`-suffixed param.
  # resource_id must fall back to the entry id, never the date string.
  def test_get_entry_occurrence_operation_metadata
    events = []
    account = create_account_client(account_id: "12345", hooks: CapturingHooks.new(events))

    occurrence = sample_entry.merge("occurrence_date" => "2024-12-22")
    stub_get("/12345/schedule_entries/789/occurrences/2024-12-22", response_body: occurrence)

    account.schedules.get_entry_occurrence(entry_id: 789, date: "2024-12-22")

    event = events.find { |e| e[:event] == :on_operation_start }
    assert event, "Expected on_operation_start to fire"
    info = event[:info]
    assert_equal "schedules", info.service
    assert_equal "get_entry_occurrence", info.operation
    assert_equal 789, info.resource_id
  end

  def test_update_settings
    updated_schedule = sample_schedule.merge("include_due_assignments" => false)
    stub_put("/12345/schedules/456", response_body: updated_schedule)

    result = @account.schedules.update_settings(
      schedule_id: 456,
      include_due_assignments: false
    )

    assert_equal false, result["include_due_assignments"]
  end

  # Note: trash_entry() is on RecordingsService, not SchedulesService (spec-conformant)
  # Use @account.recordings.trash(project_id:, recording_id:) instead

  private

  # Captures the OperationInfo passed to on_operation_start so tests can
  # assert the metadata emitted by the real generated service call.
  class CapturingHooks
    include Basecamp::Hooks

    def initialize(events)
      @events = events
    end

    def on_operation_start(info)
      @events << { event: :on_operation_start, info: info }
    end
  end
end
