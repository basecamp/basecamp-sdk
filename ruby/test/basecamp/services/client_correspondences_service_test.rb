# frozen_string_literal: true

# Tests for the ClientCorrespondencesService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Single-resource paths without .json (get)

require "test_helper"

class ClientCorrespondencesServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_correspondence(id: nil, subject: nil)
    fixture = load_fixture("client_correspondences/get.json")
    fixture.merge "id" => id || fixture["id"], "subject" => subject || fixture["subject"]
  end

  def test_list_correspondences
    stub_get("/12345/buckets/100/client/correspondences.json",
             response_body: [ sample_correspondence, sample_correspondence(id: 2, subject: "Invoice Query") ])

    correspondences = @account.client_correspondences.list(bucket_id: 100).to_a

    assert_equal 2, correspondences.length
    assert_equal "Project kickoff!", correspondences[0]["subject"]
    assert_equal "Invoice Query", correspondences[1]["subject"]
  end

  def test_get_correspondence
    # Generated service: /client/correspondences/{id} without .json
    stub_get("/12345/client/correspondences/200", response_body: sample_correspondence(id: 200))

    correspondence = @account.client_correspondences.get(correspondence_id: 200)

    assert_equal 200, correspondence["id"]
    assert_equal "Project kickoff!", correspondence["subject"]
    assert_equal 5, correspondence["replies_count"]
  end
end
