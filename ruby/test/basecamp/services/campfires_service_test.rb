# frozen_string_literal: true

# Tests for the CampfiresService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Single-resource paths without .json (get, get_chatbot, get_line, etc.)
# - Collection/create paths with .json
# - No client-side validation (API validates)

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
      "lines_url" => "https://3.basecampapi.com/12345/chats/#{id}/lines.json"
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
      "lines_url" => "https://3.basecampapi.com/12345/integrations/abc123/chats/200/lines.json"
    }
  end

  def test_list_campfires
    stub_get("/12345/chats.json", response_body: [ sample_campfire, sample_campfire(id: 2, title: "General") ])

    campfires = @account.campfires.list.to_a

    assert_equal 2, campfires.length
    assert_equal "Team Chat", campfires[0]["title"]
    assert_equal "General", campfires[1]["title"]
  end

  def test_list_campfires_with_files_url
    campfire = sample_campfire.merge(
      "files_url" => "https://3.basecampapi.com/12345/chats/1/uploads.json"
    )
    stub_get("/12345/chats.json", response_body: [ campfire ])

    campfires = @account.campfires.list.to_a

    assert_equal "https://3.basecampapi.com/12345/chats/1/uploads.json", campfires[0]["files_url"]
  end

  def test_get_campfire
    # Generated service: /chats/{id} without .json
    stub_get("/12345/chats/200", response_body: sample_campfire(id: 200))

    campfire = @account.campfires.get(campfire_id: 200)

    assert_equal 200, campfire["id"]
    assert_equal "Team Chat", campfire["title"]
  end

  def test_list_lines
    stub_get("/12345/chats/200/lines.json",
             response_body: [ sample_line, sample_line(id: 2, content: "Hi there!") ])

    lines = @account.campfires.list_lines(campfire_id: 200).to_a

    assert_equal 2, lines.length
    assert_equal "Hello!", lines[0]["content"]
    assert_equal "Hi there!", lines[1]["content"]
  end

  def test_list_lines_with_mixed_text_and_upload
    upload = sample_upload_line(id: 3, filename: "photo.png", content_type: "image/png", byte_size: 2048)
    stub_get("/12345/chats/200/lines.json",
             response_body: [ sample_line, upload ])

    lines = @account.campfires.list_lines(campfire_id: 200).to_a

    assert_equal 2, lines.length
    assert_equal "Hello!", lines[0]["content"]
    assert_nil lines[1]["content"]
    assert_equal "Chat::Lines::Upload", lines[1]["type"]
    assert_equal 1, lines[1]["attachments"].length
    assert_equal "photo.png", lines[1]["attachments"][0]["filename"]
    assert_equal 2048, lines[1]["attachments"][0]["byte_size"]
  end

  def test_get_line
    # Generated service: /lines/{id} without .json
    stub_get("/12345/chats/200/lines/300", response_body: sample_line(id: 300))

    line = @account.campfires.get_line(campfire_id: 200, line_id: 300)

    assert_equal 300, line["id"]
    assert_equal "Hello!", line["content"]
  end

  def test_create_line
    new_line = sample_line(id: 999, content: "New message")
    stub_post("/12345/chats/200/lines.json", response_body: new_line)

    line = @account.campfires.create_line(campfire_id: 200, content: "New message")

    assert_equal 999, line["id"]
    assert_equal "New message", line["content"]
  end

  def test_create_line_with_content_type
    new_line = sample_line(id: 998, content: "<strong>Rich text</strong>")
    stub_post("/12345/chats/200/lines.json", response_body: new_line)

    line = @account.campfires.create_line(
      campfire_id: 200,
      content: "<strong>Rich text</strong>",
      content_type: "text/html"
    )

    assert_equal 998, line["id"]
  end

  def test_delete_line
    # Generated service: /lines/{id} without .json
    stub_delete("/12345/chats/200/lines/300")

    result = @account.campfires.delete_line(campfire_id: 200, line_id: 300)

    assert_nil result
  end

  def test_list_chatbots
    stub_get("/12345/chats/200/integrations.json",
             response_body: [ sample_chatbot, sample_chatbot(id: 2, service_name: "AnotherBot") ])

    chatbots = @account.campfires.list_chatbots(campfire_id: 200).to_a

    assert_equal 2, chatbots.length
    assert_equal "TestBot", chatbots[0]["service_name"]
  end

  def test_get_chatbot
    # Generated service: /integrations/{id} without .json
    stub_get("/12345/chats/200/integrations/300", response_body: sample_chatbot(id: 300))

    chatbot = @account.campfires.get_chatbot(campfire_id: 200, chatbot_id: 300)

    assert_equal 300, chatbot["id"]
    assert_equal "TestBot", chatbot["service_name"]
  end

  def test_create_chatbot
    new_chatbot = sample_chatbot(id: 999, service_name: "NewBot")
    stub_post("/12345/chats/200/integrations.json", response_body: new_chatbot)

    chatbot = @account.campfires.create_chatbot(
      campfire_id: 200,
      service_name: "NewBot",
      command_url: "https://example.com/webhook"
    )

    assert_equal 999, chatbot["id"]
    assert_equal "NewBot", chatbot["service_name"]
  end

  def test_update_chatbot
    # Generated service: /integrations/{id} without .json
    updated_chatbot = sample_chatbot(id: 300, service_name: "UpdatedBot")
    stub_put("/12345/chats/200/integrations/300", response_body: updated_chatbot)

    chatbot = @account.campfires.update_chatbot(
      campfire_id: 200,
      chatbot_id: 300,
      service_name: "UpdatedBot"
    )

    assert_equal "UpdatedBot", chatbot["service_name"]
  end

  def test_delete_chatbot
    # Generated service: /integrations/{id} without .json
    stub_delete("/12345/chats/200/integrations/300")

    result = @account.campfires.delete_chatbot(campfire_id: 200, chatbot_id: 300)

    assert_nil result
  end

  def test_list_uploads
    upload_lines = [
      sample_upload_line,
      sample_upload_line(id: 2, filename: "screenshot.png", content_type: "image/png", byte_size: 2048)
    ]
    stub_get("/12345/chats/200/uploads.json", response_body: upload_lines)

    uploads = @account.campfires.list_uploads(campfire_id: 200).to_a

    assert_equal 2, uploads.length
    assert_equal "report.pdf", uploads[0]["attachments"][0]["filename"]
    assert_equal "application/pdf", uploads[0]["attachments"][0]["content_type"]
    assert_equal 1_048_576, uploads[0]["attachments"][0]["byte_size"]
    assert_equal "screenshot.png", uploads[1]["attachments"][0]["filename"]
  end

  def test_create_upload
    upload_line = sample_upload_line(id: 999)

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/chats/200/uploads\.json\?name=report\.pdf})
      .with(
        headers: { "Content-Type" => "application/pdf" }
      )
      .to_return(
        status: 201,
        body: upload_line.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    result = @account.campfires.create_upload(
      campfire_id: 200,
      name: "report.pdf",
      content_type: "application/pdf",
      data: "file data"
    )

    assert_equal 999, result["id"]
    assert_equal "Chat::Lines::Upload", result["type"]
    assert_equal "report.pdf", result["attachments"][0]["filename"]
  end

  def test_create_upload_encodes_filename
    upload_line = sample_upload_line(id: 998)

    stub_request(:post, "https://3.basecampapi.com/12345/chats/200/uploads.json?name=my+report+%281%29.pdf")
      .to_return(
        status: 201,
        body: upload_line.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    result = @account.campfires.create_upload(
      campfire_id: 200,
      name: "my report (1).pdf",
      content_type: "application/pdf",
      data: "file data"
    )

    assert_equal 998, result["id"]
  end

  def test_list_uploads_forbidden
    stub_get("/12345/chats/200/uploads.json",
             response_body: { "error" => "Forbidden" }, status: 403)

    assert_raises(Basecamp::ForbiddenError) do
      @account.campfires.list_uploads(campfire_id: 200).to_a
    end
  end

  def test_create_upload_validation_error
    stub_request(:post, %r{https://3\.basecampapi\.com/12345/chats/200/uploads\.json})
      .to_return(
        status: 422,
        body: { "error" => "Unprocessable" }.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    assert_raises(Basecamp::ValidationError) do
      @account.campfires.create_upload(
        campfire_id: 200,
        name: "bad.pdf",
        content_type: "application/pdf",
        data: "file data"
      )
    end
  end

  def test_get_campfire_with_files_url
    campfire = sample_campfire.merge(
      "files_url" => "https://3.basecampapi.com/12345/chats/200/uploads.json"
    )
    stub_get("/12345/chats/200", response_body: campfire)

    result = @account.campfires.get(campfire_id: 200)

    assert_equal "https://3.basecampapi.com/12345/chats/200/uploads.json", result["files_url"]
  end

  private

  def sample_upload_line(id: 1, filename: "report.pdf", content_type: "application/pdf", byte_size: 1_048_576)
    {
      "id" => id,
      "title" => filename,
      "type" => "Chat::Lines::Upload",
      "created_at" => "2024-01-01T00:00:00Z",
      "attachments" => [
        {
          "title" => filename,
          "url" => "https://3.basecampapi.com/12345/uploads/#{id + 100}.json",
          "filename" => filename,
          "content_type" => content_type,
          "byte_size" => byte_size,
          "download_url" => "https://3.basecampapi.com/12345/uploads/#{id + 100}/download/#{filename}"
        }
      ],
      "creator" => { "id" => 1, "name" => "Test User" }
    }
  end
end
