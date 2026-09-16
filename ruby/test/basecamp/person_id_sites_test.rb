# frozen_string_literal: true

require "test_helper"

# The typed person-id decode on the generated read path (SPEC §10 "Person Ids
# Off the Wire"): at every site Go reads as types.FlexibleInt64, and nowhere
# else. Every test goes through a real generated service over a stubbed
# transport, so the op id has to reach the decode the way callers reach it.
class PersonIdSitesTest < Minitest::Test
  include TestHelper

  RANGE = "9223372036854775808"

  def setup
    @account = create_account_client
  end

  # --- single object -------------------------------------------------------

  def test_an_untagged_string_creator_id_is_the_integer_it_spells
    stub_get("/12345/comments/1", response_body: { "id" => 1, "creator" => { "id" => "7", "name" => "A" } })

    comment = @account.comments.get(comment_id: 1)

    assert_equal 7, comment["creator"]["id"]
  end

  def test_a_non_numeric_string_id_reads_zero_with_no_system_label
    # FlexibleInt64 writes 0 on ErrSyntax and nothing else; the system_label
    # belongs to the personable_type normalizer, which this person does not
    # reach.
    stub_get("/12345/comments/1", response_body: { "creator" => { "id" => "basecamp", "name" => "A" } })

    creator = @account.comments.get(comment_id: 1)["creator"]

    assert_equal 0, creator["id"]
    assert_not creator.key?("system_label")
  end

  def test_an_out_of_range_string_id_fails_the_read
    stub_get("/12345/comments/1", response_body: { "creator" => { "id" => RANGE, "name" => "A" } })

    error = assert_raises(Basecamp::ApiError) { @account.comments.get(comment_id: 1) }

    assert_not error.retryable?
    assert_includes error.message, "GetComment"
    assert_includes error.message, "creator"
    assert_not_includes error.message, RANGE
  end

  def test_a_tagged_out_of_range_string_the_normalizer_left_now_fails_the_read
    stub_get("/12345/comments/1",
      response_body: { "creator" => { "id" => RANGE, "personable_type" => "User" } })

    assert_raises(Basecamp::ApiError) { @account.comments.get(comment_id: 1) }
  end

  def test_a_non_integer_number_null_or_boolean_id_fails_the_read
    [ 1024.0, 1e3, 2**63, nil, true, [ 1 ], {} ].each do |id|
      WebMock.reset!
      stub_get("/12345/comments/1", response_body: { "creator" => { "id" => id } })

      assert_raises(Basecamp::ApiError, "an id of #{id.inspect}") { @account.comments.get(comment_id: 1) }
    end
  end

  def test_a_person_that_is_not_a_decodable_object_is_left_untouched
    # Declared residual: Go zero-fills or refuses these as part of a whole-body
    # decode this tier does for no field.
    [ { "creator" => { "name" => "A" } }, { "creator" => nil }, { "creator" => 5 }, {} ].each do |body|
      WebMock.reset!
      stub_get("/12345/comments/1", response_body: body)

      assert_equal body, @account.comments.get(comment_id: 1)
    end
  end

  def test_a_mutation_response_is_decoded_too
    stub_put("/12345/comments/1", response_body: { "creator" => { "id" => "7" } })

    assert_equal 7, @account.comments.update(comment_id: 1, content: "x")["creator"]["id"]
  end

  def test_the_body_itself_can_be_the_person
    stub_get("/12345/people/9", response_body: { "id" => "9", "name" => "A" })

    assert_equal 9, @account.people.get(person_id: 9)["id"]
  end

  def test_nested_array_sites_are_decoded
    stub_get("/12345/card_tables/cards/1", response_body: {
      "assignees" => [ { "id" => "1" }, { "id" => "+2" } ],
      "steps" => [
        { "assignees" => [ { "id" => "3" } ], "completer" => { "id" => "4" }, "creator" => { "id" => "5" } },
        { "assignees" => [ { "id" => "007" } ] }
      ]
    })

    card = @account.cards.get(card_id: 1)

    assert_equal [ 1, 2 ], card["assignees"].map { _1["id"] }
    assert_equal [ 3 ], card["steps"][0]["assignees"].map { _1["id"] }
    assert_equal [ 4, 5 ], [ card["steps"][0]["completer"]["id"], card["steps"][0]["creator"]["id"] ]
    assert_equal [ 7 ], card["steps"][1]["assignees"].map { _1["id"] }
  end

  # --- pagination ----------------------------------------------------------

  def test_every_followed_page_of_a_bare_array_list_is_decoded
    stub_paged("/12345/recordings/1/comments.json",
      [ { "creator" => { "id" => "1" } } ],
      [ { "creator" => { "id" => "2" } } ])

    assert_equal [ 1, 2 ], @account.comments.list(recording_id: 1).to_a.map { _1["creator"]["id"] }
  end

  def test_an_out_of_range_id_on_a_followed_page_fails_when_that_page_is_read
    stub_paged("/12345/recordings/1/comments.json",
      [ { "creator" => { "id" => "1" } } ],
      [ { "creator" => { "id" => RANGE } } ])

    comments = @account.comments.list(recording_id: 1)

    error = assert_raises(Basecamp::ApiError) { comments.to_a }
    assert_includes error.message, "ListComments"
  end

  def test_a_wrapped_paginated_response_decodes_the_wrapper_and_every_page
    stub_paged("/12345/reports/users/progress/9.json",
      { "person" => { "id" => "9" }, "events" => [ { "creator" => { "id" => "1" } } ] },
      { "person" => { "id" => "9" },
        "events" => [ { "creator" => { "id" => "2" }, "attachments" => [ { "creator" => { "id" => "3" } } ] } ] })

    result = @account.reports.person_progress(person_id: 9)
    events = result["events"].to_a

    assert_equal 9, result["person"]["id"]
    assert_equal [ 1, 2 ], events.map { _1["creator"]["id"] }
    assert_equal 3, events[1]["attachments"][0]["creator"]["id"]
  end

  # --- the positional surfaces: no double effect -----------------------------

  def test_gauge_people_the_normalizer_converted_are_left_as_it_wrote_them
    stub_get("/12345/reports/gauges.json", response_body: [
      { "creator" => { "id" => "7" } },
      { "creator" => { "id" => "basecamp" } }
    ])

    gauges = @account.gauges.list_gauges.to_a

    assert_equal 7, gauges[0]["creator"]["id"]
    assert_equal({ "id" => 0, "system_label" => "basecamp" }, gauges[1]["creator"])
  end

  def test_a_gauge_needle_creator_is_decoded_once
    stub_get("/12345/gauge_needles/1", response_body: { "creator" => { "id" => "-5" } })

    assert_equal(-5, @account.gauges.get_gauge_needle(needle_id: 1)["creator"]["id"])
  end

  # --- NOT sites: plain int64 in the reference, left strict ---------------------

  def test_upcoming_schedule_people_are_not_sites
    stub_request(:get, %r{/12345/reports/schedules/upcoming\.json})
      .to_return(status: 200, headers: { "Content-Type" => "application/json" }, body: {
        "schedule_entries" => [ { "creator" => { "id" => "7" }, "participants" => [ { "id" => "8" } ] } ],
        "assignables" => [ { "assignees" => [ { "id" => "9" } ] } ]
      }.to_json)

    body = @account.reports.upcoming(window_starts_on: "2026-01-01", window_ends_on: "2026-01-02")

    assert_equal "7", body["schedule_entries"][0]["creator"]["id"]
    assert_equal "8", body["schedule_entries"][0]["participants"][0]["id"]
    assert_equal "9", body["assignables"][0]["assignees"][0]["id"]
  end

  def test_my_assignment_assignees_are_not_sites
    stub_get("/12345/my/assignments.json",
      response_body: { "priorities" => [ { "assignees" => [ { "id" => "7" } ] } ],
                       "non_priorities" => [ { "assignees" => [ { "id" => RANGE } ] } ] })

    body = @account.my_assignments.get_my_assignments

    assert_equal "7", body["priorities"][0]["assignees"][0]["id"]
    assert_equal RANGE, body["non_priorities"][0]["assignees"][0]["id"]
  end

  def test_template_library_confirmation_people_are_not_sites
    # A plain int64 id there: a string is not a confirmation person at all.
    stub_request(:post, "#{BASE_URL}/12345/template_library/copies.json")
      .to_return(status: 422, headers: { "Content-Type" => "application/json" },
        body: { "error" => "confirm", "people" => [ { "id" => "7", "name" => "A", "avatar_url" => "u" } ] }.to_json)

    error = assert_raises(Basecamp::ValidationError) do
      @account.templates.create_library_copy(template_recording_id: 1, destination_parent_id: 2)
    end
    assert_not error.is_a?(Basecamp::PeopleConfirmationRequiredError)
  end

  def test_the_table_carries_no_plain_int64_person_operation
    %w[GetUpcomingSchedule GetMyAssignments GetMyCompletedAssignments GetMyDueAssignments
       DisableOutOfOffice].each do |op|
      assert_nil Basecamp::PersonIdSites.table[op], op
    end
  end

  # --- the decode itself ----------------------------------------------------

  def test_the_decode_is_idempotent_and_needs_an_operation
    body = { "creator" => { "id" => "7" } }

    Basecamp::PersonIdSites.decode!(body, nil)
    assert_equal "7", body["creator"]["id"]

    2.times { Basecamp::PersonIdSites.decode!(body, "GetComment") }
    assert_equal({ "creator" => { "id" => 7 } }, body)
  end

  # --- followed pages decode what the reference decodes, and no more --------

  BAD_IDS = [ nil, 1.5, true, "18446744073709551616" ].freeze

  def test_a_followed_page_item_past_the_cap_is_never_decoded
    # followPagination trims a followed page to the cap before anything is
    # decoded (client.go:604-631), so the fourth item's id is never read.
    BAD_IDS.each do |bad|
      WebMock.reset!
      stub_paged("/12345/recordings/1/comments.json",
        [ { "creator" => { "id" => "7" } }, { "creator" => { "id" => 7 } } ],
        [ { "creator" => { "id" => "9" } }, { "creator" => { "id" => bad } } ])

      comments = @account.comments.list(recording_id: 1, max_items: 3)

      assert_equal [ 7, 7, 9 ], comments.to_a.map { _1["creator"]["id"] }, "a capped-off id of #{bad.inspect}"
      assert comments.meta.truncated
    end
  end

  def test_a_bad_id_past_the_cap_on_page_one_still_fails
    # Page 1 goes through Parse<Op>Response whole, before the cap applies.
    stub_paged("/12345/recordings/1/comments.json",
      [ { "creator" => { "id" => "7" } }, { "creator" => { "id" => nil } } ],
      [ { "creator" => { "id" => "9" } } ])

    assert_raises(Basecamp::ApiError) { @account.comments.list(recording_id: 1, max_items: 1) }
  end

  def test_a_followed_person_progress_page_never_reads_its_person
    # timeline.go:444-457 unmarshals only struct{ Events } on a followed page.
    BAD_IDS.each do |bad|
      WebMock.reset!
      stub_paged("/12345/reports/users/progress/5.json",
        { "person" => { "id" => 1 }, "events" => [ { "creator" => { "id" => "7" } } ] },
        { "person" => { "id" => bad }, "events" => [ { "creator" => { "id" => "8" } } ] })

      result = @account.reports.person_progress(person_id: 5)

      assert_equal [ 7, 8 ], result["events"].to_a.map { _1["creator"]["id"] }, "a page-2 person id of #{bad.inspect}"
      assert_equal 1, result["person"]["id"]
    end
  end

  def test_a_followed_person_progress_page_decodes_every_event_before_the_cap
    # ...and decodes each event of that page before trimming to the limit.
    stub_paged("/12345/reports/users/progress/5.json",
      { "person" => { "id" => 1 }, "events" => [ { "creator" => { "id" => "7" } } ] },
      { "person" => { "id" => 1 }, "events" => [ { "creator" => { "id" => "8" } }, { "creator" => { "id" => true } } ] })

    events = @account.reports.person_progress(person_id: 5, max_items: 2)["events"]

    assert_raises(Basecamp::ApiError) { events.to_a }
  end

  POSITIONAL_LISTINGS = {
    "/12345/reports/gauges.json" => ->(account) { account.gauges.list_gauges },
    "/12345/projects/1/gauge/needles.json" => ->(account) { account.gauges.list_gauge_needles(project_id: 1) },
    "/12345/my/readings/bubble_ups.json" => ->(account) { account.my_notifications.get_bubble_ups }
  }.freeze

  def test_a_null_id_on_a_followed_gauge_needle_or_bubble_up_page_reads
    # Those items decode into hand-written types whose Person.ID is a plain
    # int64 (gauges.go:236-241, 307-312; my_notifications.go:285-305), which
    # reads null as zero.
    POSITIONAL_LISTINGS.each do |path, list|
      WebMock.reset!
      stub_paged(path,
        [ { "creator" => { "id" => "7" }, "participants" => [] } ],
        [ { "creator" => { "id" => nil }, "participants" => [ { "id" => nil } ] } ])

      items = list.call(@account).to_a

      assert_equal [ 7, nil ], items.map { _1["creator"]["id"] }, path
    end
  end

  def test_a_null_id_on_page_one_of_a_gauge_needle_or_bubble_up_listing_still_fails
    POSITIONAL_LISTINGS.each do |path, list|
      WebMock.reset!
      stub_paged(path, [ { "creator" => { "id" => nil } } ], [ { "creator" => { "id" => "7" } } ])

      assert_raises(Basecamp::ApiError, path) { list.call(@account) }
    end
  end

  def test_other_bad_ids_on_a_followed_gauge_page_still_fail
    [ 1.5, true, "18446744073709551616" ].each do |bad|
      WebMock.reset!
      stub_paged("/12345/reports/gauges.json", [ { "creator" => { "id" => 1 } } ], [ { "creator" => { "id" => bad } } ])

      assert_raises(Basecamp::ApiError, bad.inspect) { @account.gauges.list_gauges.to_a }
    end
  end

  def test_a_null_id_on_a_followed_page_of_any_other_listing_still_fails
    stub_paged("/12345/recordings/1/comments.json", [ { "creator" => { "id" => 1 } } ], [ { "creator" => { "id" => nil } } ])

    assert_raises(Basecamp::ApiError) { @account.comments.list(recording_id: 1).to_a }
  end

  private

  def stub_paged(path, page1, page2)
    page2_url = "#{BASE_URL}#{path}?page=2"
    stub_request(:get, "#{BASE_URL}#{path}")
      .to_return(status: 200, body: page1.to_json,
        headers: { "Content-Type" => "application/json", "Link" => "<#{page2_url}>; rel=\"next\"" })
    stub_request(:get, page2_url)
      .to_return(status: 200, body: page2.to_json, headers: { "Content-Type" => "application/json" })
  end
end
