# frozen_string_literal: true

# Tests for the DocumentsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Single-resource paths without .json (get, update)

require "test_helper"

class DocumentsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_document(id: 1, title: "Meeting Notes")
    {
      "id" => id,
      "title" => title,
      "content" => "<p>Notes from today's meeting...</p>",
      "status" => "active",
      "comments_count" => 2,
      "created_at" => "2024-01-01T00:00:00Z"
    }
  end

  def test_list_documents
    stub_get("/12345/vaults/200/documents.json",
             response_body: [ sample_document, sample_document(id: 2, title: "Project Plan") ])

    documents = @account.documents.list(vault_id: 200).to_a

    assert_equal 2, documents.length
    assert_equal "Meeting Notes", documents[0]["title"]
    assert_equal "Project Plan", documents[1]["title"]
  end

  def test_get_document
    # Generated service: /documents/{id} without .json
    stub_get("/12345/documents/200", response_body: sample_document(id: 200))

    document = @account.documents.get(document_id: 200)

    assert_equal 200, document["id"]
    assert_equal "Meeting Notes", document["title"]
  end

  def test_create_document
    new_document = sample_document(id: 999, title: "New Document")
    stub_post("/12345/vaults/200/documents.json", response_body: new_document)

    document = @account.documents.create(
      vault_id: 200,
      title: "New Document",
      content: "<p>Document content</p>",
      status: "active"
    )

    assert_equal 999, document["id"]
    assert_equal "New Document", document["title"]
  end

  def test_create_draft_document
    draft_document = sample_document(id: 999, title: "Draft Document")
    draft_document["status"] = "drafted"
    stub_post("/12345/vaults/200/documents.json", response_body: draft_document)

    document = @account.documents.create(
      vault_id: 200,
      title: "Draft Document",
      status: "drafted"
    )

    assert_equal "drafted", document["status"]
  end

  def test_update_document
    # Generated service: /documents/{id} without .json
    updated_document = sample_document(id: 200, title: "Updated Title")
    stub_put("/12345/documents/200", response_body: updated_document)

    document = @account.documents.update(
      document_id: 200,
      title: "Updated Title",
      content: "<p>New content</p>"
    )

    assert_equal "Updated Title", document["title"]
  end
end
