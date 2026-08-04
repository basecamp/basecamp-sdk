# frozen_string_literal: true

# Tests for the CloudFilesService (generated from the OpenAPI spec).
#
# The create path is the load-bearing assertion: bc3 draws cloud_files under
# `resources :vaults` only inside the bucket scope, so create is
# /buckets/{bucket_id}/vaults/{vault_id}/cloud_files.json while get and update are
# flat and unscoped (and, like the other generated single-resource routes,
# carry no .json suffix). A wrong spelling is a live 404, so the stubs are
# anchored rather than loose.

require "test_helper"

class CloudFilesServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def cloud_file
    {
      "id" => 1001,
      "status" => "active",
      "title" => "Brand book draft",
      "type" => "CloudFile",
      # The EXTERNAL link, not this record's API url — the cloud_files jbuilder
      # renders the recording partial and then overwrites :url with the
      # recordable's.
      "url" => "https://www.dropbox.com/s/abcd1234/brand-draft.pdf",
      "app_url" => "https://3.basecamp.com/12345/buckets/2085958500/cloud_files/1001",
      "description" => "<div dir=\"auto\">Working draft</div>",
      "description_attachments" => [],
      "service" => {
        "name" => "Dropbox",
        "example_url" => "https://www.dropbox.com/s/abcd1234/example.pdf",
        "code" => "dropbox",
        "valid_patterns" => [ "(.*?\\.)?dropbox\\.com(\\/.*)?" ],
        "supporting_text" => "a file or folder on Dropbox"
      }
    }
  end

  def test_get_cloud_file
    stub_request(:get, "https://3.basecampapi.com/12345/cloud_files/1001")
      .to_return(status: 200, body: cloud_file.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.cloud_files.get_cloud_file(cloud_file_id: 1001)

    assert_equal "Brand book draft", result["title"]
    assert_equal "https://www.dropbox.com/s/abcd1234/brand-draft.pdf", result["url"]
    assert_equal "dropbox", result.dig("service", "code")
  end

  def test_get_cloud_file_raises_not_found
    stub_request(:get, "https://3.basecampapi.com/12345/cloud_files/9999")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) do
      @account.cloud_files.get_cloud_file(cloud_file_id: 9999)
    end
  end

  def test_create_cloud_file_posts_to_the_bucket_scoped_vault_nested_path
    stub_request(:post, "https://3.basecampapi.com/12345/buckets/2085958500/vaults/3001/cloud_files.json")
      .with(body: hash_including("url" => "https://www.dropbox.com/s/abcd1234/brand.zip", "service" => "dropbox"))
      .to_return(status: 201, body: cloud_file.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.cloud_files.create_cloud_file(
      bucket_id: 2085958500, vault_id: 3001,
      url: "https://www.dropbox.com/s/abcd1234/brand.zip", service: "dropbox", title: "Brand assets"
    )

    assert_equal 1001, result["id"]
  end

  def test_create_cloud_file_surfaces_the_field_keyed_422
    stub_request(:post, "https://3.basecampapi.com/12345/buckets/2085958500/vaults/3001/cloud_files.json")
      .to_return(status: 422, body: { "errors" => { "url" => [ "is not a valid Dropbox url" ] } }.to_json,
                 headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::ValidationError) do
      @account.cloud_files.create_cloud_file(
        bucket_id: 2085958500, vault_id: 3001,
        url: "https://example.com/nope", service: "dropbox"
      )
    end

    assert_equal({ "url" => [ "is not a valid Dropbox url" ] }, error.field_errors)
  end

  def test_update_cloud_file_puts_to_the_flat_path
    updated = cloud_file.merge("title" => "Brand assets v2")

    stub_request(:put, "https://3.basecampapi.com/12345/cloud_files/1001")
      .with(body: hash_including("url" => "https://www.dropbox.com/s/abcd1234/brand-v2.zip", "service" => "dropbox"))
      .to_return(status: 200, body: updated.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.cloud_files.update_cloud_file(
      cloud_file_id: 1001,
      url: "https://www.dropbox.com/s/abcd1234/brand-v2.zip", service: "dropbox", title: "Brand assets v2"
    )

    assert_equal "Brand assets v2", result["title"]
  end

  def test_update_cloud_file_raises_not_found
    stub_request(:put, "https://3.basecampapi.com/12345/cloud_files/9999")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) do
      @account.cloud_files.update_cloud_file(
        cloud_file_id: 9999, url: "https://www.dropbox.com/s/a/b.pdf", service: "dropbox"
      )
    end
  end
end
