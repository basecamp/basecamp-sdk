# frozen_string_literal: true

# Tests for the GaugesService (generated from OpenAPI spec)
#
# Notes on the generated shape, all deliberate and pinned below:
# - The needle single-resource paths carry NO .json suffix
#   (/gauge_needles/{id}), while the collection and toggle paths do.
# - Both list operations paginate over a BARE array body, so they return a
#   ListEnumerator whose #meta carries X-Total-Count and truncation.
# - create_gauge_needle runs its body through compact_params, so a nil
#   notify/subscriptions must not reach the wire at all.

require "test_helper"

class GaugesServiceTest < Minitest::Test
  include TestHelper

  GAUGES_URL = "https://3.basecampapi.com/12345/reports/gauges.json"
  NEEDLES_URL = "https://3.basecampapi.com/12345/projects/7/gauge/needles.json"
  NEEDLE_ID = 1_069_479_850
  NEEDLE_URL = "https://3.basecampapi.com/12345/gauge_needles/#{NEEDLE_ID}"
  TOGGLE_URL = "https://3.basecampapi.com/12345/projects/7/gauge.json"

  # Matches the needle single-resource path with OR without a .json suffix, so
  # a path regression surfaces as a specific assertion about which URL was
  # requested rather than as an unstubbed-connection error.
  NEEDLE_URL_EITHER = %r{\Ahttps://3\.basecampapi\.com/12345/gauge_needles/\d+(\.json)?\z}
  # Likewise for the toggle path: gauge.json vs a pluralized gauges.json.
  TOGGLE_URL_EITHER = %r{\Ahttps://3\.basecampapi\.com/12345/projects/7/gauges?\.json\z}
  # Any query string on the gauges report, so the query KEY can be asserted.
  GAUGES_URL_ANY_QUERY = %r{\Ahttps://3\.basecampapi\.com/12345/reports/gauges\.json(\?.*)?\z}

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def gauge_fixture
    @gauge_fixture ||= load_fixture("gauges/get.json")
  end

  def needle_fixture
    @needle_fixture ||= load_fixture("gauges/needle_get.json")
  end

  def json_headers
    { "Content-Type" => "application/json" }
  end

  # ===========================================================================
  # list_gauges
  # ===========================================================================

  def test_list_gauges
    stub_request(:get, GAUGES_URL)
      .to_return(status: 200, body: [ gauge_fixture ].to_json, headers: json_headers)

    gauges = @account.gauges.list_gauges.to_a

    assert_equal 1, gauges.length
    gauge = gauges.first
    assert_equal gauge_fixture["id"], gauge["id"]
    assert_equal "Gauge", gauge["type"]
    assert_equal true, gauge["enabled"]
    assert_equal "green", gauge["last_needle_color"]
    assert_equal 72, gauge["last_needle_position"]
    assert_equal 45, gauge["previous_needle_position"]
    assert_equal gauge_fixture.dig("bucket", "id"), gauge.dig("bucket", "id")
  end

  # bucket_ids is a comma-separated STRING query filter named `bucket_ids`.
  # The key is the whole contract here — Rails reads `bucket_ids`, and a
  # singular `bucket_id` would be silently ignored, returning the unfiltered
  # risk-ordered list while looking like it worked.
  def test_list_gauges_sends_bucket_ids_query_filter
    stub_request(:get, GAUGES_URL_ANY_QUERY)
      .to_return(status: 200, body: [ gauge_fixture ].to_json, headers: json_headers)

    @account.gauges.list_gauges(bucket_ids: "2085958500,2085958501").to_a

    assert_requested :get, GAUGES_URL, query: { "bucket_ids" => "2085958500,2085958501" }, times: 1
  end

  def test_list_gauges_omits_bucket_ids_when_not_given
    stub_request(:get, GAUGES_URL_ANY_QUERY)
      .to_return(status: 200, body: [ gauge_fixture ].to_json, headers: json_headers)

    @account.gauges.list_gauges.to_a

    assert_requested(:get, GAUGES_URL_ANY_QUERY) { |req| req.uri.query.nil? || req.uri.query.empty? }
  end

  # SPEC section 8: a positive page selects EXACTLY that page in one request.
  # The rel="next" the selected page advertises is deliberately not followed,
  # and that unfollowed link is what makes the result truncated.
  def test_list_gauges_page_selects_exactly_that_page
    stub_request(:get, GAUGES_URL).with(query: { "page" => "3" })
      .to_return(
        status: 200,
        body: [ gauge_fixture ].to_json,
        headers: json_headers.merge("X-Total-Count" => "120", "Link" => "<#{GAUGES_URL}?page=4>; rel=\"next\"")
      )

    enum = @account.gauges.list_gauges(page: 3)
    gauges = enum.to_a

    assert_equal 1, gauges.length
    assert_equal 120, enum.meta.total_count
    assert enum.meta.truncated, "the pinned page advertised a next page it did not follow"
    assert_requested :get, GAUGES_URL, query: { "page" => "3" }, times: 1
    assert_not_requested :get, GAUGES_URL, query: { "page" => "4" }
  end

  # No page pinned: the walk follows rel="next" across pages and, having
  # reached the last one, is not truncated.
  def test_list_gauges_without_page_follows_links_across_pages
    second = gauge_fixture.merge("id" => gauge_fixture["id"] + 1)
    stub_request(:get, GAUGES_URL)
      .to_return(
        status: 200,
        body: [ gauge_fixture ].to_json,
        headers: json_headers.merge("X-Total-Count" => "2", "Link" => "<#{GAUGES_URL}?page=2>; rel=\"next\"")
      )
    stub_request(:get, GAUGES_URL).with(query: { "page" => "2" })
      .to_return(status: 200, body: [ second ].to_json, headers: json_headers)

    enum = @account.gauges.list_gauges
    gauges = enum.to_a

    assert_equal [ gauge_fixture["id"], second["id"] ], gauges.map { |g| g["id"] }
    assert_equal 2, enum.meta.total_count
    assert_not enum.meta.truncated, "the walk reached the last page"
    assert_requested :get, GAUGES_URL, query: { "page" => "2" }, times: 1
  end

  # ListGauges declares no NotFoundError: there is no resource to miss. A 403
  # is the discriminating failure — the classification, not just the class.
  def test_list_gauges_forbidden
    stub_request(:get, GAUGES_URL)
      .to_return(status: 403, body: "", headers: json_headers)

    error = assert_raises(Basecamp::ForbiddenError) do
      @account.gauges.list_gauges.to_a
    end

    assert_equal Basecamp::ErrorCode::FORBIDDEN, error.code
    assert_equal 403, error.http_status
    assert_equal "Access denied", error.message
    assert_not error.retryable?
  end

  # ===========================================================================
  # list_gauge_needles
  # ===========================================================================

  def test_list_gauge_needles
    stub_request(:get, NEEDLES_URL)
      .to_return(status: 200, body: [ needle_fixture ].to_json, headers: json_headers)

    needles = @account.gauges.list_gauge_needles(project_id: 7).to_a

    assert_equal 1, needles.length
    needle = needles.first
    assert_equal needle_fixture["id"], needle["id"]
    assert_equal "Gauge::Needle", needle["type"]
    assert_equal "green", needle["color"]
    assert_equal 72, needle["position"]
    assert_equal needle_fixture.dig("parent", "id"), needle.dig("parent", "id")
    assert_equal 2, needle["description_attachments"].length
  end

  def test_list_gauge_needles_page_selects_exactly_that_page
    stub_request(:get, NEEDLES_URL).with(query: { "page" => "2" })
      .to_return(
        status: 200,
        body: [ needle_fixture ].to_json,
        headers: json_headers.merge("X-Total-Count" => "80", "Link" => "<#{NEEDLES_URL}?page=3>; rel=\"next\"")
      )

    enum = @account.gauges.list_gauge_needles(project_id: 7, page: 2)
    needles = enum.to_a

    assert_equal 1, needles.length
    assert_equal 80, enum.meta.total_count
    assert enum.meta.truncated, "the pinned page advertised a next page it did not follow"
    assert_requested :get, NEEDLES_URL, query: { "page" => "2" }, times: 1
    assert_not_requested :get, NEEDLES_URL, query: { "page" => "3" }
  end

  def test_list_gauge_needles_without_page_follows_links_across_pages
    second = needle_fixture.merge("id" => needle_fixture["id"] + 1)
    stub_request(:get, NEEDLES_URL)
      .to_return(
        status: 200,
        body: [ needle_fixture ].to_json,
        headers: json_headers.merge("Link" => "<#{NEEDLES_URL}?page=2>; rel=\"next\"")
      )
    stub_request(:get, NEEDLES_URL).with(query: { "page" => "2" })
      .to_return(status: 200, body: [ second ].to_json, headers: json_headers)

    enum = @account.gauges.list_gauge_needles(project_id: 7)
    needles = enum.to_a

    assert_equal [ needle_fixture["id"], second["id"] ], needles.map { |n| n["id"] }
    assert_not enum.meta.truncated, "the walk reached the last page"
    assert_requested :get, NEEDLES_URL, query: { "page" => "2" }, times: 1
  end

  # Unlike ListGauges, this one IS project-scoped, so it declares NotFoundError.
  def test_list_gauge_needles_not_found
    stub_request(:get, NEEDLES_URL)
      .to_return(status: 404, body: { "error" => "Project not found" }.to_json, headers: json_headers)

    error = assert_raises(Basecamp::NotFoundError) do
      @account.gauges.list_gauge_needles(project_id: 7).to_a
    end

    assert_equal Basecamp::ErrorCode::NOT_FOUND, error.code
    assert_equal 404, error.http_status
    assert_equal "Project not found", error.message
    assert_not error.retryable?
  end

  # ===========================================================================
  # get_gauge_needle
  # ===========================================================================

  def test_get_gauge_needle
    stub_request(:get, NEEDLE_URL_EITHER)
      .to_return(status: 200, body: needle_fixture.to_json, headers: json_headers)

    needle = @account.gauges.get_gauge_needle(needle_id: NEEDLE_ID)

    assert_equal needle_fixture["id"], needle["id"]
    assert_equal "Gauge::Needle", needle["type"]
    assert_equal "green", needle["color"]
    assert_equal 72, needle["position"]
    assert_equal needle_fixture.dig("parent", "id"), needle.dig("parent", "id")
  end

  # The single-resource needle route carries NO .json suffix — deliberate in
  # the Smithy spec (@http uri "/{accountId}/gauge_needles/{needleId}"), and
  # the one detail of this operation a reader would most likely "fix".
  def test_get_gauge_needle_path_carries_no_json_suffix
    stub_request(:get, NEEDLE_URL_EITHER)
      .to_return(status: 200, body: needle_fixture.to_json, headers: json_headers)

    @account.gauges.get_gauge_needle(needle_id: NEEDLE_ID)

    assert_requested :get, NEEDLE_URL, times: 1
    assert_not_requested :get, "#{NEEDLE_URL}.json"
  end

  # description_attachments carries bc3's float-spelled integer width
  # (1024.0, not 1024) alongside a non-previewable attachment whose width and
  # height are null. Both must survive the round trip untouched.
  def test_get_gauge_needle_description_attachments_flow_through
    stub_request(:get, NEEDLE_URL_EITHER)
      .to_return(status: 200, body: needle_fixture.to_json, headers: json_headers)

    attachments = @account.gauges.get_gauge_needle(needle_id: NEEDLE_ID)["description_attachments"]

    assert_equal 2, attachments.length
    assert_equal 1024.0, attachments[0]["width"]
    assert_kind_of Float, attachments[0]["width"]
    assert_nil attachments[1]["width"]
    assert_equal needle_fixture.dig("description_attachments", 0, "id"), attachments[0]["id"]
    assert_equal needle_fixture.dig("description_attachments", 1, "id"), attachments[1]["id"]
  end

  def test_get_gauge_needle_not_found
    stub_request(:get, NEEDLE_URL_EITHER)
      .to_return(status: 404, body: { "error" => "Needle not found" }.to_json, headers: json_headers)

    error = assert_raises(Basecamp::NotFoundError) do
      @account.gauges.get_gauge_needle(needle_id: NEEDLE_ID)
    end

    assert_equal Basecamp::ErrorCode::NOT_FOUND, error.code
    assert_equal 404, error.http_status
    assert_equal "Needle not found", error.message
    assert_not error.retryable?
  end

  # ===========================================================================
  # create_gauge_needle
  # ===========================================================================

  def test_create_gauge_needle
    stub_request(:post, NEEDLES_URL)
      .to_return(status: 201, body: needle_fixture.to_json, headers: json_headers)

    needle = @account.gauges.create_gauge_needle(
      project_id: 7,
      gauge_needle: { "position" => 72, "color" => "green", "description" => "<div>Shipped it</div>" }
    )

    assert_equal needle_fixture["id"], needle["id"]
    assert_equal "Gauge::Needle", needle["type"]
    assert_equal "green", needle["color"]
    assert_equal 72, needle["position"]
    assert_requested(:post, NEEDLES_URL, times: 1) do |req|
      JSON.parse(req.body) == {
        "gauge_needle" => { "position" => 72, "color" => "green", "description" => "<div>Shipped it</div>" }
      }
    end
  end

  # compact_params drops a nil notify/subscriptions entirely. Sending
  # "notify": null would be a different request — bc3 reads notify's presence.
  def test_create_gauge_needle_omits_nil_notify_and_subscriptions
    stub_request(:post, NEEDLES_URL)
      .to_return(status: 201, body: needle_fixture.to_json, headers: json_headers)

    @account.gauges.create_gauge_needle(project_id: 7, gauge_needle: { "position" => 10 })

    assert_requested(:post, NEEDLES_URL, times: 1) do |req|
      body = JSON.parse(req.body)
      !body.key?("notify") && !body.key?("subscriptions") && body.keys == [ "gauge_needle" ]
    end
  end

  def test_create_gauge_needle_sends_notify_and_subscriptions_when_given
    stub_request(:post, NEEDLES_URL)
      .to_return(status: 201, body: needle_fixture.to_json, headers: json_headers)

    @account.gauges.create_gauge_needle(
      project_id: 7,
      gauge_needle: { "position" => 10 },
      notify: "custom",
      subscriptions: [ 1049715915, 1049715916 ]
    )

    assert_requested(:post, NEEDLES_URL, times: 1) do |req|
      JSON.parse(req.body) == {
        "gauge_needle" => { "position" => 10 },
        "notify" => "custom",
        "subscriptions" => [ 1049715915, 1049715916 ]
      }
    end
  end

  # CreateGaugeNeedle declares ValidationError; position is required and
  # constrained to 0-100, which bc3 answers with the field-keyed 422 body.
  def test_create_gauge_needle_raises_validation_error_on_422
    stub_request(:post, NEEDLES_URL)
      .to_return(
        status: 422,
        body: { "errors" => { "position" => [ "is not included in the list" ] } }.to_json,
        headers: json_headers
      )

    error = assert_raises(Basecamp::ValidationError) do
      @account.gauges.create_gauge_needle(project_id: 7, gauge_needle: { "position" => 999 })
    end

    assert_equal Basecamp::ErrorCode::VALIDATION, error.code
    assert_equal 422, error.http_status
    assert_equal "position: is not included in the list", error.message
    assert_equal({ "position" => [ "is not included in the list" ] }, error.field_errors)
  end

  # ===========================================================================
  # update_gauge_needle
  # ===========================================================================

  def test_update_gauge_needle
    updated = needle_fixture.merge("description" => "<div>Revised</div>")
    stub_request(:put, NEEDLE_URL_EITHER)
      .to_return(status: 200, body: updated.to_json, headers: json_headers)

    needle = @account.gauges.update_gauge_needle(
      needle_id: NEEDLE_ID,
      gauge_needle: { "description" => "<div>Revised</div>" }
    )

    assert_equal needle_fixture["id"], needle["id"]
    assert_equal "<div>Revised</div>", needle["description"]
    # Position and color are immutable; the response still carries them.
    assert_equal "green", needle["color"]
    assert_equal 72, needle["position"]
    assert_requested(:put, NEEDLE_URL, times: 1) do |req|
      JSON.parse(req.body) == { "gauge_needle" => { "description" => "<div>Revised</div>" } }
    end
    assert_not_requested :put, "#{NEEDLE_URL}.json"
  end

  def test_update_gauge_needle_not_found
    stub_request(:put, NEEDLE_URL_EITHER)
      .to_return(status: 404, body: { "error" => "Needle not found" }.to_json, headers: json_headers)

    error = assert_raises(Basecamp::NotFoundError) do
      @account.gauges.update_gauge_needle(needle_id: NEEDLE_ID, gauge_needle: { "description" => "x" })
    end

    assert_equal Basecamp::ErrorCode::NOT_FOUND, error.code
    assert_equal 404, error.http_status
    assert_equal "Needle not found", error.message
  end

  # ===========================================================================
  # destroy_gauge_needle
  # ===========================================================================

  def test_destroy_gauge_needle
    stub_request(:delete, NEEDLE_URL_EITHER).to_return(status: 204, body: "")

    assert_nil @account.gauges.destroy_gauge_needle(needle_id: NEEDLE_ID)

    assert_requested :delete, NEEDLE_URL, times: 1
    assert_not_requested :delete, "#{NEEDLE_URL}.json"
  end

  def test_destroy_gauge_needle_not_found
    stub_request(:delete, NEEDLE_URL_EITHER)
      .to_return(status: 404, body: { "error" => "Needle not found" }.to_json, headers: json_headers)

    error = assert_raises(Basecamp::NotFoundError) do
      @account.gauges.destroy_gauge_needle(needle_id: NEEDLE_ID)
    end

    assert_equal Basecamp::ErrorCode::NOT_FOUND, error.code
    assert_equal 404, error.http_status
    assert_equal "Needle not found", error.message
  end

  # ===========================================================================
  # toggle_gauge
  # ===========================================================================

  def test_toggle_gauge_enable
    stub_request(:put, TOGGLE_URL_EITHER).to_return(status: 204, body: "")

    assert_nil @account.gauges.toggle_gauge(project_id: 7, gauge: { "enabled" => true })

    assert_requested(:put, TOGGLE_URL, times: 1) do |req|
      JSON.parse(req.body) == { "gauge" => { "enabled" => true } }
    end
  end

  # The gauge toggle lives at the singular /projects/{id}/gauge.json — the
  # needles collection beneath it is what is plural, not the gauge itself.
  def test_toggle_gauge_disable
    stub_request(:put, TOGGLE_URL_EITHER).to_return(status: 204, body: "")

    assert_nil @account.gauges.toggle_gauge(project_id: 7, gauge: { "enabled" => false })

    assert_requested(:put, TOGGLE_URL, times: 1) do |req|
      JSON.parse(req.body) == { "gauge" => { "enabled" => false } }
    end
    assert_not_requested :put, "https://3.basecampapi.com/12345/projects/7/gauges.json"
  end

  # Only project admins may toggle a gauge; bc3 answers everyone else with
  # `head :forbidden`. ToggleGauge declares no NotFoundError at all, so 403 is
  # the operation's characteristic denial.
  def test_toggle_gauge_forbidden
    stub_request(:put, TOGGLE_URL_EITHER).to_return(status: 403, body: "", headers: json_headers)

    error = assert_raises(Basecamp::ForbiddenError) do
      @account.gauges.toggle_gauge(project_id: 7, gauge: { "enabled" => true })
    end

    assert_equal Basecamp::ErrorCode::FORBIDDEN, error.code
    assert_equal 403, error.http_status
    assert_equal "Access denied", error.message
    assert_not error.retryable?
  end
end
