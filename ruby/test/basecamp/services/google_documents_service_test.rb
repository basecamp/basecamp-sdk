# frozen_string_literal: true

# Tests for the GoogleDocumentsService (generated from the OpenAPI spec).
#
# The create path is the load-bearing assertion: bc3 draws google_documents
# under `resources :vaults` only inside the bucket scope, so create is
# /buckets/{bucket_id}/vaults/{vault_id}/google_documents.json while get and
# update are flat and unscoped (and, like the other generated single-resource
# routes, carry no .json suffix). A wrong spelling is a live 404, so the stubs
# are anchored rather than loose.

require "test_helper"

class GoogleDocumentsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def google_document
    {
      "id" => 2002,
      "status" => "active",
      "title" => "Roadmap (draft)",
      "type" => "GoogleDocument",
      "url" => "https://docs.google.com/document/d/abcd1234/edit",
      "app_url" => "https://3.basecamp.com/12345/buckets/2085958500/google_documents/2002",
      "description" => "<div dir=\"auto\">Quarterly roadmap</div>",
      "description_attachments" => [],
      "document_type" => "doc"
    }
  end

  def test_get_google_document
    stub_request(:get, "https://3.basecampapi.com/12345/google_documents/2002")
      .to_return(status: 200, body: google_document.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.google_documents.get_google_document(google_document_id: 2002)

    assert_equal "Roadmap (draft)", result["title"]
    assert_equal "doc", result["document_type"]
    assert_equal "https://docs.google.com/document/d/abcd1234/edit", result["url"]
  end

  def test_get_google_document_raises_not_found
    stub_request(:get, "https://3.basecampapi.com/12345/google_documents/9999")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) do
      @account.google_documents.get_google_document(google_document_id: 9999)
    end
  end

  def test_create_google_document_posts_to_the_bucket_scoped_vault_nested_path
    stub_request(:post, "https://3.basecampapi.com/12345/buckets/2085958500/vaults/3001/google_documents.json")
      .with(body: hash_including("url" => "https://docs.google.com/document/d/abcd1234/edit", "document_type" => "doc"))
      .to_return(status: 201, body: google_document.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.google_documents.create_google_document(
      bucket_id: 2085958500, vault_id: 3001,
      url: "https://docs.google.com/document/d/abcd1234/edit", document_type: "doc", title: "Roadmap"
    )

    assert_equal 2002, result["id"]
  end

  # bc3 rejects an unrecognized document_type in a before_action, since the Rails
  # enum would raise rather than add a validation error. It renders the wrapped
  # field-keyed 422 with a literal hash.
  def test_create_google_document_surfaces_the_document_type_rejection
    stub_request(:post, "https://3.basecampapi.com/12345/buckets/2085958500/vaults/3001/google_documents.json")
      .to_return(status: 422,
                 body: { "errors" => { "document_type" => [ "is not a valid document type" ] } }.to_json,
                 headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::ValidationError) do
      @account.google_documents.create_google_document(
        bucket_id: 2085958500, vault_id: 3001,
        url: "https://docs.google.com/document/d/abcd1234/edit", document_type: "bogus"
      )
    end

    assert_equal({ "document_type" => [ "is not a valid document type" ] }, error.field_errors)
  end

  def test_update_google_document_puts_to_the_flat_path
    updated = google_document.merge("title" => "Roadmap (revised)")

    stub_request(:put, "https://3.basecampapi.com/12345/google_documents/2002")
      .with(body: hash_including("document_type" => "doc"))
      .to_return(status: 200, body: updated.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.google_documents.update_google_document(
      google_document_id: 2002,
      url: "https://docs.google.com/document/d/abcd1234/edit", document_type: "doc", title: "Roadmap (revised)"
    )

    assert_equal "Roadmap (revised)", result["title"]
  end

  def test_update_google_document_raises_not_found
    stub_request(:put, "https://3.basecampapi.com/12345/google_documents/9999")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) do
      @account.google_documents.update_google_document(
        google_document_id: 9999, url: "https://docs.google.com/d/x", document_type: "doc"
      )
    end
  end
end
