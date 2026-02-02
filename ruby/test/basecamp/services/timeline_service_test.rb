# frozen_string_literal: true

# Tests for the TimelineService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Only get_project_timeline() available (not progress, person_progress, etc.)
# - No client-side validation (API validates)

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

  def test_get_project_timeline
    events = [ sample_event(id: 1, action: "updated"), sample_event(id: 2, action: "completed") ]

    stub_get("/12345/timeline.json", response_body: events)

    result = @account.timeline.get_project_timeline.to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal "updated", result[0]["action"]
  end

  # Note: progress(), person_progress(), person_progress_events() methods
  # not available in generated service (spec-conformant)
end
