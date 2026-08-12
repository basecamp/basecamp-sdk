# frozen_string_literal: true

require "test_helper"

class WebhooksServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list
    response = [ { "id" => 1, "payload_url" => "https://example.com/webhook", "active" => true } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/buckets/\d+/webhooks\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.webhooks.list(bucket_id: 1).to_a
    assert_kind_of Array, result
    assert_equal "https://example.com/webhook", result.first["payload_url"]
  end

  def test_list_operation_metadata
    events = []
    account = create_account_client(account_id: "12345", hooks: CapturingHooks.new(events))

    response = [ { "id" => 1, "payload_url" => "https://example.com/webhook", "active" => true } ]
    stub_request(:get, %r{https://3\.basecampapi\.com/12345/buckets/\d+/webhooks\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    # Paginated operations fire hooks on iteration, so drain the enumerator.
    account.webhooks.list(bucket_id: 1).to_a

    event = events.find { |e| e[:event] == :on_operation_start }
    assert event, "Expected on_operation_start to fire"
    info = event[:info]
    assert_equal "webhooks", info.service
    assert_equal "list", info.operation
    assert_equal 1, info.project_id
    assert_nil info.resource_id
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

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/buckets/\d+/webhooks\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.webhooks.create(
      bucket_id: 1,
      payload_url: "https://example.com/webhook",
      types: [ "Todo" ]
    )
    assert_equal [ "Todo" ], result["types"]
  end

  # webhooks_controller.rb:31 renders `json: @webhook.errors` at 400 — the field
  # map with no "errors" wrapper (SPEC section 6 step 2).
  def test_create_surfaces_bare_field_map_400
    body = { "payload_url" => [ "is not a valid URL" ], "types" => [ "is invalid" ] }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/buckets/\d+/webhooks\.json})
      .to_return(status: 400, body: body.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::ValidationError) do
      @account.webhooks.create(bucket_id: 1, payload_url: "https://example.com/hook", types: [ "Todo" ])
    end

    assert_equal "payload_url: is not a valid URL, types: is invalid", error.message
    assert_equal body, error.field_errors
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
    # UTF-8 regardless of process locale (LC_ALL=C would otherwise read as US-ASCII)
    path = File.expand_path("../../../../spec/fixtures/webhooks/get.json", __dir__)
    fixture = JSON.parse(File.read(path, encoding: "UTF-8"))

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/webhooks/\d+})
      .to_return(status: 200, body: fixture.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.webhooks.get(webhook_id: 9007199254741433)
    assert_equal 1, result["recent_deliveries"].length

    delivery = result["recent_deliveries"].first
    assert_equal 1230, delivery["id"]
    assert_equal "todo_created", delivery["request"]["body"]["kind"]
    assert_equal 200, delivery["response"]["code"]
  end

  private

  # Captures the OperationInfo passed to on_operation_start so tests can
  # assert the metadata emitted by the real generated service call.
  class CapturingHooks
    include Basecamp::Hooks

    def initialize(events)
      @events = events
    end

    def on_operation_start(info)
      @events << { event: :on_operation_start, info: info }
    end
  end
end
