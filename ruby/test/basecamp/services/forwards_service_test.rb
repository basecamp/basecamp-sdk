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

  def sample_forward(id: nil, subject: nil)
    fixture = load_fixture("forwards/get.json")
    fixture.merge "id" => id || fixture["id"], "subject" => subject || fixture["subject"]
  end

  def sample_reply(id: nil, content: nil)
    fixture = load_fixture("forwards/reply_get.json")
    fixture.merge "id" => id || fixture["id"], "content" => content || fixture["content"]
  end

  def test_get_inbox
    # Generated service: /inboxes/{id} without .json
    stub_get("/12345/inboxes/200", response_body: sample_inbox(id: 200))

    inbox = @account.forwards.get_inbox(inbox_id: 200)

    assert_equal 200, inbox["id"]
    assert_equal "Email Forwards", inbox["name"]
  end

  def test_list_forwards
    stub_get("/12345/inboxes/200/inbox_forwards.json",
             response_body: [ sample_forward, sample_forward(id: 2, subject: "Another Email") ])

    forwards = @account.forwards.list(inbox_id: 200).to_a

    assert_equal 2, forwards.length
    assert_equal "Project proposal from client", forwards[0]["subject"]
    assert_equal "Another Email", forwards[1]["subject"]
  end

  def test_get_forward
    # Generated service: /inbox_forwards/{id} without .json
    stub_get("/12345/inbox_forwards/200", response_body: sample_forward(id: 200))

    forward = @account.forwards.get(forward_id: 200)

    assert_equal 200, forward["id"]
    assert_equal "Project proposal from client", forward["subject"]
    assert_equal "client@example.com", forward["from"]
  end

  def test_list_replies
    stub_get("/12345/inbox_forwards/200/replies.json",
             response_body: [ sample_reply, sample_reply(id: 2, content: "<p>Follow up!</p>") ])

    replies = @account.forwards.list_replies(forward_id: 200).to_a

    assert_equal 2, replies.length
    assert_equal load_fixture("forwards/reply_get.json")["content"], replies[0]["content"]
  end

  def test_get_reply
    # Generated service: /inbox_forwards/{id}/replies/{reply_id} without .json
    stub_get("/12345/inbox_forwards/200/replies/300", response_body: sample_reply(id: 300))

    reply = @account.forwards.get_reply(forward_id: 200, reply_id: 300)

    assert_equal 300, reply["id"]
    assert_equal load_fixture("forwards/reply_get.json")["content"], reply["content"]
  end
end
