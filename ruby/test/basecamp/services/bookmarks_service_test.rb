# frozen_string_literal: true

require "test_helper"

class BookmarksServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_bookmark(id:)
    {
      "id" => id,
      "created_at" => "2026-07-01T00:00:00Z",
      "updated_at" => "2026-07-02T00:00:00Z",
      "recording" => {
        "id" => 900,
        "status" => "active",
        "visible_to_clients" => false,
        "created_at" => "2026-06-01T00:00:00Z",
        "updated_at" => "2026-06-02T00:00:00Z",
        "title" => "Kickoff notes",
        "inherits_status" => true,
        "type" => "Document",
        "url" => "https://3.basecampapi.com/12345/buckets/2/documents/900.json",
        "app_url" => "https://3.basecamp.com/12345/buckets/2/documents/900",
        "bucket" => { "id" => 2, "name" => "The Leto Laptop", "type" => "Project" },
        "creator" => { "id" => 1, "name" => "Victor Cooper" }
      }
    }
  end

  def test_list_my_bookmarks
    stub_get("/12345/my/bookmarks.json", response_body: [ sample_bookmark(id: 1), sample_bookmark(id: 2) ])

    result = @account.bookmarks.list_my_bookmarks.to_a

    assert_equal 2, result.length
    assert_equal 1, result[0]["id"]
    assert_equal "Kickoff notes", result[0]["recording"]["title"]
  end

  def test_list_my_bookmarks_raises_on_401
    stub_request(:get, "https://3.basecampapi.com/12345/my/bookmarks.json")
      .to_return(status: 401, body: { "error" => "Unauthorized" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::AuthError) do
      @account.bookmarks.list_my_bookmarks.to_a
    end
  end

  def test_get_bookmark_reports_state
    stub_get("/12345/recordings/900/bookmark.json", response_body: { "bookmarked" => true })

    status = @account.bookmarks.get_bookmark(recording_id: 900)

    assert_equal true, status["bookmarked"]
  end

  def test_get_bookmark_raises_not_found
    stub_request(:get, "https://3.basecampapi.com/12345/recordings/999/bookmark.json")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) do
      @account.bookmarks.get_bookmark(recording_id: 999)
    end
  end

  def test_create_bookmark_returns_envelope
    stub_post("/12345/recordings/900/bookmark.json", response_body: sample_bookmark(id: 7))

    bookmark = @account.bookmarks.create_bookmark(recording_id: 900)

    assert_equal 7, bookmark["id"]
    assert_equal 900, bookmark["recording"]["id"]
  end

  def test_create_bookmark_raises_not_found
    stub_request(:post, "https://3.basecampapi.com/12345/recordings/999/bookmark.json")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) do
      @account.bookmarks.create_bookmark(recording_id: 999)
    end
  end

  def test_delete_bookmark_returns_no_content
    stub_request(:delete, "https://3.basecampapi.com/12345/recordings/900/bookmark.json")
      .to_return(status: 204, body: "")

    @account.bookmarks.delete_bookmark(recording_id: 900)

    assert_requested(:delete, "https://3.basecampapi.com/12345/recordings/900/bookmark.json")
  end

  def test_delete_bookmark_raises_forbidden
    stub_request(:delete, "https://3.basecampapi.com/12345/recordings/900/bookmark.json")
      .to_return(status: 403, body: { "error" => "Forbidden" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::ForbiddenError) do
      @account.bookmarks.delete_bookmark(recording_id: 900)
    end
  end
end
