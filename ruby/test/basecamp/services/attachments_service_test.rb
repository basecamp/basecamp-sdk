# frozen_string_literal: true

require "test_helper"

class AttachmentsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_create_attachment
    attachment_response = {
      "attachable_sgid" => "BAh7CEkiCGdpZAY6BkVUSSIvZ2lk...",
      "content_type" => "application/pdf",
      "filename" => "report.pdf",
      "byte_size" => 1024
    }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/attachments\.json\?name=report\.pdf})
      .with(
        headers: { "Content-Type" => "application/pdf" }
      )
      .to_return(
        status: 200,
        body: attachment_response.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    result = @account.attachments.create(
      filename: "report.pdf",
      content_type: "application/pdf",
      data: "file data"
    )

    assert_equal "BAh7CEkiCGdpZAY6BkVUSSIvZ2lk...", result["attachable_sgid"]
    assert_equal "application/pdf", result["content_type"]
  end

  def test_create_attachment_with_special_characters_in_filename
    attachment_response = {
      "attachable_sgid" => "BAh7CEkiCGdpZAY6BkVUSSIvZ2lk...",
      "filename" => "my report (1).pdf"
    }

    stub_request(:post, "https://3.basecampapi.com/12345/attachments.json?name=my+report+%281%29.pdf")
      .to_return(
        status: 200,
        body: attachment_response.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    result = @account.attachments.create(
      filename: "my report (1).pdf",
      content_type: "application/pdf",
      data: "file data"
    )

    assert_equal "my report (1).pdf", result["filename"]
  end
end
