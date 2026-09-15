# frozen_string_literal: true

require "test_helper"

# Tests for the `recordings.summarize` composite (SPEC section 18, Appendix F).
#
# The conformance fixture (conformance/tests/recording_summary.json) pins the
# routing matrix, the projection's shape and the request sequence. What lives
# here is what a fixture case cannot reach: the routing table's own rules, the
# bucket check, the discovery cache's lifetime across calls, the candidate
# budget, and the stale-Campfire reporting.
class RecordingsSummarizeTest < Minitest::Test
  include TestHelper

  BUCKET = 2085958499

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def recording(overrides = {})
    {
      "id" => 1,
      "status" => "active",
      "type" => "Comment",
      "title" => "Re: We won Leto!",
      "app_url" => "https://3.basecamp.com/12345/buckets/#{BUCKET}/comments/1",
      "content" => "<div>hi</div>",
      "updated_at" => "2022-10-30T01:01:58.169Z",
      "parent" => { "id" => 9, "title" => "We won Leto!" },
      "bucket" => { "id" => BUCKET, "name" => "The Leto Laptop" },
      "creator" => { "id" => 7, "name" => "Annie Bryan" }
    }.merge(overrides)
  end

  def summarize(**)
    @account.recordings.summarize(bucket_id: BUCKET, recording_id: 1, **)
  end

  # --- routing -------------------------------------------------------------

  def test_routes_an_event_type_by_its_subject_whatever_the_action
    # Any action on a routed subject reaches that subject's read — the feed
    # catalogues more actions than the fixture enumerates.
    stub_get("/12345/comments/1", response_body: recording)

    %w[comment.created comment.updated comment.content_changed].each do |event_type|
      assert_equal "Comment", summarize(event_type: event_type)["type"]
    end
  end

  def test_recording_type_wins_over_event_type
    # The recording type is the more exact of the two, so it decides the read.
    stub_get("/12345/messages/1", response_body: recording("type" => "Message"))

    assert_equal "Message", summarize(event_type: "comment.created", recording_type: "Message")["type"]
  end

  def test_every_chat_line_subtype_takes_the_discovery_path
    %w[Chat::Lines::Text Chat::Lines::RichText Chat::Lines::Code Chat::Lines::Upload
       Chat::Lines::Integration].each do |type|
      stub_request(:get, "#{BASE_URL}/12345/projects/#{BUCKET}").to_return(status: 404, body: "{}")
      stub_get("/12345/chats.json", response_body: [])

      error = assert_raises(Basecamp::UnresolvedRecordingError) { summarize(recording_type: type) }

      assert_empty error.campfire_ids
      # A fresh client per iteration: the discovery caches live on the Client
      # and would otherwise answer the next subtype from the first one's reads.
      @account = create_account_client(account_id: "12345")
      WebMock.reset!
    end
  end

  def test_refuses_boost_before_any_request
    error = assert_raises(Basecamp::RecordingRoutingError) { summarize(event_type: "boost.created") }

    assert_equal "no_recording_type", error.kind
    assert_equal Basecamp::ErrorCode::USAGE, error.code
    assert_not_requested(:any, %r{\A#{Regexp.escape(BASE_URL)}})
  end

  def test_refuses_a_type_outside_the_deliberate_set
    # The set is deliberate, not exhaustive: a timesheet entry has an id-only
    # read and is still not routed.
    [ "Timesheet::Entry", "Gauge::Needle", "Client::Reply", "" ].each do |recording_type|
      error = assert_raises(Basecamp::RecordingRoutingError) do
        summarize(recording_type: recording_type, event_type: nil)
      end

      assert_equal "unknown_recording_type", error.kind
    end
  end

  def test_refuses_an_event_type_that_is_not_subject_dot_action
    [ "comment", "comment.", ".created", "unknown.created", "" ].each do |event_type|
      error = assert_raises(Basecamp::RecordingRoutingError) { summarize(event_type: event_type) }

      assert_equal "unknown_recording_type", error.kind
    end
    assert_not_requested(:any, %r{\A#{Regexp.escape(BASE_URL)}})
  end

  def test_refuses_an_incomplete_pointer_before_routing
    assert_raises(Basecamp::UsageError) do
      @account.recordings.summarize(bucket_id: 0, recording_id: 1, event_type: "comment.created")
    end
    assert_raises(Basecamp::UsageError) do
      @account.recordings.summarize(bucket_id: BUCKET, recording_id: 0, event_type: "comment.created")
    end
  end

  def test_refuses_a_malformed_id_rather_than_reading_a_different_recording
    # Coercing would turn "12oops" and 12.9 into 12 and go and fetch THAT
    # recording — a wrong answer wearing the shape of a right one, and one whose
    # bucket can still match, so nothing downstream would catch it.
    # "1_2" is Ruby's integer-literal grammar, not a string of digits, and
    # Integer() would read it as 12 — the same wrong-record hazard in disguise.
    [ "12oops", 12.9, nil, "", [ 12 ], "1_2", " 12 ", "+12", (2**70).to_s,
      "12\xFF".dup.force_encoding(Encoding::UTF_8) ].each do |malformed|
      assert_raises(Basecamp::UsageError) do
        @account.recordings.summarize(bucket_id: BUCKET, recording_id: malformed, event_type: "comment.created")
      end
      assert_raises(Basecamp::UsageError) do
        @account.recordings.summarize(bucket_id: malformed, recording_id: 1, event_type: "comment.created")
      end
    end
    assert_not_requested(:any, %r{\A#{Regexp.escape(BASE_URL)}})
    # A string of digits is a reasonable thing to hold, and is accepted.
    stub_get("/12345/comments/1", response_body: recording)
    summary = @account.recordings.summarize(
      bucket_id: BUCKET.to_s, recording_id: "1", event_type: "comment.created"
    )

    assert_equal 1, summary["id"]
  end

  def test_the_documented_sets_match_the_routing_table
    types = @account.recordings.summarizable_recording_types

    assert_equal types.sort, types
    assert_includes types, "Chat::Lines::*"
    assert_includes types, "Kanban::Card"
    assert_not_includes types, "Timesheet::Entry"
    assert_equal [ "card.*", "chat.line.*", "comment.*", "message.*", "todo.*" ],
                 @account.recordings.summarizable_event_types
  end

  def test_a_routed_kind_with_no_projection_is_refused_by_name
    # The fallback the Go original carries: adding a routing row and forgetting
    # the projection must refuse the pointer, not return nil and fail elsewhere.
    service = @account.recordings

    error = assert_raises(Basecamp::RecordingRoutingError) do
      service.send(:read_summary, :a_kind_nobody_projected, bucket_id: BUCKET, recording_id: 1)
    end

    assert_equal "unknown_recording_type", error.kind
  end

  def test_a_listing_id_matches_a_dock_id_whatever_the_wire_spelled_it
    # Both sources normalize their ids, so a candidate already tried from the
    # dock is not tried again — and its budget not spent twice — when the
    # listing reports the same Campfire as a string.
    stub_dock([ 500 ])
    stub_line(500, status: 404, body: { "error" => "Record not found" })
    stub_get("/12345/chats.json", response_body: [
      { "id" => "500", "bucket" => { "id" => BUCKET } }
    ])

    error = assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    assert_equal [ 500 ], error.campfire_ids
    assert_requested(:get, "#{BASE_URL}/12345/chats/500/lines/1", times: 1)
  end

  # --- projection ----------------------------------------------------------

  def test_refuses_a_read_that_came_back_from_another_bucket
    # A pointer from one project must never resolve to a recording in another.
    stub_get("/12345/comments/1", response_body: recording("bucket" => { "id" => 999 }))

    error = assert_raises(Basecamp::BucketMismatchError) { summarize(event_type: "comment.created") }

    assert_equal "bucket_mismatch", error.kind
    assert_equal 999, error.actual_bucket_id
  end

  def test_a_bucket_id_that_is_present_and_unreadable_fails_closed
    # Absent and malformed are different answers. The reference decodes into a
    # typed integer and fails the READ on a payload like this; reading it as
    # absent would skip the comparison below, which is the one check standing
    # between a pointer and a recording in another bucket.
    [ [], {}, 12.5, true, "12oops" ].each do |malformed|
      stub_get("/12345/comments/1", response_body: recording("bucket" => { "id" => malformed }))

      assert_raises(Basecamp::BucketMismatchError, "a bucket id of #{malformed.inspect} must fail closed") do
        summarize(event_type: "comment.created")
      end
      WebMock.reset!
    end
  end

  def test_a_bucket_that_is_not_an_object_is_read_as_absent
    # A "bucket" member that is not an object has no id at all, which is the
    # reference's zero value and not a malformed one.
    [ 5, "x", [] ].each do |malformed|
      stub_get("/12345/comments/1", response_body: recording("bucket" => malformed))

      assert_equal 1, summarize(event_type: "comment.created")["id"]
      WebMock.reset!
    end
  end

  def test_a_bucket_id_spelled_as_digits_is_read_as_that_id
    # The wire spelling is not always an integer, and the two must compare. A
    # matching one passes; a different one is still a mismatch.
    stub_get("/12345/comments/1", response_body: recording("bucket" => { "id" => BUCKET.to_s }))

    assert_equal 1, summarize(event_type: "comment.created")["id"]

    WebMock.reset!
    stub_get("/12345/comments/1", response_body: recording("bucket" => { "id" => "999" }))

    error = assert_raises(Basecamp::BucketMismatchError) { summarize(event_type: "comment.created") }

    assert_equal 999, error.actual_bucket_id
  end

  def test_a_malformed_campfire_id_is_skipped_rather_than_raising
    stub_get("/12345/projects/#{BUCKET}", response_body: {
      "id" => BUCKET, "dock" => [ { "id" => [], "name" => "chat" }, { "id" => 500, "name" => "chat" } ]
    })
    stub_get("/12345/chats.json", response_body: [
      { "id" => {}, "bucket" => { "id" => BUCKET } },
      { "id" => 501, "bucket" => { "id" => [] } }
    ])
    [ 500, 501 ].each { |id| stub_line(id, status: 404, body: { "error" => "Record not found" }) }

    error = assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    assert_equal [ 500 ], error.campfire_ids
  end

  def test_a_campfire_id_spelled_as_digits_is_a_candidate
    # The branch that reads a digit string was untested, and the comment saying
    # why it exists — so a listing id spelled as a string dedupes against the
    # same id from the dock rather than spending the budget twice — stood in
    # for a test until now.
    stub_get("/12345/projects/#{BUCKET}", response_body: {
      "id" => BUCKET, "dock" => [ { "id" => 500, "name" => "chat" } ]
    })
    stub_get("/12345/chats.json", response_body: [ { "id" => "500", "bucket" => { "id" => BUCKET.to_s } } ])
    stub_line(500, status: 404, body: { "error" => "Record not found" })

    error = assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    assert_equal [ 500 ], error.campfire_ids
    assert_requested(:get, "#{BASE_URL}/12345/chats/500/lines/1", times: 1)
  end

  def test_a_negative_campfire_id_is_still_a_candidate
    # The reference keeps any non-zero id; requiring a positive one would search
    # one Campfire fewer than it does.
    stub_get("/12345/projects/#{BUCKET}", response_body: {
      "id" => BUCKET, "dock" => [ { "id" => -5, "name" => "chat" }, { "id" => 0, "name" => "chat" } ]
    })
    stub_get("/12345/chats.json", response_body: [])
    stub_line(-5, status: 404, body: { "error" => "Record not found" })

    error = assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    assert_equal [ -5 ], error.campfire_ids
  end

  def test_a_malformed_assignees_member_is_read_as_absent
    # Same reason a malformed bucket is: a bad projection must not become an
    # exception class no caller expects.
    stub_get("/12345/todos/1", response_body: recording("type" => "Todo", "assignees" => 5))

    assert_not_includes summarize(event_type: "todo.created").keys, "assignees"
  end

  def test_a_read_with_no_bucket_is_not_a_mismatch
    stub_get("/12345/comments/1", response_body: recording.except("bucket"))

    summary = summarize(event_type: "comment.created")

    assert_not_includes summary.keys, "bucket"
  end

  def test_absent_members_are_omitted_rather_than_carried_as_nil
    stub_get("/12345/comments/1", response_body: recording.except("parent", "creator"))

    summary = summarize(event_type: "comment.created")

    assert_not_includes summary.keys, "parent"
    assert_not_includes summary.keys, "creator"
    assert_not_includes summary.keys, "assignees"
    # Always present, so a consumer reads [] rather than a missing key.
    assert_equal [], summary["mentioned_person_ids"]
  end

  def test_a_type_with_no_rich_text_projects_an_empty_string
    # A vault has no content, and the projection says so with "" rather than nil.
    stub_get("/12345/vaults/1", response_body: recording("type" => "Vault"))

    assert_equal "", summarize(recording_type: "Vault")["content"]
  end

  def test_a_todos_rich_text_is_its_description_not_its_title
    # Mentions live in the description; `content` is the plain title.
    stub_get("/12345/todos/1", response_body: recording(
      "type" => "Todo", "title" => nil, "content" => "Program the MCU",
      "description" => "<div>the rich text</div>", "assignees" => [ { "id" => 3 } ]
    ))

    summary = summarize(event_type: "todo.created")

    assert_equal "Program the MCU", summary["title"]
    assert_equal "<div>the rich text</div>", summary["content"]
    assert_equal [ { "id" => 3 } ], summary["assignees"]
  end

  # --- chat line discovery -------------------------------------------------

  def stub_dock(campfire_ids, bucket: BUCKET)
    stub_get("/12345/projects/#{bucket}", response_body: {
      "id" => bucket,
      "dock" => campfire_ids.map { |id| { "id" => id, "name" => "chat" } } +
        [ { "id" => 88, "name" => "message_board" } ]
    })
  end

  def stub_line(campfire_id, status: 200, body: nil)
    stub_request(:get, "#{BASE_URL}/12345/chats/#{campfire_id}/lines/1")
      .to_return(
        status: status,
        body: (body || { "id" => 1, "type" => "Chat::Lines::Text", "content" => "Hello",
                         "bucket" => { "id" => BUCKET } }).to_json,
        headers: { "Content-Type" => "application/json" }
      )
  end

  # An account client whose discovery caches run on a clock the test moves, so
  # the TTL and the refresh floor can be crossed without sleeping.
  def account_with_clock(clock)
    account = create_account_client(account_id: "12345")
    index = Basecamp::CampfireIndex.new(clock: clock)
    account.define_singleton_method(:campfire_index) { index }
    account
  end

  def test_the_dock_answers_without_the_listing_being_fetched
    # The listing answers 503, not a valid body. Asserting only that it was not
    # requested proves the call did not NEED it; making it fail proves the call
    # does not DEPEND on it, which is the claim worth pinning.
    stub_dock([ 500 ])
    stub_line(500)
    stub_request(:get, "#{BASE_URL}/12345/chats.json")
      .to_return(status: 503, body: '{"error":"down"}', headers: { "Content-Type" => "application/json" })

    summary = summarize(event_type: "chat.line.created")

    assert_equal 500, summary["campfire_id"]
    assert_not_requested(:get, "#{BASE_URL}/12345/chats.json")
  end

  def test_a_plain_line_reports_no_mentions_even_when_its_text_looks_like_markup
    # A Text line's content is HTML-escaped on the way out, so a literal
    # "<bc-attachment>" in it mentions nobody.
    stub_dock([ 500 ])
    stub_line(500, body: {
      "id" => 1, "type" => "Chat::Lines::Text", "bucket" => { "id" => BUCKET },
      "content" => %(<bc-attachment sgid="whatever"></bc-attachment>)
    })

    assert_equal [], summarize(event_type: "chat.line.created")["mentioned_person_ids"]
  end

  def test_a_non_404_from_a_candidate_stops_the_loop_and_is_raised_as_itself
    # A permission failure never masquerades as "not here", and the next
    # candidate is not tried.
    stub_dock([ 500, 501 ])
    stub_line(500, status: 403, body: { "error" => "Forbidden" })

    assert_raises(Basecamp::ForbiddenError) { summarize(event_type: "chat.line.created") }
    assert_not_requested(:get, "#{BASE_URL}/12345/chats/501/lines/1")
  end

  def test_a_listing_failure_that_is_not_an_overflow_passes_through
    # Only a listing OVER ITS CAP is "incomplete". Any other failure of that
    # read is that read's error — a 503 is not a settled verdict about where the
    # line is, and calling it incomplete or unresolved would say something the
    # call never learned.
    stub_dock([])
    stub_request(:get, "#{BASE_URL}/12345/chats.json")
      .to_return(status: 403, body: '{"error":"Access denied"}', headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::ForbiddenError) { summarize(event_type: "chat.line.created") }
  end

  def test_a_dock_failure_that_is_not_a_404_passes_through
    # A bucket that is not a project answers 404 and simply has no dock. Any
    # other failure is the project read's own — the rescue is that narrow, and
    # a 403 here must not read as "this bucket has no Campfires".
    stub_request(:get, "#{BASE_URL}/12345/projects/#{BUCKET}")
      .to_return(status: 403, body: '{"error":"Access denied"}', headers: { "Content-Type" => "application/json" })
    stub_get("/12345/chats.json", response_body: [])

    assert_raises(Basecamp::ForbiddenError) { summarize(event_type: "chat.line.created") }
    # And the listing is never reached, so the failure cannot be mistaken for it.
    assert_not_requested(:get, "#{BASE_URL}/12345/chats.json")
  end

  def test_the_dock_refresh_is_not_blocked_by_the_listing
    # The dock's refresh gets its say BEFORE the listing is fetched, so a
    # listing that is down never stands between a project's line and the one
    # project read that finds it.
    clock = 0.0
    @account = account_with_clock(-> { clock })
    stub_dock([])
    stub_get("/12345/chats.json", response_body: [])

    assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    clock += Basecamp::CampfireIndex::MIN_REFRESH + 1
    WebMock.reset!
    # The refreshed dock now names the Campfire the line is in; the listing is
    # down. The line still resolves.
    stub_dock([ 500 ])
    stub_line(500)
    stub_request(:get, "#{BASE_URL}/12345/chats.json")
      .to_return(status: 503, body: '{"error":"down"}', headers: { "Content-Type" => "application/json" })

    assert_equal 500, summarize(event_type: "chat.line.created")["campfire_id"]
    assert_not_requested(:get, "#{BASE_URL}/12345/chats.json")
  end

  def test_the_caches_are_reused_across_calls_on_one_client
    stub_dock([ 500 ])
    stub_line(500)

    3.times { summarize(event_type: "chat.line.created") }

    # One project read for the bucket, however many lines arrive in it.
    assert_requested(:get, "#{BASE_URL}/12345/projects/#{BUCKET}", times: 1)
  end

  def test_the_caches_are_shared_across_account_clients_of_one_client
    client = create_client
    stub_dock([ 500 ])
    stub_line(500)

    2.times do
      client.for_account("12345").recordings.summarize(
        bucket_id: BUCKET, recording_id: 1, event_type: "chat.line.created"
      )
    end

    assert_requested(:get, "#{BASE_URL}/12345/projects/#{BUCKET}", times: 1)
  end

  def test_a_miss_refreshes_the_cached_sources_once_the_floor_has_passed
    clock = 0.0
    @account = account_with_clock(-> { clock })
    stub_dock([])
    stub_get("/12345/chats.json", response_body: [])

    assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }
    # Within the floor: the cached sources are reused, nothing is re-read, and
    # the conclusion says so.
    error = assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    assert_not error.refreshed?
    assert_requested(:get, "#{BASE_URL}/12345/projects/#{BUCKET}", times: 1)

    clock += Basecamp::CampfireIndex::MIN_REFRESH + 1
    error = assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    assert error.refreshed?
    assert_requested(:get, "#{BASE_URL}/12345/projects/#{BUCKET}", times: 2)
  end

  def test_a_candidate_the_refreshed_sources_dropped_is_reported_as_stale
    # BC3 answers 404 for a Campfire the caller may no longer see, so
    # "unresolved" cannot tell lost visibility from absence. This is what lets a
    # consumer see which it was.
    clock = 0.0
    @account = account_with_clock(-> { clock })
    stub_dock([ 500 ])
    stub_line(500, status: 404, body: { "error" => "Record not found" })
    stub_get("/12345/chats.json", response_body: [])

    assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    clock += Basecamp::CampfireIndex::MIN_REFRESH + 1
    WebMock.reset!
    stub_dock([]) # the Campfire is gone from the dock the caller can now see
    stub_get("/12345/chats.json", response_body: [])
    stub_line(500, status: 404, body: { "error" => "Record not found" })

    error = assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    assert error.refreshed?
    assert_equal [ 500 ], error.stale_campfire_ids
  end

  # The three answers a spent candidate budget can produce. They are three
  # different verdicts, not one: "incomplete" tells a consumer to look again,
  # "unresolved" is settled, and telling them apart is the whole point of the
  # bound.
  def test_a_budget_spent_before_the_listing_is_consulted_is_incomplete
    # Candidates may exist in the listing, unsearched, and nothing unsearched is
    # ever reported absent.
    over_budget = (1..(Basecamp::Services::RecordingsExtensions::MAX_CAMPFIRE_CANDIDATES + 5)).to_a
    stub_dock(over_budget)
    over_budget.each { |id| stub_line(id, status: 404, body: { "error" => "Record not found" }) }
    # 503 rather than an empty body: the verdict must not depend on this read at
    # all, and a failing one would surface as its own error if it were made.
    stub_request(:get, "#{BASE_URL}/12345/chats.json")
      .to_return(status: 503, body: '{"error":"down"}', headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::CampfireDiscoveryIncompleteError) do
      summarize(event_type: "chat.line.created")
    end

    assert_equal "campfire_discovery_incomplete", error.kind
    assert_match(/spent before the account listing was consulted/, error.message)
    assert_not_requested(:get, "#{BASE_URL}/12345/chats.json")
  end

  def test_a_spent_budget_does_not_pay_for_a_re_read_that_cannot_help
    # A source already consulted cannot hand this call a candidate it may try,
    # so its refresh is skipped. Proved by making both sources answer 503 on the
    # second call: a re-read would surface that transient error in place of the
    # settled verdict, which is exactly the failure this rule prevents.
    clock = 0.0
    @account = account_with_clock(-> { clock })
    exactly = (1..Basecamp::Services::RecordingsExtensions::MAX_CAMPFIRE_CANDIDATES).to_a
    stub_dock([])
    stub_get("/12345/chats.json",
      response_body: exactly.map { |id| { "id" => id, "bucket" => { "id" => BUCKET } } })
    exactly.each { |id| stub_line(id, status: 404, body: { "error" => "Record not found" }) }

    assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    clock += Basecamp::CampfireIndex::MIN_REFRESH + 1
    WebMock.reset!
    stub_request(:get, "#{BASE_URL}/12345/projects/#{BUCKET}")
      .to_return(status: 503, body: '{"error":"down"}', headers: { "Content-Type" => "application/json" })
    stub_request(:get, "#{BASE_URL}/12345/chats.json")
      .to_return(status: 503, body: '{"error":"down"}', headers: { "Content-Type" => "application/json" })
    exactly.each { |id| stub_line(id, status: 404, body: { "error" => "Record not found" }) }

    error = assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    assert_not error.refreshed?, "a re-read was spent with no budget left to try its candidates"
    assert_not_requested(:get, "#{BASE_URL}/12345/projects/#{BUCKET}")
    assert_not_requested(:get, "#{BASE_URL}/12345/chats.json")
  end

  def test_a_budget_spent_with_both_sources_consulted_is_unresolved
    # Everything was searched. "Look again" would be the wrong answer.
    exactly = (1..Basecamp::Services::RecordingsExtensions::MAX_CAMPFIRE_CANDIDATES).to_a
    stub_dock([])
    stub_get("/12345/chats.json",
      response_body: exactly.map { |id| { "id" => id, "bucket" => { "id" => BUCKET } } })
    exactly.each { |id| stub_line(id, status: 404, body: { "error" => "Record not found" }) }

    # First call consults the listing in pass 2 and caches it.
    assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }
    WebMock.reset!
    exactly.each { |id| stub_line(id, status: 404, body: { "error" => "Record not found" }) }

    # Both sources answer 503 now. The second call must reach its verdict from
    # what it already holds, rather than merely happening not to re-read them.
    stub_request(:get, "#{BASE_URL}/12345/projects/#{BUCKET}")
      .to_return(status: 503, body: '{"error":"down"}', headers: { "Content-Type" => "application/json" })
    stub_request(:get, "#{BASE_URL}/12345/chats.json")
      .to_return(status: 503, body: '{"error":"down"}', headers: { "Content-Type" => "application/json" })

    # Second call consults both sources from cache, spends the budget on them,
    # and must conclude rather than report the search unfinished.
    error = assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    assert_equal "recording_unresolved", error.kind
    assert_equal Basecamp::Services::RecordingsExtensions::MAX_CAMPFIRE_CANDIDATES, error.campfire_ids.length
    assert_not_requested(:get, "#{BASE_URL}/12345/projects/#{BUCKET}")
    assert_not_requested(:get, "#{BASE_URL}/12345/chats.json")
  end

  def test_more_candidates_than_the_budget_leaves_one_untried_is_incomplete
    # The other incomplete: a candidate was OBSERVED and could not be tried.
    # Reached when both sources are consulted and the listing still holds more.
    stub_dock([])
    over_budget = (1..(Basecamp::Services::RecordingsExtensions::MAX_CAMPFIRE_CANDIDATES + 5)).to_a
    stub_get("/12345/chats.json",
      response_body: over_budget.map { |id| { "id" => id, "bucket" => { "id" => BUCKET } } })
    over_budget.each { |id| stub_line(id, status: 404, body: { "error" => "Record not found" }) }

    error = assert_raises(Basecamp::CampfireDiscoveryIncompleteError) do
      summarize(event_type: "chat.line.created")
    end

    assert_match(/more than #{Basecamp::Services::RecordingsExtensions::MAX_CAMPFIRE_CANDIDATES}/, error.message)
  end

  def test_a_truncated_listing_is_incomplete_not_unresolved
    # The listing only ever serves what the dock did not cover, so an account
    # too large to walk is reported as unfinished rather than cached as a
    # verdict it never reached. max_pages stands in for the item cap here: both
    # leave a next Link unfetched, which is what marks the listing truncated.
    account = create_client(config: Basecamp::Config.new(base_url: BASE_URL, max_pages: 1)) \
      .for_account("12345")
    stub_dock([])
    stub_request(:get, "#{BASE_URL}/12345/chats.json")
      .to_return(
        status: 200,
        body: [ { "id" => 500, "bucket" => { "id" => BUCKET } } ].to_json,
        headers: {
          "Content-Type" => "application/json",
          "Link" => %(<#{BASE_URL}/12345/chats.json?page=2>; rel="next")
        }
      )

    error = assert_raises(Basecamp::CampfireDiscoveryIncompleteError) do
      account.recordings.summarize(bucket_id: BUCKET, recording_id: 1, event_type: "chat.line.created")
    end

    assert_equal "campfire_discovery_incomplete", error.kind
    assert_match(/exceeds #{Basecamp::CampfireIndex::MAX_LISTING}/, error.message)
    # Not cached: the next call re-reads rather than remembering the overflow.
    assert_raises(Basecamp::CampfireDiscoveryIncompleteError) do
      account.recordings.summarize(bucket_id: BUCKET, recording_id: 1, event_type: "chat.line.created")
    end
    assert_requested(:get, "#{BASE_URL}/12345/chats.json", times: 2)
  end
end
