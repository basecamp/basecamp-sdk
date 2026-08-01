# frozen_string_literal: true

require "test_helper"

# Pagination metadata (ListEnumerator/ListMeta) and max_items behavior.
#
# Contract (shared with the other SDKs):
# - The first page is fetched eagerly, so meta.total_count (X-Total-Count,
#   0 when absent) is available as soon as the call returns.
# - Pages beyond the first are fetched lazily during enumeration.
# - meta.truncated is final only once enumeration completes: true only when
#   items beyond those yielded were available (dropped by max_items, or a
#   next Link was left unfetched at the max_items or max_pages cap).
#   Landing exactly on the final item is not truncation.
# - Non-positive max_items disables the cap.
class HttpPaginationMetaTest < Minitest::Test
  include TestHelper

  def setup
    @http = Basecamp::Http.new(config: default_config, token_provider: test_token_provider)
  end

  def test_paginate_returns_list_enumerator_with_meta
    stub_get("/items.json", response_body: [ { "id" => 1 } ])

    enum = @http.paginate("/items.json")

    assert_kind_of Enumerator, enum
    assert_kind_of Basecamp::ListEnumerator, enum
    assert_kind_of Basecamp::ListMeta, enum.meta
  end

  def test_total_count_available_before_consumption
    stub_get("/items.json", response_body: [], headers: { "X-Total-Count" => "42" })

    enum = @http.paginate("/items.json")

    assert_equal 42, enum.meta.total_count
  end

  def test_missing_total_count_is_zero
    stub_get("/items.json", response_body: [ { "id" => 1 } ])

    enum = @http.paginate("/items.json")

    assert_equal 0, enum.meta.total_count
  end

  def test_malformed_total_count_is_zero
    stub_get("/items.json", response_body: [], headers: { "X-Total-Count" => "not-a-number" })

    assert_equal 0, @http.paginate("/items.json").meta.total_count
  end

  def test_first_page_fetched_eagerly
    stub_get("/items.json", response_body: [ { "id" => 1 } ])

    @http.paginate("/items.json")

    assert_requested :get, "#{base_url}/items.json", times: 1
  end

  # PIN (passes before and after the metadata work): partial consumption must
  # not fetch pages beyond what iteration demands — laziness survives the
  # Enumerator subclass.
  def test_first_n_fetches_only_one_page
    stub_get(
      "/items.json",
      response_body: [ { "id" => 1 }, { "id" => 2 } ],
      headers: { "Link" => "<#{base_url}/items.json?page=2>; rel=\"next\"" }
    )

    items = @http.paginate("/items.json").first(2)

    assert_equal [ 1, 2 ], items.map { |i| i["id"] }
    assert_requested :get, "#{base_url}/items.json", times: 1
    assert_not_requested :get, "#{base_url}/items.json?page=2"
  end

  # PIN: external iteration (.next / .peek) works on the returned enumerator.
  def test_external_iteration_next_and_peek
    stub_get("/items.json", response_body: [ { "id" => 1 }, { "id" => 2 } ])

    enum = @http.paginate("/items.json")

    assert_equal 1, enum.peek["id"]
    assert_equal 1, enum.next["id"]
    assert_equal 2, enum.next["id"]
  end

  # PIN: .lazy chains work on the returned enumerator.
  def test_lazy_chain
    stub_get(
      "/items.json",
      response_body: [ { "id" => 1 }, { "id" => 2 } ],
      headers: { "Link" => "<#{base_url}/items.json?page=2>; rel=\"next\"" }
    )

    ids = @http.paginate("/items.json").lazy.map { |i| i["id"] }.first(2)

    assert_equal [ 1, 2 ], ids
    assert_not_requested :get, "#{base_url}/items.json?page=2"
  end

  def test_truncated_false_until_consumption_discovers_it
    # Visibility semantics: truncated starts false and is final only once
    # enumeration completes — asserted here across the consumption boundary.
    stub_get(
      "/items.json",
      response_body: [ { "id" => 1 }, { "id" => 2 } ],
      headers: { "Link" => "<#{base_url}/items.json?page=2>; rel=\"next\"" }
    )

    enum = @http.paginate("/items.json", max_items: 1)

    assert_not enum.meta.truncated, "truncated must not be finalized before consumption"
    enum.to_a
    assert enum.meta.truncated
    assert enum.meta.truncated?
  end

  def test_truncated_at_max_pages_cap
    config = Basecamp::Config.new(base_url: base_url, max_pages: 2)
    http = Basecamp::Http.new(config: config, token_provider: test_token_provider)
    stub_get(
      "/items.json",
      response_body: [ { "id" => 1 } ],
      headers: { "Link" => "<#{base_url}/items.json?page=2>; rel=\"next\"" }
    )
    stub_get(
      "/items.json?page=2",
      response_body: [ { "id" => 2 } ],
      headers: { "Link" => "<#{base_url}/items.json?page=3>; rel=\"next\"" }
    )

    enum = http.paginate("/items.json")
    items = enum.to_a

    assert_equal 2, items.length
    assert enum.meta.truncated
    assert_not_requested :get, "#{base_url}/items.json?page=3"
  end

  def test_not_truncated_when_all_pages_consumed
    stub_get(
      "/items.json",
      response_body: [ { "id" => 1 } ],
      headers: { "X-Total-Count" => "2", "Link" => "<#{base_url}/items.json?page=2>; rel=\"next\"" }
    )
    stub_get("/items.json?page=2", response_body: [ { "id" => 2 } ])

    enum = @http.paginate("/items.json")
    items = enum.to_a

    assert_equal 2, items.length
    assert_equal 2, enum.meta.total_count
    assert_not enum.meta.truncated
  end

  def test_max_items_caps_across_pages_and_truncates
    stub_get(
      "/items.json",
      response_body: [ { "id" => 1 }, { "id" => 2 } ],
      headers: { "Link" => "<#{base_url}/items.json?page=2>; rel=\"next\"" }
    )
    stub_get(
      "/items.json?page=2",
      response_body: [ { "id" => 3 }, { "id" => 4 } ],
      headers: { "Link" => "<#{base_url}/items.json?page=3>; rel=\"next\"" }
    )

    enum = @http.paginate("/items.json", max_items: 3)
    items = enum.to_a

    assert_equal [ 1, 2, 3 ], items.map { |i| i["id"] }
    assert enum.meta.truncated
    assert_not_requested :get, "#{base_url}/items.json?page=3"
  end

  def test_max_items_exact_boundary_not_truncated
    stub_get(
      "/items.json",
      response_body: [ { "id" => 1 }, { "id" => 2 } ],
      headers: { "Link" => "<#{base_url}/items.json?page=2>; rel=\"next\"" }
    )
    stub_get("/items.json?page=2", response_body: [ { "id" => 3 }, { "id" => 4 } ])

    enum = @http.paginate("/items.json", max_items: 4)
    items = enum.to_a

    assert_equal 4, items.length
    assert_not enum.meta.truncated
  end

  def test_max_items_drop_within_single_page_truncates
    stub_get("/items.json", response_body: [ { "id" => 1 }, { "id" => 2 } ])

    enum = @http.paginate("/items.json", max_items: 1)
    items = enum.to_a

    assert_equal 1, items.length
    assert enum.meta.truncated
  end

  def test_max_items_cap_met_with_next_link_truncates
    # Cap lands on the last item of a page that still advertises a next page:
    # nothing dropped in-page, but the unfetched next Link is truncation.
    stub_get(
      "/items.json",
      response_body: [ { "id" => 1 }, { "id" => 2 } ],
      headers: { "Link" => "<#{base_url}/items.json?page=2>; rel=\"next\"" }
    )

    enum = @http.paginate("/items.json", max_items: 2)
    items = enum.to_a

    assert_equal 2, items.length
    assert enum.meta.truncated
    assert_not_requested :get, "#{base_url}/items.json?page=2"
  end

  def test_non_positive_max_items_means_no_cap
    [ 0, -5 ].each do |cap|
      WebMock.reset!
      stub_get("/items.json", response_body: [ { "id" => 1 }, { "id" => 2 } ])

      enum = @http.paginate("/items.json", max_items: cap)

      assert_equal 2, enum.to_a.length
      assert_not enum.meta.truncated
    end
  end

  def test_paginate_key_meta_and_max_items
    stub_get(
      "/events.json",
      response_body: { "events" => [ { "id" => 1 }, { "id" => 2 } ] },
      headers: { "X-Total-Count" => "9", "Link" => "<#{base_url}/events.json?page=2>; rel=\"next\"" }
    )

    enum = @http.paginate_key("/events.json", key: "events", max_items: 1)

    assert_equal 9, enum.meta.total_count
    assert_equal 1, enum.to_a.length
    assert enum.meta.truncated
    assert_not_requested :get, "#{base_url}/events.json?page=2"
  end

  def test_paginate_key_non_positive_max_items_means_no_cap
    stub_get("/events.json", response_body: { "events" => [ { "id" => 1 }, { "id" => 2 } ] })

    enum = @http.paginate_key("/events.json", key: "events", max_items: 0)

    assert_equal 2, enum.to_a.length
    assert_not enum.meta.truncated
  end

  def test_paginate_wrapped_shape_survives_with_meta
    stub_get(
      "/progress.json",
      response_body: { "person" => { "id" => 7 }, "events" => [ { "id" => 1 }, { "id" => 2 } ] },
      headers: { "X-Total-Count" => "5" }
    )

    result = @http.paginate_wrapped("/progress.json", key: "events")

    assert_kind_of Hash, result
    assert_equal({ "id" => 7 }, result["person"])
    assert_kind_of Basecamp::ListEnumerator, result["events"]
    assert_equal 5, result["events"].meta.total_count
    assert_equal [ 1, 2 ], result["events"].to_a.map { |i| i["id"] }
    assert_not result["events"].meta.truncated
  end

  def test_paginate_wrapped_max_items
    stub_get(
      "/progress.json",
      response_body: { "person" => { "id" => 7 }, "events" => [ { "id" => 1 }, { "id" => 2 } ] },
      headers: { "Link" => "<#{base_url}/progress.json?page=2>; rel=\"next\"" }
    )

    result = @http.paginate_wrapped("/progress.json", key: "events", max_items: 1)
    events = result["events"]

    assert_equal 1, events.to_a.length
    assert events.meta.truncated
    assert_not_requested :get, "#{base_url}/progress.json?page=2"
  end

  def test_paginate_wrapped_non_positive_max_items_means_no_cap
    stub_get("/progress.json", response_body: { "person" => {}, "events" => [ { "id" => 1 }, { "id" => 2 } ] })

    result = @http.paginate_wrapped("/progress.json", key: "events", max_items: -1)

    assert_equal 2, result["events"].to_a.length
    assert_not result["events"].meta.truncated
  end

  # PIN: block form still yields every item across pages.
  def test_paginate_block_form_still_yields_items
    stub_get(
      "/items.json",
      response_body: [ { "id" => 1 } ],
      headers: { "Link" => "<#{base_url}/items.json?page=2>; rel=\"next\"" }
    )
    stub_get("/items.json?page=2", response_body: [ { "id" => 2 } ])

    ids = []
    @http.paginate("/items.json") { |item| ids << item["id"] }

    assert_equal [ 1, 2 ], ids
  end
