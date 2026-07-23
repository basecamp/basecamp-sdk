# frozen_string_literal: true

require "test_helper"

class SearchServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_search
    results = [
      { "id" => 1, "title" => "Quarterly Report", "type" => "Message" },
      { "id" => 2, "title" => "Q1 Report Draft", "type" => "Document" }
    ]
    stub_request(:get, "https://3.basecampapi.com/12345/search.json")
      .with(query: { q: "quarterly report" })
      .to_return(status: 200, body: results.to_json)

    result = @account.search.search(q: "quarterly report").to_a

    assert_equal 2, result.length
    assert_equal "Quarterly Report", result[0]["title"]
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
