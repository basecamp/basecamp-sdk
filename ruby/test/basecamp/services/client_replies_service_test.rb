# frozen_string_literal: true

require "test_helper"

class ClientRepliesServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_reply(id: 1, content: "<p>Thank you for the update!</p>")
    {
      "id" => id,
      "content" => content,
      "creator" => { "id" => 1, "name" => "Client User" },
      "created_at" => "2024-01-01T00:00:00Z"
    }
  end

  def test_list_replies
    stub_get("/12345/buckets/100/client/recordings/200/replies.json",
             response_body: [ sample_reply, sample_reply(id: 2, content: "<p>Looking forward to it!</p>") ])

    replies = @account.client_replies.list(project_id: 100, recording_id: 200).to_a

    assert_equal 2, replies.length
    assert_equal "<p>Thank you for the update!</p>", replies[0]["content"]
    assert_equal "<p>Looking forward to it!</p>", replies[1]["content"]
  end

  def test_get_reply
    stub_get("/12345/buckets/100/client/recordings/200/replies/300.json", response_body: sample_reply(id: 300))

    reply = @account.client_replies.get(project_id: 100, recording_id: 200, reply_id: 300)

    assert_equal 300, reply["id"]
    assert_equal "<p>Thank you for the update!</p>", reply["content"]
    assert_equal "Client User", reply["creator"]["name"]
  end
end
