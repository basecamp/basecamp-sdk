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

    stub_get("/12345/projects/456/timeline.json", response_body: events)

    result = @account.timeline.get_project_timeline(project_id: 456).to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal "updated", result[0]["action"]
  end

  def test_get_project_timeline_decodes_additive_activity_fields
    events = [
      {
        "id" => 1,
        "created_at" => "2024-03-15T10:30:00Z",
        "kind" => "chat_transcript_rollup",
        "avatars_sample" => [
          "https://3.basecampapi.com/1/people/aaa/avatar",
          "https://3.basecampapi.com/1/people/bbb/avatar"
        ]
      },
      {
        "id" => 2,
        "created_at" => "2024-03-15T10:31:00Z",
        "kind" => "schedule_entry_created",
        "avatars_sample" => [],
        "data" => {
          "all_day" => true,
          "starts_at" => "2025-10-30",
          "ends_at" => "2025-10-30"
        }
      },
      {
        "id" => 3,
        "created_at" => "2024-03-15T10:32:00Z",
        "kind" => "upload_created",
        "avatars_sample" => [],
        "attachments" => [
          {
            "id" => 900,
            "type" => "Upload",
            "status" => "active",
            "visible_to_clients" => false,
            "title" => "Diagram",
            "filename" => "diagram.png",
            "content_type" => "image/png",
            "byte_size" => 20480,
            "width" => 1024.0,
            "height" => 768.0,
            "url" => "https://3.basecampapi.com/1/buckets/2/uploads/900.json",
            "app_url" => "https://3.basecamp.com/1/buckets/2/uploads/900",
            "download_url" => "https://3.basecampapi.com/1/buckets/2/uploads/900/download/diagram.png",
            "app_download_url" => "https://3.basecamp.com/1/buckets/2/uploads/900/download"
          }
        ]
      },
      {
        "id" => 4,
        "created_at" => "2024-03-15T10:33:00Z",
        "kind" => "comment_created",
        "avatars_sample" => [],
        "attachments" => [
          {
            "id" => 500,
            "attachable_sgid" => "sgid-attachable-500",
            "sgid" => "sgid-500",
            "status_url" => "https://3.basecampapi.com/1/attachments/sgid-500/status.json",
            "caption" => "See attached",
            "filename" => "notes.pdf",
            "content_type" => "application/pdf",
            "byte_size" => 4096,
            "key" => "blobkey500",
            "width" => nil,
            "height" => nil,
            "previewable" => true,
            "download_url" => "https://3.basecampapi.com/1/blobs/blobkey500/download/notes.pdf",
            "preview_url" => "https://3.basecampapi.com/1/blobs/blobkey500/previews/full",
            "thumbnail_url" => "https://3.basecampapi.com/1/blobs/blobkey500/previews/card"
          }
        ]
      }
    ]

    stub_get("/12345/projects/456/timeline.json", response_body: events)

    result = @account.timeline.get_project_timeline(project_id: 456).to_a

    assert_equal 4, result.length

    # Event 0: non-empty avatars_sample
    assert_equal 2, result[0]["avatars_sample"].length

    # Event 1: schedule-entry timing payload, all-day date-only bounds
    assert_equal true, result[1]["data"]["all_day"]
    assert_equal "2025-10-30", result[1]["data"]["starts_at"]
    assert_equal "2025-10-30", result[1]["data"]["ends_at"]

    # Event 2: full Upload recording attachment variant
    upload = result[2]["attachments"][0]
    assert_equal "Upload", upload["type"]
    assert_equal "diagram.png", upload["filename"]
    assert_not_nil upload["app_download_url"]
    assert_equal 1024.0, upload["width"]

    # Event 3: rich-text attachment/blob partial variant
    blob = result[3]["attachments"][0]
    assert_equal "sgid-attachable-500", blob["attachable_sgid"]
    assert_equal "See attached", blob["caption"]
    assert_equal "blobkey500", blob["key"]
    assert_equal true, blob["previewable"]
    assert_nil blob["width"]
  end

  # Note: progress(), person_progress(), person_progress_events() methods
  # not available in generated service (spec-conformant)
end
