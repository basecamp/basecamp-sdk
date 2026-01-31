# frozen_string_literal: true

# Tests for the ForwardsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Single-resource paths without .json (get_inbox, get, get_reply)

require "test_helper"

class ForwardsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_inbox(id: 1)
    {
      "id" => id,
      "name" => "Email Forwards",
      "forwards_count" => 5
    }
  end

  def sample_forward(id: 1, subject: "Client Question")
    {
      "id" => id,
      "subject" => subject,
      "from" => "client@example.com",
      "content" => "<p>I have a question about the project.</p>",
      "created_at" => "2024-01-01T00:00:00Z"
    }
  end

  def sample_reply(id: 1, content: "<p>Thanks for reaching out!</p>")
    {
      "id" => id,
      "content" => content,
      "creator" => { "id" => 1, "name" => "Test User" },
      "created_at" => "2024-01-01T00:00:00Z"
    }
  end

  def test_get_inbox
    # Generated service: /inboxes/{id} without .json
    stub_get("/12345/buckets/100/inboxes/200", response_body: sample_inbox(id: 200))

    inbox = @account.forwards.get_inbox(project_id: 100, inbox_id: 200)

    assert_equal 200, inbox["id"]
    assert_equal "Email Forwards", inbox["name"]
  end

  def test_list_forwards
    stub_get("/12345/buckets/100/inboxes/200/forwards.json",
             response_body: [ sample_forward, sample_forward(id: 2, subject: "Another Email") ])

    forwards = @account.forwards.list(project_id: 100, inbox_id: 200).to_a

    assert_equal 2, forwards.length
    assert_equal "Client Question", forwards[0]["subject"]
    assert_equal "Another Email", forwards[1]["subject"]
  end

  def test_get_forward
    # Generated service: /inbox_forwards/{id} without .json
    stub_get("/12345/buckets/100/inbox_forwards/200", response_body: sample_forward(id: 200))

    forward = @account.forwards.get(project_id: 100, forward_id: 200)

    assert_equal 200, forward["id"]
    assert_equal "Client Question", forward["subject"]
    assert_equal "client@example.com", forward["from"]
  end

  def test_list_replies
    stub_get("/12345/buckets/100/inbox_forwards/200/replies.json",
             response_body: [ sample_reply, sample_reply(id: 2, content: "<p>Follow up!</p>") ])

    replies = @account.forwards.list_replies(project_id: 100, forward_id: 200).to_a

    assert_equal 2, replies.length
    assert_equal "<p>Thanks for reaching out!</p>", replies[0]["content"]
  end

  def test_get_reply
    # Generated service: /inbox_forwards/{id}/replies/{reply_id} without .json
    stub_get("/12345/buckets/100/inbox_forwards/200/replies/300", response_body: sample_reply(id: 300))

    reply = @account.forwards.get_reply(project_id: 100, forward_id: 200, reply_id: 300)

    assert_equal 300, reply["id"]
    assert_equal "<p>Thanks for reaching out!</p>", reply["content"]
  end

  def test_create_reply
    new_reply = sample_reply(id: 999, content: "<p>Here is my response.</p>")
    stub_post("/12345/buckets/100/inbox_forwards/200/replies.json", response_body: new_reply)

    reply = @account.forwards.create_reply(
      project_id: 100,
      forward_id: 200,
      content: "<p>Here is my response.</p>"
    )

    assert_equal 999, reply["id"]
    assert_equal "<p>Here is my response.</p>", reply["content"]
  end
end
