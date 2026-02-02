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

  def sample_approval(id: 1, subject: "Design Review")
    {
      "id" => id,
      "subject" => subject,
      "approval_status" => "pending",
      "content" => "<p>Please review the attached designs.</p>",
      "created_at" => "2024-01-01T00:00:00Z"
    }
  end

  def test_list_approvals
    stub_get("/12345/client/approvals.json",
             response_body: [ sample_approval, sample_approval(id: 2, subject: "Budget Approval") ])

    approvals = @account.client_approvals.list.to_a

    assert_equal 2, approvals.length
    assert_equal "Design Review", approvals[0]["subject"]
    assert_equal "Budget Approval", approvals[1]["subject"]
  end

  def test_get_approval
    # Generated service: /client/approvals/{id} without .json
    stub_get("/12345/client/approvals/200", response_body: sample_approval(id: 200))

    approval = @account.client_approvals.get(approval_id: 200)

    assert_equal 200, approval["id"]
    assert_equal "Design Review", approval["subject"]
    assert_equal "pending", approval["approval_status"]
  end
end
