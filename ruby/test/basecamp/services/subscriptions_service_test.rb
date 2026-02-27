# frozen_string_literal: true

require "test_helper"

class SubscriptionsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_get
    response = {
      "subscribed" => true,
      "count" => 5,
      "subscribers" => []
    }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/recordings/\d+/subscription\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.subscriptions.get(recording_id: 2)
    assert_equal true, result["subscribed"]
    assert_equal 5, result["count"]
  end

  def test_subscribe
    response = { "subscribed" => true }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/recordings/\d+/subscription\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.subscriptions.subscribe(recording_id: 2)
    assert_equal true, result["subscribed"]
  end

  def test_unsubscribe
    stub_request(:delete, %r{https://3\.basecampapi\.com/12345/recordings/\d+/subscription\.json})
      .to_return(status: 204)

    result = @account.subscriptions.unsubscribe(recording_id: 2)
    assert_nil result
  end

  def test_update
    response = { "subscribed" => true, "count" => 2 }

    stub_request(:put, %r{https://3\.basecampapi\.com/12345/recordings/\d+/subscription\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.subscriptions.update(
      recording_id: 2,
      subscriptions: [ 100, 101 ]
    )
    assert_equal 2, result["count"]
  end
end
