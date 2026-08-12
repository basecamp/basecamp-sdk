# frozen_string_literal: true

require "test_helper"

class SearchServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_search
    results = [
      # The search projection carries the matching type's rich-text companion
      # array; a Message result surfaces content_attachments.
      # BC3 emits `json.content nil` / `json.description nil` unconditionally on
      # every search result, so both keys are always present and always null. A
      # stub that omits them is a payload the API cannot produce.
      { "id" => 1, "title" => "Quarterly Report", "type" => "Message", "content_attachments" => [],
        "url" => "https://3.basecampapi.com/12345/buckets/1/messages/1.json",
        "app_url" => "https://3.basecamp.com/12345/buckets/1/messages/1",
        "content" => nil, "description" => nil,
        "plain_text_content" => "Q1 <mark class=\"circled-text\"><span></span>Report</mark> summary." },
      { "id" => 2, "title" => "Q1 Report Draft", "type" => "Document", "content_attachments" => [],
        "url" => "https://3.basecampapi.com/12345/buckets/1/documents/2.json",
        "app_url" => "https://3.basecamp.com/12345/buckets/1/documents/2",
        "content" => nil, "description" => nil },
      # A file-attachment hit (searches/_attachment.json.jbuilder): the one
      # branch that omits the id/title/type/url/app_url envelope keys and
      # carries the ten file keys instead. width/height ride only on
      # previewable files and may arrive float-spelled (1920.0).
      { "parent" => { "id" => 10, "title" => "Message Board", "type" => "Message",
                      "url" => "https://3.basecampapi.com/12345/buckets/1/messages/11.json",
                      "app_url" => "https://3.basecamp.com/12345/buckets/1/messages/11" },
        "bucket" => { "id" => 1, "name" => "Leto", "type" => "Project" },
        "created_at" => "2022-10-28T15:25:00.000Z",
        "filename" => "leto-hero.jpg", "content_type" => "image/jpeg",
        "byte_size" => 512000, "previewable" => true,
        "width" => 1920.0, "height" => 1080,
        "preview_url" => "https://3.basecampapi.com/12345/blobs/hero/previews/leto-hero.jpg",
        "thumbnail_url" => "https://3.basecampapi.com/12345/blobs/hero/thumbnails/leto-hero.jpg",
        "download_url" => "https://3.basecampapi.com/12345/blobs/hero/download/leto-hero.jpg",
        "app_download_url" => "https://3.basecamp.com/12345/blobs/hero/download/leto-hero.jpg",
        "content" => nil, "description" => nil }
    ]
    stub_request(:get, "https://3.basecampapi.com/12345/search.json")
      .with(query: { q: "quarterly report" })
      .to_return(status: 200, body: results.to_json)

    result = @account.search.search(q: "quarterly report").to_a

    assert_equal 3, result.length
    assert_equal "Quarterly Report", result[0]["title"]
    # The optional projection array surfaces on each matching-type result.
    assert_equal [], result[0]["content_attachments"]
    assert_equal [], result[1]["content_attachments"]

    # Present and null, never absent.
    result.each do |r|
      assert r.key?("content"), "content must be present on every search result"
      assert_nil r["content"]
      assert r.key?("description"), "description must be present on every search result"
      assert_nil r["description"]
    end
    assert_includes result[0]["plain_text_content"], "circled-text"
    assert_nil result[1]["plain_text_content"]

    # The file-attachment hit: no envelope keys, file keys instead.
    hit = result[2]
    %w[id title type url app_url].each do |key|
      assert_not hit.key?(key), "attachment hit must omit #{key}"
    end
    assert_equal "leto-hero.jpg", hit["filename"]
    assert_equal "image/jpeg", hit["content_type"]
    assert_equal 512000, hit["byte_size"]
    assert hit["previewable"]
    assert_equal 1920, hit["width"]
    assert_equal 1080, hit["height"]
    assert_includes hit["preview_url"], "/previews/"
    assert_includes hit["thumbnail_url"], "/thumbnails/"
    assert_includes hit["download_url"], "/download/"
    assert_includes hit["app_download_url"], "/download/"
    assert_equal "Message", hit.dig("parent", "type")
  end

  def test_search_with_sort
    results = [ { "id" => 3, "title" => "Recent Doc", "type" => "Document" } ]
    stub_request(:get, "https://3.basecampapi.com/12345/search.json")
      .with(query: { q: "doc", sort: "best_match" })
      .to_return(status: 200, body: results.to_json)

    result = @account.search.search(q: "doc", sort: "best_match").to_a

    assert_equal 1, result.length
  end

  # Faraday's NestedParamsEncoder serializes an array kwarg keyed by the clean
  # name (bucket_ids) into the bracketed repeated form (bucket_ids[]=1&...),
  # which is the only form Rails' permit(bucket_ids: []) accepts. The exact
  # query match also proves no bare `bucket_ids` key leaks through.
  def test_search_encodes_array_filters_as_bracketed_keys
    # WebMock encodes an array-valued hash key into the bracketed repeated form
    # (bucket_ids => [1,2] ⇒ bucket_ids[]=1&bucket_ids[]=2), matching the wire.
    # The exact (non-hash_including) match proves no bare key leaks through.
    stub_request(:get, "https://3.basecampapi.com/12345/search.json")
      .with(query: {
        "q" => "hello",
        "bucket_ids" => %w[1 2],
        "type_names" => %w[Message Todo],
        "creator_ids" => %w[7]
      })
      .to_return(status: 200, body: [].to_json)

    result = @account.search.search(
      q: "hello",
      bucket_ids: [ 1, 2 ],
      type_names: %w[Message Todo],
      creator_ids: [ 7 ]
    ).to_a

    assert_equal 0, result.length
  end

  # An empty array filter means "no filter" and must be omitted entirely — a
  # bare `bucket_ids[]` would be normalized to a bogus [0] project filter by
  # Rails. compact_query_params drops empty arrays (unlike compact_params).
  def test_search_omits_empty_array_filters
    stub_request(:get, "https://3.basecampapi.com/12345/search.json")
      .with(query: { "q" => "hello", "type_names" => %w[Message] })
      .to_return(status: 200, body: [].to_json)

    result = @account.search.search(
      q: "hello",
      type_names: %w[Message],
      bucket_ids: [],
      creator_ids: []
    ).to_a

    assert_equal 0, result.length
  end

  # Exercises the full filter surface: arrays, scalars, and deprecated singulars.
  def test_search_encodes_full_filter_surface
    stub_request(:get, "https://3.basecampapi.com/12345/search.json")
      .with(query: {
        "q" => "hello",
        "bucket_ids" => %w[1 2],
        "type_names" => %w[Message],
        "creator_ids" => %w[7],
        "file_type" => "Image",
        "exclude_chat" => "true",
        "since" => "last_30_days",
        "sort" => "recency",
        "type" => "Message",
        "bucket_id" => "9",
        "creator_id" => "3"
      })
      .to_return(status: 200, body: [].to_json)

    result = @account.search.search(
      q: "hello",
      bucket_ids: [ 1, 2 ],
      type_names: %w[Message],
      creator_ids: [ 7 ],
      file_type: "Image",
      exclude_chat: true,
      since: "last_30_days",
      sort: "recency",
      type: "Message",
      bucket_id: 9,
      creator_id: 3
    ).to_a

    assert_equal 0, result.length
  end

  def test_metadata
    metadata = {
      "recording_search_types" => [
        { "key" => nil, "value" => "Everything" },
        { "key" => "Message", "value" => "Messages" }
      ],
      "file_search_types" => [
        { "key" => nil, "value" => "All files" },
        { "key" => "Image", "value" => "Images" }
      ],
      "default_creator_label" => "Anyone",
      "default_bucket_label" => "All projects",
      "default_circle_label" => "All pings",
      "default_file_type_label" => "All files",
      "default_type_label" => "Everything"
    }
    stub_get("/12345/searches/metadata.json", response_body: metadata)

    result = @account.search.metadata

    assert_equal 2, result["recording_search_types"].length
    assert_nil result["recording_search_types"][0]["key"]
    assert_equal "Messages", result["recording_search_types"][1]["value"]
    assert_equal "Image", result["file_search_types"][1]["key"]
    assert_equal "Anyone", result["default_creator_label"]
    assert_equal "Everything", result["default_type_label"]
  end
end
