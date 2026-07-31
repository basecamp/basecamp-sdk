# frozen_string_literal: true

require "test_helper"

class DraftsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_draft(id:, overrides: {})
    {
      "id" => id,
      "app_url" => "https://3.basecamp.com/12345/buckets/2/documents/#{id}",
      "title" => "Quarterly plan",
      "type" => "document",
      "bucket" => { "id" => 2, "name" => "The Leto Laptop", "app_url" => "https://3.basecamp.com/12345/projects/2" },
      "parent" => { "id" => 500, "title" => "Docs & Files", "app_url" => "https://3.basecamp.com/12345/buckets/2/vaults/500" },
      "excerpt" => "First 300 chars of the body",
      "created_at" => "2026-07-01T00:00:00Z",
      "updated_at" => "2026-07-02T00:00:00Z",
      "scheduled_posting_at" => nil
    }.merge(overrides)
  end

  def test_list_my_drafts_includes_null_parent_and_schedule
    drafts = [
      sample_draft(id: 1),
      sample_draft(id: 2, overrides: { "parent" => nil, "scheduled_posting_at" => "2026-08-01T09:00:00Z" })
    ]
    stub_get("/12345/my/drafts.json", response_body: drafts)

    result = @account.drafts.list_my_drafts.to_a

    assert_equal 2, result.length
    assert_equal "Docs & Files", result[0]["parent"]["title"]
    assert_nil result[0]["scheduled_posting_at"]
    assert_nil result[1]["parent"]
    assert_equal "2026-08-01T09:00:00Z", result[1]["scheduled_posting_at"]
  end

  def test_list_my_drafts_raises_on_401
    stub_request(:get, "https://3.basecampapi.com/12345/my/drafts.json")
      .to_return(status: 401, body: { "error" => "Unauthorized" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::AuthError) do
      @account.drafts.list_my_drafts.to_a
    end
  end
end
