# frozen_string_literal: true

require "test_helper"

class WebhooksServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list
    response = [ { "id" => 1, "payload_url" => "https://example.com/webhook", "active" => true } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/webhooks\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.webhooks.list.to_a
    assert_kind_of Array, result
    assert_equal "https://example.com/webhook", result.first["payload_url"]
  end

  def test_get
    response = { "id" => 1, "payload_url" => "https://example.com/webhook" }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/webhooks/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.webhooks.get(webhook_id: 2)
    assert_equal "https://example.com/webhook", result["payload_url"]
  end

  def test_create
    response = { "id" => 1, "payload_url" => "https://example.com/webhook", "types" => [ "Todo" ] }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/webhooks\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.webhooks.create(
      payload_url: "https://example.com/webhook",
      types: [ "Todo" ]
    )
    assert_equal [ "Todo" ], result["types"]
  end

  def test_update
    response = { "id" => 1, "active" => false }

    stub_request(:put, %r{https://3\.basecampapi\.com/12345/webhooks/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.webhooks.update(webhook_id: 2, active: false)
    assert_equal false, result["active"]
  end

  def test_delete
    stub_request(:delete, %r{https://3\.basecampapi\.com/12345/webhooks/\d+})
      .to_return(status: 204)

    result = @account.webhooks.delete(webhook_id: 2)
    assert_nil result
  end

  def test_get_with_recent_deliveries
    fixture = JSON.parse(File.read(File.expand_path("../../../../spec/fixtures/webhooks/get.json", __dir__)))

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/webhooks/\d+})
      .to_return(status: 200, body: fixture.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.webhooks.get(webhook_id: 9007199254741433)
    assert_equal 1, result["recent_deliveries"].length

    delivery = result["recent_deliveries"].first
    assert_equal 1230, delivery["id"]
    assert_equal "todo_created", delivery["request"]["body"]["kind"]
    assert_equal 200, delivery["response"]["code"]
  end
end
