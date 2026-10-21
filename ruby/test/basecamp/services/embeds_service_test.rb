# frozen_string_literal: true

require "test_helper"

class EmbedsServiceTest < Minitest::Test
  include TestHelper

  YOUTUBE_URL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
  SLASHED_SGID = JSON.parse(File.read(File.expand_path("../../../../spec/fixtures/embeds/pending.json", __dir__)))["sgid"]

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_create
    stub = stub_request(:post, "https://3.basecampapi.com/12345/embeds.json")
      .with(body: { url: YOUTUBE_URL }.to_json)
      .to_return(status: 200, body: load_fixture("embeds/pending.json").to_json, headers: { "Content-Type" => "application/json" })

    embed = @account.embeds.create(url: YOUTUBE_URL)

    assert_requested stub
    assert_equal "pending", embed["status"]
    assert_equal SLASHED_SGID, embed["sgid"]
    assert_not embed.key?("content_type")
  end

  def test_create_returns_an_existing_active_embed
    stub_post("/12345/embeds.json", status: 200, response_body: load_fixture("embeds/active.json"))

    embed = @account.embeds.create(url: YOUTUBE_URL)

    assert_equal "active", embed["status"]
    assert_equal "embed/youtube-video", embed["content_type"]
    assert_nil embed["image_url"]
  end

  def test_create_refuses_a_url_no_provider_embeds
    stub_post("/12345/embeds.json", status: 422, response_body: { "error" => "Basecamp can't embed this URL. Link to it instead." })

    error = assert_raises(Basecamp::ValidationError) { @account.embeds.create(url: "https://example.com/page") }
    assert_equal 422, error.http_status
    assert_equal "Basecamp can't embed this URL. Link to it instead.", error.message
  end

  def test_create_rate_limited
    stub_post("/12345/embeds.json", status: 429, response_body: { "error" => "Too many new embeds. Try again later." })

    error = assert_raises(Basecamp::RateLimitError) { @account.embeds.create(url: YOUTUBE_URL) }
    assert_equal 429, error.http_status
  end

  def test_get_escapes_the_sgid_as_one_path_segment
    escaped = SLASHED_SGID.gsub("/", "%2F").gsub("=", "%3D")
    stub = stub_request(:get, "https://3.basecampapi.com/12345/embeds/#{escaped}")
      .to_return(status: 200, body: load_fixture("embeds/active.json").to_json, headers: { "Content-Type" => "application/json" })

    embed = @account.embeds.get(sgid: SLASHED_SGID)

    assert_requested stub
    assert_equal "active", embed["status"]
  end

  def test_get_not_found
    stub_get("/12345/embeds/missing", status: 404, response_body: { "error" => "Not found" })

    assert_raises(Basecamp::NotFoundError) { @account.embeds.get(sgid: "missing") }
  end
end
