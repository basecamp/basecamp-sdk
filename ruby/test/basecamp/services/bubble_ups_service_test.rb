# frozen_string_literal: true

require "test_helper"

class BubbleUpsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_create_bubble_up_schedules_with_at
    stub_request(:post, "https://3.basecampapi.com/12345/recordings/900/bubble_up.json")
      .to_return(status: 204, body: "")

    @account.bubble_ups.create_bubble_up(recording_id: 900, at: "2026-09-10T09:00:00Z")

    assert_requested(:post, "https://3.basecampapi.com/12345/recordings/900/bubble_up.json") do |req|
      JSON.parse(req.body) == { "at" => "2026-09-10T09:00:00Z" }
    end
  end

  def test_create_bubble_up_omits_at_when_absent
    stub_request(:post, "https://3.basecampapi.com/12345/recordings/900/bubble_up.json")
      .to_return(status: 204, body: "")

    @account.bubble_ups.create_bubble_up(recording_id: 900)

    assert_requested(:post, "https://3.basecampapi.com/12345/recordings/900/bubble_up.json") do |req|
      !JSON.parse(req.body).key?("at")
    end
  end

  def test_create_bubble_up_raises_not_found
    stub_request(:post, "https://3.basecampapi.com/12345/recordings/999/bubble_up.json")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) do
      @account.bubble_ups.create_bubble_up(recording_id: 999)
    end
  end

  def test_delete_bubble_up_returns_no_content
    stub_request(:delete, "https://3.basecampapi.com/12345/recordings/900/bubble_up.json")
      .to_return(status: 204, body: "")

    @account.bubble_ups.delete_bubble_up(recording_id: 900)

    assert_requested(:delete, "https://3.basecampapi.com/12345/recordings/900/bubble_up.json")
  end

  def test_delete_bubble_up_raises_forbidden
    stub_request(:delete, "https://3.basecampapi.com/12345/recordings/900/bubble_up.json")
      .to_return(status: 403, body: { "error" => "Forbidden" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::ForbiddenError) do
      @account.bubble_ups.delete_bubble_up(recording_id: 900)
    end
  end
end
