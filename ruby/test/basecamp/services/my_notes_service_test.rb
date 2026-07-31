# frozen_string_literal: true

require "test_helper"

class MyNotesServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def written_note
    {
      "id" => 5,
      "type" => "Notebook::Note",
      "created_at" => "2026-07-21T00:02:30.308Z",
      "updated_at" => "2026-07-21T00:02:30.308Z",
      "content" => "<div dir=\"auto\">Things to remember…</div>",
      "content_attachments" => [],
      "url" => "https://3.basecampapi.com/12345/my/notes.json",
      "app_url" => "https://3.basecamp.com/12345/my/navigation/notes"
    }
  end

  def test_get_my_note
    stub_get("/12345/my/notes.json", response_body: written_note)

    note = @account.my_notes.get_my_note

    assert_equal 5, note["id"]
    assert_equal "Notebook::Note", note["type"]
  end

  def test_get_my_note_pre_first_write_nulls
    stub_get("/12345/my/notes.json",
             response_body: written_note.merge("id" => nil, "created_at" => nil, "updated_at" => nil, "content" => ""))

    note = @account.my_notes.get_my_note

    assert_nil note["id"]
    assert_nil note["created_at"]
    assert_nil note["updated_at"]
    assert_equal "", note["content"]
  end

  def test_update_my_note_sends_nested_envelope
    stub_request(:put, "https://3.basecampapi.com/12345/my/notes.json")
      .with(body: { "note" => { "content" => "<div>Updated</div>" } }.to_json)
      .to_return(status: 200, body: written_note.merge("content" => "<div>Updated</div>").to_json,
                 headers: { "Content-Type" => "application/json" })

    note = @account.my_notes.update_my_note(note: { "content" => "<div>Updated</div>" })

    assert_equal "<div>Updated</div>", note["content"]
  end

  def test_update_my_note_raises_validation_error_on_422
    stub_request(:put, "https://3.basecampapi.com/12345/my/notes.json")
      .to_return(status: 422, body: { "error" => "Unprocessable" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::ValidationError) do
      @account.my_notes.update_my_note(note: { "content" => "x" })
    end
  end
end
