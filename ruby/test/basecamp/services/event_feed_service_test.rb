# frozen_string_literal: true

require "test_helper"

class EventFeedServiceTest < Minitest::Test
  include TestHelper

  BASE = "https://3.basecampapi.com/12345"

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def feed_event(id, **overrides)
    {
      "id" => id,
      "kind" => "message_created",
      "action" => "created",
      "created_at" => "2026-07-14T06:10:00.159Z",
      "event_type" => "message.created",
      "bucket_id" => 2085958499,
      "creator_id" => 1049715945,
      "performed_by_id" => nil,
      "recording_id" => 1069479766
    }.merge(overrides.transform_keys(&:to_s))
  end

  def json_response(status, body)
    { status: status, body: JSON.generate(body), headers: { "Content-Type" => "application/json" } }
  end

  def test_poll_events_sends_entry_and_filters_and_decodes_the_envelope
    stub_request(:get, "#{BASE}/events.json")
      .with(query: { "since" => "0", "types" => "message.created,boost.created", "buckets" => "2085958499",
                     "exclude_performers" => "self", "actor_types" => "agent,person" })
      .to_return(json_response(200, {
        "events" => [
          feed_event(1071915468),
          feed_event(1071915470, kind: "boost_created", event_type: "boost.created", performed_by_id: 1049715999,
                                 details: { "boost_id" => 501, "boosted_event_id" => 1071915468, "boosted_event_type" => "message.created" })
        ],
        "position" => "posAAA",
        "next" => "https://3.basecampapi.com/12345/events.json?position=posAAA&types=message.created%2Cboost.created"
      }))

    page = @account.event_feed.poll_events(since: "0", types: "message.created,boost.created", buckets: "2085958499",
                                           exclude_performers: "self", actor_types: "agent,person")

    assert_equal "posAAA", page["position"]
    assert_includes page["next"], "position=posAAA"
    assert_equal 2, page["events"].length
    assert_nil page["events"][0]["performed_by_id"]
    assert_not page["events"][0].key?("details")
    assert_equal 1049715999, page["events"][1]["performed_by_id"]
    assert_equal 501, page["events"][1].dig("details", "boost_id")
  end

  def test_poll_events_bare_enters_at_the_present
    stub_request(:get, "#{BASE}/events.json")
      .with { |request| request.uri.query.nil? }
      .to_return(json_response(200, { "events" => [], "position" => "posNOW" }))

    page = @account.event_feed.poll_events

    assert_equal [], page["events"]
    assert_equal "posNOW", page["position"]
    assert_not page.key?("next")
  end

  def test_poll_events_409_filter_mismatch_is_a_non_retryable_error
    stub_request(:get, "#{BASE}/events.json")
      .with(query: { "position" => "posAAA" })
      .to_return(json_response(409, { "error" => "Positions are bound to the filter set they were minted for.",
                                      "position_digest" => "38b223c13c89dc89", "filters_digest" => "44136fa355b3678a" }))

    error = assert_raises(Basecamp::Error) { @account.event_feed.poll_events(position: "posAAA") }

    assert_equal 409, error.http_status
    assert_not error.retryable
    assert_includes error.message, "Positions are bound"
  end

  def test_poll_events_410_stale_position_is_a_non_retryable_error
    stub_request(:get, "#{BASE}/events.json")
      .with(query: { "position" => "posOLD" })
      .to_return(json_response(410, { "error" => "That position predates this feed's epoch, so the history behind it can't be served.",
                                      "epoch_after_id" => 1071915000,
                                      "resume" => "https://3.basecampapi.com/12345/events.json?since=1071915000" }))

    error = assert_raises(Basecamp::Error) { @account.event_feed.poll_events(position: "posOLD") }

    assert_equal 410, error.http_status
    assert_not error.retryable
  end

  def test_poll_inbox_decodes_the_envelope_and_sends_reasons
    stub_request(:get, "#{BASE}/inbox.json")
      .with(query: { "since" => "0", "reasons" => "mentioned,assigned" })
      .to_return(json_response(200, {
        "items" => [
          { "addressing_id" => 991, "reason" => "mentioned", "addressed_at" => "2026-07-14T06:10:00.159Z",
            "event" => feed_event(1071915468, kind: "comment_created", event_type: "comment.created") }
        ],
        "position" => "inboxPos"
      }))

    page = @account.event_feed.poll_inbox(since: "0", reasons: "mentioned,assigned")

    assert_equal "inboxPos", page["position"]
    assert_equal 1, page["items"].length
    assert_equal 991, page["items"][0]["addressing_id"]
    assert_equal "comment.created", page["items"][0].dig("event", "event_type")
  end

  def test_poll_inbox_403_is_forbidden
    stub_request(:get, "#{BASE}/inbox.json").to_return(status: 403, body: "")

    error = assert_raises(Basecamp::Error) { @account.event_feed.poll_inbox }

    assert_equal 403, error.http_status
    assert_equal "forbidden", error.code
  end

  def test_create_stream_ticket_posts_without_a_body_and_decodes_the_mint
    stub_request(:post, "#{BASE}/events/stream_ticket.json")
      .with { |request| request.body.nil? || request.body.empty? }
      .to_return(json_response(200, { "ticket" => "fixture-ticket-not-a-credential", "expires_in" => 120,
                                      "url" => "wss://cable.example.invalid/12345?ticket=fixture-ticket-not-a-credential" }))

    ticket = @account.event_feed.create_stream_ticket

    assert_equal "fixture-ticket-not-a-credential", ticket["ticket"]
    assert_equal 120, ticket["expires_in"]
    assert_includes ticket["url"], "?ticket="
  end

  def test_create_stream_ticket_401_is_auth_required
    stub_request(:post, "#{BASE}/events/stream_ticket.json")
      .to_return(json_response(401, { "error" => "Unauthorized" }))

    error = assert_raises(Basecamp::Error) { @account.event_feed.create_stream_ticket }

    assert_equal "auth_required", error.code
  end
end
