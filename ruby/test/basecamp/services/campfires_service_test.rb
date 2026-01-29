# frozen_string_literal: true

require "test_helper"

class CampfiresServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_campfire(id: 1, title: "Team Chat")
    {
      "id" => id,
      "title" => title,
      "lines_url" => "https://3.basecampapi.com/12345/buckets/100/chats/#{id}/lines.json"
    }
  end

  def sample_line(id: 1, content: "Hello!")
    {
      "id" => id,
      "content" => content,
      "creator" => { "id" => 1, "name" => "Test User" },
      "created_at" => "2024-01-01T00:00:00Z"
    }
  end

  def sample_chatbot(id: 1, service_name: "TestBot")
    {
      "id" => id,
      "service_name" => service_name,
      "lines_url" => "https://3.basecampapi.com/12345/integrations/abc123/buckets/100/chats/200/lines.json"
    }
  end

  def test_list_campfires
    stub_get("/12345/chats.json", response_body: [ sample_campfire, sample_campfire(id: 2, title: "General") ])

    campfires = @account.campfires.list.to_a

    assert_equal 2, campfires.length
    assert_equal "Team Chat", campfires[0]["title"]
    assert_equal "General", campfires[1]["title"]
  end

  def test_get_campfire
    stub_get("/12345/buckets/100/chats/200.json", response_body: sample_campfire(id: 200))

    campfire = @account.campfires.get(project_id: 100, campfire_id: 200)

    assert_equal 200, campfire["id"]
    assert_equal "Team Chat", campfire["title"]
  end

  def test_list_lines
    stub_get("/12345/buckets/100/chats/200/lines.json",
             response_body: [ sample_line, sample_line(id: 2, content: "Hi there!") ])

    lines = @account.campfires.list_lines(project_id: 100, campfire_id: 200).to_a

    assert_equal 2, lines.length
    assert_equal "Hello!", lines[0]["content"]
    assert_equal "Hi there!", lines[1]["content"]
  end

  def test_get_line
    stub_get("/12345/buckets/100/chats/200/lines/300.json", response_body: sample_line(id: 300))

    line = @account.campfires.get_line(project_id: 100, campfire_id: 200, line_id: 300)

    assert_equal 300, line["id"]
    assert_equal "Hello!", line["content"]
  end

  def test_create_line
    new_line = sample_line(id: 999, content: "New message")
    stub_post("/12345/buckets/100/chats/200/lines.json", response_body: new_line)

    line = @account.campfires.create_line(project_id: 100, campfire_id: 200, content: "New message")

    assert_equal 999, line["id"]
    assert_equal "New message", line["content"]
  end

  def test_delete_line
    stub_delete("/12345/buckets/100/chats/200/lines/300.json")

    result = @account.campfires.delete_line(project_id: 100, campfire_id: 200, line_id: 300)

    assert_nil result
  end

  def test_list_chatbots
    stub_get("/12345/buckets/100/chats/200/integrations.json",
             response_body: [ sample_chatbot, sample_chatbot(id: 2, service_name: "AnotherBot") ])

    chatbots = @account.campfires.list_chatbots(project_id: 100, campfire_id: 200).to_a

    assert_equal 2, chatbots.length
    assert_equal "TestBot", chatbots[0]["service_name"]
  end

  def test_get_chatbot
    stub_get("/12345/buckets/100/chats/200/integrations/300.json", response_body: sample_chatbot(id: 300))

    chatbot = @account.campfires.get_chatbot(project_id: 100, campfire_id: 200, chatbot_id: 300)

    assert_equal 300, chatbot["id"]
    assert_equal "TestBot", chatbot["service_name"]
  end

  def test_create_chatbot
    new_chatbot = sample_chatbot(id: 999, service_name: "NewBot")
    stub_post("/12345/buckets/100/chats/200/integrations.json", response_body: new_chatbot)

    chatbot = @account.campfires.create_chatbot(
      project_id: 100,
      campfire_id: 200,
      service_name: "NewBot",
      command_url: "https://example.com/webhook"
    )

    assert_equal 999, chatbot["id"]
    assert_equal "NewBot", chatbot["service_name"]
  end

  def test_update_chatbot
    updated_chatbot = sample_chatbot(id: 300, service_name: "UpdatedBot")
    stub_put("/12345/buckets/100/chats/200/integrations/300.json", response_body: updated_chatbot)

    chatbot = @account.campfires.update_chatbot(
      project_id: 100,
      campfire_id: 200,
      chatbot_id: 300,
      service_name: "UpdatedBot"
    )

    assert_equal "UpdatedBot", chatbot["service_name"]
  end

  def test_delete_chatbot
    stub_delete("/12345/buckets/100/chats/200/integrations/300.json")

    result = @account.campfires.delete_chatbot(project_id: 100, campfire_id: 200, chatbot_id: 300)

    assert_nil result
  end
end