end

# Service-layer coverage: the generated list methods expose max_items and the
# hook wrapper (wrap_paginated) preserves the metadata-carrying enumerator.
class ServicePaginationMetaTest < Minitest::Test
  include TestHelper

  def test_service_list_exposes_meta
    stub_get(
      "/12345/projects.json",
      response_body: [ sample_project ],
      headers: { "X-Total-Count" => "17" }
    )

    enum = create_account_client.projects.list

    assert_kind_of Basecamp::ListEnumerator, enum
    assert_equal 17, enum.meta.total_count
  end

  def test_service_list_accepts_max_items
    stub_get("/12345/projects.json", response_body: [ sample_project(id: 1), sample_project(id: 2) ])

    enum = create_account_client.projects.list(max_items: 1)
    items = enum.to_a

    assert_equal 1, items.length
    assert enum.meta.truncated
  end

  def test_service_list_non_positive_max_items_means_no_cap
    stub_get("/12345/projects.json", response_body: [ sample_project(id: 1), sample_project(id: 2) ])

    enum = create_account_client.projects.list(max_items: 0)

    assert_equal 2, enum.to_a.length
    assert_not enum.meta.truncated
  end

  # PIN: .first through the service layer fetches only the pages it needs.
  def test_service_list_first_fetches_one_page
    stub_get(
      "/12345/projects.json",
      response_body: [ sample_project(id: 1), sample_project(id: 2) ],
      headers: { "Link" => "<https://3.basecampapi.com/12345/projects.json?page=2>; rel=\"next\"" }
    )

    first = create_account_client.projects.list.first(2)

    assert_equal 2, first.length
    assert_requested :get, "https://3.basecampapi.com/12345/projects.json", times: 1
    assert_not_requested :get, "https://3.basecampapi.com/12345/projects.json?page=2"
  end

  def test_wrapped_service_preserves_meta_and_shape
    stub_get(
      "/12345/reports/users/progress/9.json",
      response_body: { "person" => { "id" => 9 }, "events" => [ { "id" => 1 } ] },
      headers: { "X-Total-Count" => "3" }
    )

    result = create_account_client.reports.person_progress(person_id: 9)

    assert_kind_of Hash, result
    assert_kind_of Basecamp::ListEnumerator, result["events"]
    assert_equal 3, result["events"].meta.total_count
  end
end
