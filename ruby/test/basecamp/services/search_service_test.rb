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
      .with(query: { query: "quarterly report" })
      .to_return(status: 200, body: results.to_json)

    result = @account.search.search(query: "quarterly report").to_a

    assert_equal 2, result.length
    assert_equal "Quarterly Report", result[0]["title"]
  end

  def test_search_with_sort
    results = [ { "id" => 3, "title" => "Recent Doc", "type" => "Document" } ]
    stub_request(:get, "https://3.basecampapi.com/12345/search.json")
      .with(query: { query: "doc", sort: "updated_at" })
      .to_return(status: 200, body: results.to_json)

    result = @account.search.search(query: "doc", sort: "updated_at").to_a

    assert_equal 1, result.length
  end

  def test_metadata
    metadata = {
      "projects" => [
        { "id" => 100, "name" => "Project A" },
        { "id" => 200, "name" => "Project B" }
      ]
    }
    stub_get("/12345/searches/metadata.json", response_body: metadata)

    result = @account.search.metadata

    assert_equal 2, result["projects"].length
    assert_equal "Project A", result["projects"][0]["name"]
  end
end
