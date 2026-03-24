# frozen_string_literal: true

require "test_helper"

class AccountServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_get_account_deserializes_logo_object
    stub_get("/12345/account.json", response_body: {
      "id" => 3,
      "name" => "37signals",
      "created_at" => "2024-01-01T00:00:00Z",
      "updated_at" => "2024-01-01T00:00:00Z",
      "logo" => { "url" => "https://3.basecampapi.com/2914079/account/logo?v=1650492527" }
    })

    account = @account.account.get_account

    assert_kind_of Hash, account["logo"]
    assert_equal "https://3.basecampapi.com/2914079/account/logo?v=1650492527", account["logo"]["url"]
  end

  def test_get_account_nil_logo_when_absent
    stub_get("/12345/account.json", response_body: {
      "id" => 3,
      "name" => "37signals",
      "created_at" => "2024-01-01T00:00:00Z",
      "updated_at" => "2024-01-01T00:00:00Z"
    })

    account = @account.account.get_account

    assert_nil account["logo"]
  end
end
