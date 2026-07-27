# frozen_string_literal: true

# Tests for the EverythingService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - get_everything_files() is paginated and returns the parsed JSON array
# - No client-side validation (API validates)

require "test_helper"

class EverythingServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_get_everything_files_decodes_heterogeneous_feed
    files = [
      {
        "id" => 900,
        "type" => "Upload",
        "status" => "active",
        "visible_to_clients" => false,
        "title" => "logo.png",
        "inherits_status" => true,
        "filename" => "logo.png",
        "content_type" => "image/png",
        "byte_size" => 1281,
        "width" => 1024.0,
        "height" => 768.0,
        "url" => "https://3.basecampapi.com/1/buckets/2/uploads/900.json",
        "app_url" => "https://3.basecamp.com/1/buckets/2/uploads/900",
        "download_url" => "https://3.basecampapi.com/1/buckets/2/uploads/900/download/logo.png",
        "app_download_url" => "https://storage.3.basecamp.com/1/buckets/2/uploads/900/download/logo.png",
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" },
        "creator" => { "id" => 1, "name" => "Victor Cooper" }
      },
      {
        "id" => 901,
        "type" => "Document",
        "status" => "active",
        "visible_to_clients" => false,
        "title" => "Spec",
        "inherits_status" => true,
        "content_type" => "text/html",
        "url" => "https://3.basecampapi.com/1/buckets/2/documents/901.json",
        "app_url" => "https://3.basecamp.com/1/buckets/2/documents/901",
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" },
        "creator" => { "id" => 1, "name" => "Victor Cooper" }
      },
      {
        "id" => 902,
        "type" => "Attachment",
        "attachable_sgid" => "sgid-902",
        "filename" => "chart.avif",
        "content_type" => "image/avif",
        "byte_size" => 4096,
        "width" => nil,
        "height" => nil,
        "download_url" => "https://storage.3.basecamp.com/1/blobs/902/download/chart.avif",
        "parent" => { "id" => 800, "title" => "A message", "type" => "Message" }
      }
    ]

    stub_get("/12345/files.json", response_body: files)

    result = @account.everything.get_everything_files(kind: nil, people_ids: nil).to_a

    assert_kind_of Array, result
    assert_equal 3, result.length

    # Variant 0: full Upload recording
    assert_equal "Upload", result[0]["type"]
    assert_equal "logo.png", result[0]["filename"]
    assert_not_nil result[0]["app_download_url"]
    assert_equal 1024.0, result[0]["width"]

    # Variant 1: Basecamp Document recording
    assert_equal "Document", result[1]["type"]
    assert_equal "Spec", result[1]["title"]

    # Variant 2: rich-text Attachment envelope
    assert_equal "Attachment", result[2]["type"]
    assert_equal "sgid-902", result[2]["attachable_sgid"]
    assert_not_nil result[2]["parent"]
    assert_nil result[2]["width"]
  end

  def test_get_everything_messages_decodes_recording_feed
    messages = [
      {
        "id" => 1001,
        "type" => "Message",
        "status" => "active",
        "visible_to_clients" => false,
        "title" => "Kickoff",
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" },
        "creator" => { "id" => 1, "name" => "Victor Cooper" }
      },
      {
        "id" => 1002,
        "type" => "Message",
        "status" => "active",
        "visible_to_clients" => true,
        "title" => "Update",
        "bucket" => { "id" => 3, "name" => "Honcho Design", "type" => "Project" },
        "creator" => { "id" => 1, "name" => "Victor Cooper" }
      }
    ]

    stub_get("/12345/messages.json", response_body: messages)

    result = @account.everything.get_everything_messages.to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal 1001, result[0]["id"]
    assert_equal "Message", result[0]["type"]
    assert_equal 2, result[0]["bucket"]["id"]
    assert_equal "Project", result[0]["bucket"]["type"]
  end

  def test_get_everything_comments_decodes_recording_feed
    comments = [
      {
        "id" => 2001,
        "type" => "Comment",
        "status" => "active",
        "visible_to_clients" => false,
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" },
        "creator" => { "id" => 1, "name" => "Victor Cooper" }
      },
      {
        "id" => 2002,
        "type" => "Comment",
        "status" => "active",
        "visible_to_clients" => false,
        "bucket" => { "id" => 3, "name" => "Honcho Design", "type" => "Project" },
        "creator" => { "id" => 1, "name" => "Victor Cooper" }
      }
    ]

    stub_get("/12345/comments.json", response_body: comments)

    result = @account.everything.get_everything_comments.to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal 2001, result[0]["id"]
    assert_equal "Comment", result[0]["type"]
    assert_equal 2, result[0]["bucket"]["id"]
  end

  def test_get_everything_checkins_decodes_recording_feed
    checkins = [
      {
        "id" => 3001,
        "type" => "Question::Answer",
        "status" => "active",
        "visible_to_clients" => false,
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" },
        "creator" => { "id" => 1, "name" => "Victor Cooper" }
      },
      {
        "id" => 3002,
        "type" => "Question::Answer",
        "status" => "active",
        "visible_to_clients" => false,
        "bucket" => { "id" => 3, "name" => "Honcho Design", "type" => "Project" },
        "creator" => { "id" => 1, "name" => "Victor Cooper" }
      }
    ]

    stub_get("/12345/checkins.json", response_body: checkins)

    result = @account.everything.get_everything_checkins.to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal 3001, result[0]["id"]
    assert_equal "Question::Answer", result[0]["type"]
    assert_equal 2, result[0]["bucket"]["id"]
  end

  def test_get_everything_forwards_decodes_recording_feed
    forwards = [
      {
        "id" => 4001,
        "type" => "Inbox::Forward",
        "status" => "active",
        "visible_to_clients" => false,
        "subject" => "FW: Invoice",
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" },
        "creator" => { "id" => 1, "name" => "Victor Cooper" }
      },
      {
        "id" => 4002,
        "type" => "Inbox::Forward",
        "status" => "active",
        "visible_to_clients" => false,
        "subject" => "FW: Contract",
        "bucket" => { "id" => 3, "name" => "Honcho Design", "type" => "Project" },
        "creator" => { "id" => 1, "name" => "Victor Cooper" }
      }
    ]

    stub_get("/12345/forwards.json", response_body: forwards)

    result = @account.everything.get_everything_forwards.to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal 4001, result[0]["id"]
    assert_equal "Inbox::Forward", result[0]["type"]
    assert_equal 2, result[0]["bucket"]["id"]
  end

  def test_get_everything_boosts_decodes_boost_feed
    boosts = [
      {
        "id" => 5001,
        "content" => "🎉",
        "created_at" => "2024-01-15T10:00:00Z",
        "booster" => { "id" => 1, "name" => "Victor Cooper" },
        "recording" => {
          "id" => 1001,
          "type" => "Message",
          "title" => "Kickoff",
          "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" }
        }
      },
      {
        "id" => 5002,
        "content" => "👏",
        "created_at" => "2024-01-15T09:00:00Z",
        "booster" => { "id" => 1, "name" => "Victor Cooper" },
        "recording" => {
          "id" => 2001,
          "type" => "Comment",
          "bucket" => { "id" => 3, "name" => "Honcho Design", "type" => "Project" }
        }
      }
    ]

    stub_get("/12345/boosts.json", response_body: boosts)

    result = @account.everything.get_everything_boosts.to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal 5001, result[0]["id"]
    assert_equal "🎉", result[0]["content"]
    assert_equal "Victor Cooper", result[0]["booster"]["name"]
    assert_not_nil result[0]["recording"]
    assert_equal 1001, result[0]["recording"]["id"]
    assert_equal "Message", result[0]["recording"]["type"]
  end

  def test_get_everything_overdue_todos_decodes_oldest_first
    todos = [
      {
        "id" => 6001,
        "type" => "Todo",
        "status" => "active",
        "visible_to_clients" => false,
        "title" => "Ship the thing",
        "due_on" => "2024-01-01",
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" }
      },
      {
        "id" => 6002,
        "type" => "Todo",
        "status" => "active",
        "visible_to_clients" => false,
        "title" => "Review the doc",
        "due_on" => "2024-02-01",
        "bucket" => { "id" => 3, "name" => "Honcho Design", "type" => "Project" }
      }
    ]

    stub_get("/12345/todos/overdue.json", response_body: todos)

    result = @account.everything.get_everything_overdue_todos.to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal 6001, result[0]["id"]
    assert_equal "2024-01-01", result[0]["due_on"]
    # Oldest-first ordering
    assert result[0]["due_on"] < result[1]["due_on"]
  end

  def test_get_everything_overdue_cards_decodes_oldest_first
    cards = [
      {
        "id" => 7001,
        "type" => "Kanban::Card",
        "status" => "active",
        "visible_to_clients" => false,
        "title" => "Draft proposal",
        "due_on" => "2024-01-10",
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" }
      },
      {
        "id" => 7002,
        "type" => "Kanban::Card",
        "status" => "active",
        "visible_to_clients" => false,
        "title" => "Send invoice",
        "due_on" => "2024-02-10",
        "bucket" => { "id" => 3, "name" => "Honcho Design", "type" => "Project" }
      }
    ]

    stub_get("/12345/cards/overdue.json", response_body: cards)

    result = @account.everything.get_everything_overdue_cards.to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal 7001, result[0]["id"]
    assert_equal "2024-01-10", result[0]["due_on"]
    # Oldest-first ordering
    assert result[0]["due_on"] < result[1]["due_on"]
  end

  # Every everything operation must surface a canonical 4xx as a typed error.
  def test_operations_propagate_not_found
    calls = {
      "/12345/messages.json" => -> { @account.everything.get_everything_messages.to_a },
      "/12345/comments.json" => -> { @account.everything.get_everything_comments.to_a },
      "/12345/checkins.json" => -> { @account.everything.get_everything_checkins.to_a },
      "/12345/forwards.json" => -> { @account.everything.get_everything_forwards.to_a },
      "/12345/boosts.json" => -> { @account.everything.get_everything_boosts.to_a },
      "/12345/files.json" => -> { @account.everything.get_everything_files(kind: nil, people_ids: nil).to_a },
      "/12345/todos/overdue.json" => -> { @account.everything.get_everything_overdue_todos.to_a },
      "/12345/cards/overdue.json" => -> { @account.everything.get_everything_overdue_cards.to_a },
      "/12345/todos/open.json" => -> { @account.everything.get_everything_open_todos.to_a },
      "/12345/todos/completed.json" => -> { @account.everything.get_everything_completed_todos.to_a },
      "/12345/todos/unassigned.json" => -> { @account.everything.get_everything_unassigned_todos.to_a },
      "/12345/todos/no_due_date.json" => -> { @account.everything.get_everything_no_due_date_todos.to_a },
      "/12345/cards/open.json" => -> { @account.everything.get_everything_open_cards.to_a },
      "/12345/cards/completed.json" => -> { @account.everything.get_everything_completed_cards.to_a },
      "/12345/cards/unassigned.json" => -> { @account.everything.get_everything_unassigned_cards.to_a },
      "/12345/cards/no_due_date.json" => -> { @account.everything.get_everything_no_due_date_cards.to_a },
      "/12345/cards/not_now.json" => -> { @account.everything.get_everything_not_now_cards.to_a }
    }

    calls.each do |path, call|
      stub_get(path, response_body: "", status: 404)
      assert_raises(Basecamp::NotFoundError, "expected #{path} to raise NotFoundError") do
        call.call
      end
    end
  end

  def test_get_everything_open_todos_decodes_bucket_groups
    groups = [
      {
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" },
        "todos" => [
          {
            "id" => 8001,
            "type" => "Todo",
            "status" => "active",
            "visible_to_clients" => false,
            "title" => "Wire up auth",
            "steps" => [
              { "id" => 90_001, "type" => "Todo", "title" => "Add login form", "completed" => false }
            ]
          }
        ]
      }
    ]

    stub_get("/12345/todos/open.json", response_body: groups)

    result = @account.everything.get_everything_open_todos.to_a

    assert_kind_of Array, result
    assert_equal 1, result.length
    assert_equal 2, result[0]["bucket"]["id"]
    assert_equal "Project", result[0]["bucket"]["type"]
    assert_equal 8001, result[0]["todos"][0]["id"]
    assert_equal "Wire up auth", result[0]["todos"][0]["title"]
    assert_equal 90_001, result[0]["todos"][0]["steps"][0]["id"]
    assert_equal "Add login form", result[0]["todos"][0]["steps"][0]["title"]
  end

  def test_get_everything_completed_todos_decodes_bucket_groups
    groups = [
      {
        "bucket" => { "id" => 3, "name" => "Honcho Design", "type" => "Project" },
        "todos" => [
          {
            "id" => 8101,
            "type" => "Todo",
            "status" => "active",
            "visible_to_clients" => false,
            "title" => "Ship the redesign",
            "completed" => true,
            "steps" => [
              { "id" => 90_101, "type" => "Todo", "title" => "Publish assets", "completed" => true }
            ]
          }
        ]
      }
    ]

    stub_get("/12345/todos/completed.json", response_body: groups)

    result = @account.everything.get_everything_completed_todos.to_a

    assert_kind_of Array, result
    assert_equal 1, result.length
    assert_equal 3, result[0]["bucket"]["id"]
    assert_equal "Honcho Design", result[0]["bucket"]["name"]
    assert_equal 8101, result[0]["todos"][0]["id"]
    assert result[0]["todos"][0]["completed"]
    assert_equal 90_101, result[0]["todos"][0]["steps"][0]["id"]
    assert result[0]["todos"][0]["steps"][0]["completed"]
  end

  def test_get_everything_unassigned_todos_decodes_bucket_groups
    groups = [
      {
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" },
        "todos" => [
          {
            "id" => 8201,
            "type" => "Todo",
            "status" => "active",
            "visible_to_clients" => false,
            "title" => "Triage inbox",
            "assignees" => [],
            "steps" => [
              { "id" => 90_201, "type" => "Todo", "title" => "Sort by priority", "completed" => false }
            ]
          }
        ]
      }
    ]

    stub_get("/12345/todos/unassigned.json", response_body: groups)

    result = @account.everything.get_everything_unassigned_todos.to_a

    assert_kind_of Array, result
    assert_equal 1, result.length
    assert_equal 2, result[0]["bucket"]["id"]
    assert_equal 8201, result[0]["todos"][0]["id"]
    assert_empty result[0]["todos"][0]["assignees"]
    assert_equal 90_201, result[0]["todos"][0]["steps"][0]["id"]
    assert_equal "Sort by priority", result[0]["todos"][0]["steps"][0]["title"]
  end

  def test_get_everything_no_due_date_todos_decodes_bucket_groups
    groups = [
      {
        "bucket" => { "id" => 3, "name" => "Honcho Design", "type" => "Project" },
        "todos" => [
          {
            "id" => 8301,
            "type" => "Todo",
            "status" => "active",
            "visible_to_clients" => false,
            "title" => "Someday maybe",
            "due_on" => nil,
            "steps" => [
              { "id" => 90_301, "type" => "Todo", "title" => "Brainstorm ideas", "completed" => false }
            ]
          }
        ]
      }
    ]

    stub_get("/12345/todos/no_due_date.json", response_body: groups)

    result = @account.everything.get_everything_no_due_date_todos.to_a

    assert_kind_of Array, result
    assert_equal 1, result.length
    assert_equal 3, result[0]["bucket"]["id"]
    assert_equal 8301, result[0]["todos"][0]["id"]
    assert_nil result[0]["todos"][0]["due_on"]
    assert_equal 90_301, result[0]["todos"][0]["steps"][0]["id"]
    assert_equal "Brainstorm ideas", result[0]["todos"][0]["steps"][0]["title"]
  end

  def test_get_everything_open_cards_decodes_bucket_groups
    groups = [
      {
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" },
        "cards" => [
          {
            "id" => 8401,
            "type" => "Kanban::Card",
            "status" => "active",
            "visible_to_clients" => false,
            "title" => "Draft proposal",
            "steps" => [
              { "id" => 90_401, "type" => "Kanban::Step", "title" => "Outline sections", "completed" => false }
            ]
          }
        ]
      }
    ]

    stub_get("/12345/cards/open.json", response_body: groups)

    result = @account.everything.get_everything_open_cards.to_a

    assert_kind_of Array, result
    assert_equal 1, result.length
    assert_equal 2, result[0]["bucket"]["id"]
    assert_equal "Project", result[0]["bucket"]["type"]
    assert_equal 8401, result[0]["cards"][0]["id"]
    assert_equal "Kanban::Card", result[0]["cards"][0]["type"]
    assert_equal 90_401, result[0]["cards"][0]["steps"][0]["id"]
    assert_equal "Outline sections", result[0]["cards"][0]["steps"][0]["title"]
  end

  def test_get_everything_completed_cards_decodes_bucket_groups
    groups = [
      {
        "bucket" => { "id" => 3, "name" => "Honcho Design", "type" => "Project" },
        "cards" => [
          {
            "id" => 8501,
            "type" => "Kanban::Card",
            "status" => "active",
            "visible_to_clients" => false,
            "title" => "Send invoice",
            "completed" => true,
            "steps" => [
              { "id" => 90_501, "type" => "Kanban::Step", "title" => "Attach PDF", "completed" => true }
            ]
          }
        ]
      }
    ]

    stub_get("/12345/cards/completed.json", response_body: groups)

    result = @account.everything.get_everything_completed_cards.to_a

    assert_kind_of Array, result
    assert_equal 1, result.length
    assert_equal 3, result[0]["bucket"]["id"]
    assert_equal 8501, result[0]["cards"][0]["id"]
    assert result[0]["cards"][0]["completed"]
    assert_equal 90_501, result[0]["cards"][0]["steps"][0]["id"]
    assert result[0]["cards"][0]["steps"][0]["completed"]
  end

  def test_get_everything_unassigned_cards_decodes_bucket_groups
    groups = [
      {
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" },
        "cards" => [
          {
            "id" => 8601,
            "type" => "Kanban::Card",
            "status" => "active",
            "visible_to_clients" => false,
            "title" => "Unclaimed work",
            "assignees" => [],
            "steps" => [
              { "id" => 90_601, "type" => "Kanban::Step", "title" => "Claim this", "completed" => false }
            ]
          }
        ]
      }
    ]

    stub_get("/12345/cards/unassigned.json", response_body: groups)

    result = @account.everything.get_everything_unassigned_cards.to_a

    assert_kind_of Array, result
    assert_equal 1, result.length
    assert_equal 2, result[0]["bucket"]["id"]
    assert_equal 8601, result[0]["cards"][0]["id"]
    assert_empty result[0]["cards"][0]["assignees"]
    assert_equal 90_601, result[0]["cards"][0]["steps"][0]["id"]
    assert_equal "Claim this", result[0]["cards"][0]["steps"][0]["title"]
  end

  def test_get_everything_no_due_date_cards_decodes_bucket_groups
    groups = [
      {
        "bucket" => { "id" => 3, "name" => "Honcho Design", "type" => "Project" },
        "cards" => [
          {
            "id" => 8701,
            "type" => "Kanban::Card",
            "status" => "active",
            "visible_to_clients" => false,
            "title" => "No deadline",
            "due_on" => nil,
            "steps" => [
              { "id" => 90_701, "type" => "Kanban::Step", "title" => "Schedule later", "completed" => false }
            ]
          }
        ]
      }
    ]

    stub_get("/12345/cards/no_due_date.json", response_body: groups)

    result = @account.everything.get_everything_no_due_date_cards.to_a

    assert_kind_of Array, result
    assert_equal 1, result.length
    assert_equal 3, result[0]["bucket"]["id"]
    assert_equal 8701, result[0]["cards"][0]["id"]
    assert_nil result[0]["cards"][0]["due_on"]
    assert_equal 90_701, result[0]["cards"][0]["steps"][0]["id"]
    assert_equal "Schedule later", result[0]["cards"][0]["steps"][0]["title"]
  end

  def test_get_everything_not_now_cards_decodes_bucket_groups
    groups = [
      {
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" },
        "cards" => [
          {
            "id" => 8801,
            "type" => "Kanban::Card",
            "status" => "active",
            "visible_to_clients" => false,
            "title" => "Parked idea",
            "steps" => [
              { "id" => 90_801, "type" => "Kanban::Step", "title" => "Revisit next quarter", "completed" => false }
            ]
          }
        ]
      }
    ]

    stub_get("/12345/cards/not_now.json", response_body: groups)

    result = @account.everything.get_everything_not_now_cards.to_a

    assert_kind_of Array, result
    assert_equal 1, result.length
    assert_equal 2, result[0]["bucket"]["id"]
    assert_equal 8801, result[0]["cards"][0]["id"]
    assert_equal "Parked idea", result[0]["cards"][0]["title"]
    assert_equal 90_801, result[0]["cards"][0]["steps"][0]["id"]
    assert_equal "Revisit next quarter", result[0]["cards"][0]["steps"][0]["title"]
  end
end
