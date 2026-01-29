# frozen_string_literal: true

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

  def sample_entry(id: 789, summary: "Team Meeting")
    {
      "id" => id,
      "summary" => summary,
      "starts_at" => "2024-12-15T09:00:00Z",
      "ends_at" => "2024-12-15T10:00:00Z",
      "all_day" => false
    }
  end

  def test_get
    stub_get("/12345/buckets/100/schedules/456", response_body: sample_schedule)

    result = @account.schedules.get(project_id: 100, schedule_id: 456)

    assert_equal 456, result["id"]
    assert_equal "Schedule", result["title"]
  end

  def test_list_entries
    entries = [ sample_entry, sample_entry(id: 790, summary: "Another Event") ]
    stub_get("/12345/buckets/100/schedules/456/entries.json", response_body: entries)

    result = @account.schedules.list_entries(project_id: 100, schedule_id: 456).to_a

    assert_equal 2, result.length
    assert_equal "Team Meeting", result[0]["summary"]
  end

  def test_get_entry
    stub_get("/12345/buckets/100/schedule_entries/789", response_body: sample_entry)

    result = @account.schedules.get_entry(project_id: 100, entry_id: 789)

    assert_equal 789, result["id"]
    assert_equal "Team Meeting", result["summary"]
  end

  def test_create_entry
    new_entry = sample_entry(id: 999, summary: "New Event")
    stub_post("/12345/buckets/100/schedules/456/entries.json", response_body: new_entry)

    result = @account.schedules.create_entry(
      project_id: 100,
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
    stub_post("/12345/buckets/100/schedules/456/entries.json", response_body: new_entry)

    result = @account.schedules.create_entry(
      project_id: 100,
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

  def test_update_entry
    updated_entry = sample_entry(summary: "Updated Meeting")
    stub_put("/12345/buckets/100/schedule_entries/789", response_body: updated_entry)

    result = @account.schedules.update_entry(
      project_id: 100,
      entry_id: 789,
      summary: "Updated Meeting"
    )

    assert_equal "Updated Meeting", result["summary"]
  end

  def test_get_entry_occurrence
    occurrence = sample_entry.merge("occurrence_date" => "2024-12-22")
    stub_get("/12345/buckets/100/schedule_entries/789/occurrences/2024-12-22", response_body: occurrence)

    result = @account.schedules.get_entry_occurrence(project_id: 100, entry_id: 789, date: "2024-12-22")

    assert_equal "2024-12-22", result["occurrence_date"]
  end

  def test_update_settings
    updated_schedule = sample_schedule.merge("include_due_assignments" => false)
    stub_put("/12345/buckets/100/schedules/456", response_body: updated_schedule)

    result = @account.schedules.update_settings(
      project_id: 100,
      schedule_id: 456,
      include_due_assignments: false
    )

    assert_equal false, result["include_due_assignments"]
  end

  def test_trash_entry
    stub_put("/12345/buckets/100/recordings/789/status/trashed.json", response_body: {})

    result = @account.schedules.trash_entry(project_id: 100, entry_id: 789)

    assert_nil result
  end
end
