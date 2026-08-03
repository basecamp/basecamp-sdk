# frozen_string_literal: true

# Tests for the RecordingsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - list_events() moved to EventsService
# - subscribe/unsubscribe/get_subscription() moved to SubscriptionsService
# - set_client_visibility() moved to ClientVisibilityService
# - No client-side validation (API validates)
# - bucket param is string, not array
# - Single-resource paths without .json (get)

require "test_helper"

class RecordingsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  # Sourced from the shared recordings/get.json fixture (the validated source of
  # truth). It is a Message recording, so the rich-text companion array it
  # carries is content_attachments (one inline file); description_attachments is
  # absent for this type.
  def sample_recording(id: nil, title: nil)
    fixture = load_fixture("recordings/get.json")
    fixture.merge "id" => id || fixture["id"], "title" => title || fixture["title"]
  end

  def test_list
    recordings = [ sample_recording, sample_recording(id: 457, title: "Another Recording") ]
    stub_request(:get, "https://3.basecampapi.com/12345/projects/recordings.json")
      .with(query: { type: "Todo" })
      .to_return(status: 200, body: recordings.to_json)

    result = @account.recordings.list(type: "Todo").to_a

    assert_equal 2, result.length
    assert_equal "We won Leto!", result[0]["title"]
  end

  def test_list_with_filters
    recordings = [ sample_recording ]
    stub_request(:get, "https://3.basecampapi.com/12345/projects/recordings.json")
      .with(query: { type: "Message", bucket: "100", status: "archived" })
      .to_return(status: 200, body: recordings.to_json)

    # Generated service uses bucket as string, not array
    result = @account.recordings.list(type: "Message", bucket: "100", status: "archived").to_a

    assert_equal 1, result.length
  end

  def test_archive
    stub_put("/12345/recordings/456/status/archived.json", response_body: {})

    result = @account.recordings.archive(recording_id: 456)

    assert_nil result
  end

  def test_unarchive
    stub_put("/12345/recordings/456/status/active.json", response_body: {})

    result = @account.recordings.unarchive(recording_id: 456)

    assert_nil result
  end

  def test_trash
    stub_put("/12345/recordings/456/status/trashed.json", response_body: {})

    result = @account.recordings.trash(recording_id: 456)

    assert_nil result
  end

  # Note: list_events() is on EventsService (spec-conformant)
  # Note: subscribe/unsubscribe/get_subscription() are on SubscriptionsService (spec-conformant)
  # Note: set_client_visibility() is on ClientVisibilityService (spec-conformant)
end
