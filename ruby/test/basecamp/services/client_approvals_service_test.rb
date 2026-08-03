# frozen_string_literal: true

# Tests for the ClientApprovalsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Single-resource paths without .json (get)

require "test_helper"

class ClientApprovalsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_approval(id: nil, subject: nil)
    fixture = load_fixture("client_approvals/get.json")
    fixture.merge "id" => id || fixture["id"], "subject" => subject || fixture["subject"]
  end

  def test_list_approvals
    stub_get("/12345/buckets/100/client/approvals.json",
             response_body: [ sample_approval, sample_approval(id: 2, subject: "Budget Approval") ])

    approvals = @account.client_approvals.list(bucket_id: 100).to_a

    assert_equal 2, approvals.length
    assert_equal "New logo for the website", approvals[0]["subject"]
    assert_equal "Budget Approval", approvals[1]["subject"]
  end

  def test_get_approval
    # Generated service: /client/approvals/{id} without .json
    stub_get("/12345/client/approvals/200", response_body: sample_approval(id: 200))

    approval = @account.client_approvals.get(approval_id: 200)

    assert_equal 200, approval["id"]
    assert_equal "New logo for the website", approval["subject"]
    assert_equal "approved", approval["approval_status"]
  end
end
