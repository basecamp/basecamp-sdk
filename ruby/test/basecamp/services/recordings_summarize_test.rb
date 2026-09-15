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

  # A real Marshal-envelope sgid for a person, so a test about the rich-text
  # filter is driven by input the filter actually changes.
  def person_sgid(id)
    gid = "gid://bc3/Person/#{id}"
    payload = "\x04\b{\aI\"\bgid\x06:\x06ET" + marshal_string(gid) +
              "I\"\fpurpose\x06;\x00T" + marshal_string("attachable")
    [ payload.b ].pack("m0").tr("+/", "-_").delete("=") + "--abc123"
  end

  def marshal_string(value)
    bytes = value.b
    length = bytes.bytesize < 123 ? (bytes.bytesize + 5).chr : "\x01" + bytes.bytesize.chr
    "I\"" + length + bytes + "\x06:\x06ET"
  end

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

  def test_a_routing_argument_with_invalid_utf_8_is_refused_not_a_crash
    # A Go string carries arbitrary bytes, so the reference routes
    # "comment.\xFF" on its "comment" subject like any other. Ruby's
    # String#strip validates the encoding and raised
    # Encoding::CompatibilityError straight out of this public method — a native
    # exception where the contract promises a Basecamp::Error.
    broken = "Comm\xFFent".dup.force_encoding(Encoding::UTF_8)

    error = assert_raises(Basecamp::RecordingRoutingError) { summarize(recording_type: broken) }

    assert_equal "unknown_recording_type", error.kind

    # And the event-type arm routes on its subject, as the reference does.
    stub_get("/12345/comments/1", response_body: recording)
    subject = "comment.\xFF".dup.force_encoding(Encoding::UTF_8)

    assert_equal 1, summarize(event_type: subject)["id"]
  end

  def test_every_scalar_the_reference_types_as_a_string_is_typed_here
    # The projection's boundary is drawn where the REFERENCE draws it, not where
    # this port happened to need it. app_url was coerced with to_s, which turned
    # an array into Ruby inspection text and fabricated a URL-shaped value the
    # caller could not tell from a real one; status was passed through raw. Both
    # are plain strings in the reference, so a value of another type is a decode
    # failure there.
    %w[status app_url].each do |field|
      [ [ "x" ], { "a" => 1 }, 5, true ].each do |malformed|
        stub_get("/12345/comments/1", response_body: recording(field => malformed))

        assert_raises(Basecamp::ApiError, "#{field} of #{malformed.inspect}") do
          summarize(event_type: "comment.created")
        end
        WebMock.reset!
      end

      # Null normalizes to "" at every one of them, which is what the
      # reference's decoder does — so the rule cannot be satisfied by refusing
      # everything that is not already a String.
      stub_get("/12345/comments/1", response_body: recording(field => nil))

      assert_equal "", summarize(event_type: "comment.created")[field]
      WebMock.reset!
    end
  end

  def test_content_that_is_not_a_string_fails_the_read
    # Content is the one member this composite INTERPRETS — mentioned_person_ids
    # is derived from it — so to_s turned an array or a hash into its Ruby
    # rendering and then scanned THAT for mentions. Title goes the same way,
    # because first_non_empty reaches both.
    [ [ "x" ], { "a" => 1 }, 5, true ].each do |malformed|
      stub_get("/12345/comments/1", response_body: recording("content" => malformed))

      assert_raises(Basecamp::ApiError, "content of #{malformed.inspect}") { summarize(event_type: "comment.created") }
      WebMock.reset!

      stub_get("/12345/comments/1", response_body: recording("title" => malformed))

      assert_raises(Basecamp::ApiError, "title of #{malformed.inspect}") { summarize(event_type: "comment.created") }
      WebMock.reset!
    end
  end

  def test_a_routing_key_is_trimmed_as_the_reference_trims_it
    # String#strip is NOT strings.TrimSpace, and the difference decides routing.
    # strip removes a leading or trailing NUL where TrimSpace does not, so
    # "Comment\0" SELECTED A REAL TYPE here and was unknown_recording_type
    # there — a malformed key reaching a live read, in the accepting direction.
    nul = 0.chr

    [ "Comment#{nul}", "#{nul}Comment", "Comment#{nul}#{nul}" ].each do |type|
      error = assert_raises(Basecamp::RecordingRoutingError, "a type of #{type.inspect}") do
        summarize(recording_type: type)
      end

      assert_equal "unknown_recording_type", error.kind
    end

    # A LEADING nul pollutes the subject, which is compared, so it is refused.
    error = assert_raises(Basecamp::RecordingRoutingError) { summarize(event_type: "#{nul}comment.created") }

    assert_equal "unknown_recording_type", error.kind
    assert_not_requested(:any, %r{\A#{Regexp.escape(BASE_URL)}})

    # A TRAILING one does not, and this row is here because the review that
    # found the defect claimed it did. The event path splits on the last "." and
    # compares only the subject, so a nul in the ACTION is never looked at —
    # measured, the reference routes this too. The nul matters exactly where a
    # trimmed value is compared against a table, which is the recording-type
    # path above and not this one.
    stub_get("/12345/comments/1", response_body: recording)

    assert_equal "Comment", summarize(event_type: "comment.created#{nul}")["type"]
    WebMock.reset!

    # And the spaces the reference DOES trim are still trimmed — ASCII, and the
    # multi-byte ones a byte-wise strip would have left in place. Without these
    # rows the rule above is satisfied by trimming nothing at all.
    stub_get("/12345/comments/1", response_body: recording)

    [ " Comment ", "\tComment\n", "\u00A0Comment\u00A0", "\u2003Comment\u3000", "\u0085Comment" ].each do |type|
      assert_equal "Comment", summarize(recording_type: type)["type"], "a type of #{type.inspect}"
    end
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

  def test_a_listed_campfire_already_tried_from_the_dock_is_not_tried_twice
    # One candidate, two sources, one line read — the budget is not spent twice
    # on the same Campfire.
    stub_dock([ 500 ])
    stub_line(500, status: 404, body: { "error" => "Record not found" })
    stub_get("/12345/chats.json", response_body: [ { "id" => 500, "bucket" => { "id" => BUCKET } } ])

    error = assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    assert_equal [ 500 ], error.campfire_ids
    assert_requested(:get, "#{BASE_URL}/12345/chats/500/lines/1", times: 1)
  end

  def test_refuses_a_read_that_came_back_from_another_bucket
    # A pointer from one project must never resolve to a recording in another.
    stub_get("/12345/comments/1", response_body: recording("bucket" => { "id" => 999 }))

    error = assert_raises(Basecamp::BucketMismatchError) { summarize(event_type: "comment.created") }

    assert_equal "bucket_mismatch", error.kind
    assert_equal 999, error.actual_bucket_id
  end

  def test_a_bucket_id_of_the_wrong_type_fails_the_read
    # The reference decodes every id into a typed integer, so a payload like
    # this is a decode failure that fails the READ — it never reaches the
    # comparison. Reading it as absent would skip that comparison, which is how
    # a recording from another project gets returned.
    [ [], {}, 12.5, true, "12oops", "2085958499" ].each do |malformed|
      stub_get("/12345/comments/1", response_body: recording("bucket" => { "id" => malformed }))

      error = assert_raises(Basecamp::ApiError, "a bucket id of #{malformed.inspect}") do
        summarize(event_type: "comment.created")
      end

      assert_equal Basecamp::ErrorCode::API, error.code
      assert_not error.retryable?, "re-requesting cannot repair a malformed body"
      WebMock.reset!
    end
  end

  def test_a_bucket_that_is_not_an_object_fails_the_read
    # Absent or null is genuinely no bucket — the reference holds a pointer
    # there and reads its zero value. A number, a string or an array is a decode
    # failure there, not a zero value.
    [ 5, "x", [] ].each do |malformed|
      stub_get("/12345/comments/1", response_body: recording("bucket" => malformed))

      assert_raises(Basecamp::ApiError, "a bucket of #{malformed.inspect}") do
        summarize(event_type: "comment.created")
      end
      WebMock.reset!
    end
  end

  def test_a_negative_bucket_id_is_a_mismatch
    # NON-ZERO, not positive. The reference compares whenever its bucket id is
    # not the zero value, so a negative one is a mismatch there — and this was
    # the one place an earlier sweep of that same defect did not reach, which
    # left the check failing OPEN.
    stub_get("/12345/comments/1", response_body: recording("bucket" => { "id" => -5 }))

    error = assert_raises(Basecamp::BucketMismatchError) { summarize(event_type: "comment.created") }

    assert_equal(-5, error.actual_bucket_id)
  end

  def test_a_campfire_id_of_the_wrong_type_fails_the_read
    # Same rule, same reason: a decode failure in the reference fails the whole
    # project or listing read rather than quietly skipping one candidate.
    stub_get("/12345/projects/#{BUCKET}", response_body: {
      "id" => BUCKET, "dock" => [ { "id" => [], "name" => "chat" } ]
    })

    assert_raises(Basecamp::ApiError) { summarize(event_type: "chat.line.created") }

    WebMock.reset!
    stub_dock([])
    stub_get("/12345/chats.json", response_body: [ { "id" => "500", "bucket" => { "id" => BUCKET } } ])

    assert_raises(Basecamp::ApiError) { summarize(event_type: "chat.line.created") }
  end

  def test_a_listed_campfire_with_a_zero_id_is_still_a_candidate
    # The reference applies NO id filter to a listed Campfire — its only guard
    # is on the bucket id — so a zero id is tried there and was dropped here.
    stub_dock([])
    stub_get("/12345/chats.json", response_body: [ { "id" => 0, "bucket" => { "id" => BUCKET } } ])
    stub_request(:get, "#{BASE_URL}/12345/chats/0/lines/1")
      .to_return(status: 404, body: '{"error":"Record not found"}',
                 headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    assert_equal [ 0 ], error.campfire_ids
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

  def test_a_body_that_is_not_an_object_fails_the_read
    # The guard one level above the field checks, and the one that matters most.
    # A String body does not fail in Ruby — it PASSES: `"scalar"["bucket"]` is a
    # substring search answering nil, so the bucket reads as absent, the
    # cross-bucket comparison never runs, and a recording from another project
    # comes back. That is this composite's third fail-open from the same shape.
    # The others raised natively — TypeError, TypeError — which is an exception
    # class out of a public method that no caller expects.
    #
    # "null" is NOT in this list, and the first version of this test had it
    # wrong. Measured against the reference: json.Unmarshal of `null` into a
    # struct returns no error and leaves it zero, so a null body is a
    # zero-valued summary there rather than a decode failure. Refusing it would
    # have been this port inventing a rule and calling it the contract.
    [ '"scalar"', "[1,2]", "5" ].each do |body|
      stub_get("/12345/comments/1", response_body: body)

      error = assert_raises(Basecamp::ApiError, "a body of #{body}") do
        summarize(event_type: "comment.created")
      end

      assert_equal Basecamp::ErrorCode::API, error.code
      assert_not error.retryable?
      WebMock.reset!
    end
  end

  def test_assignees_that_are_not_an_array_of_objects_fail_the_read
    # The reference decodes a slice of people, so neither of these is a value it
    # could read. This used to be deleted from the projection, on a comment
    # saying it was "the same reason a malformed bucket is" — which stopped
    # being true when the bucket started raising, and the comment outlived it.
    [ 5, "x", {}, [ 5 ], [ "x" ], [ [] ] ].each do |malformed|
      stub_get("/12345/todos/1", response_body: recording("type" => "Todo", "assignees" => malformed))

      assert_raises(Basecamp::ApiError, "assignees of #{malformed.inspect}") do
        summarize(event_type: "todo.created")
      end
      WebMock.reset!
    end
  end

  def test_a_well_formed_assignees_list_carries_the_assignees_through
    # The positive half of the rule above. It had been asserted only inside a
    # test about a to-do's rich text, so a reader checking what guards the
    # assignees path would not have found it — and the revert harness reports a
    # rule killed only by a test that does not name it as unguarded, which is
    # the right verdict.
    people = [ { "id" => 3, "name" => "Annie" }, { "id" => 4, "name" => "Vic" } ]
    stub_get("/12345/todos/1", response_body: recording("type" => "Todo", "assignees" => people))

    assert_equal people, summarize(event_type: "todo.created")["assignees"]
  end

  def test_a_person_id_is_emitted_decoded_rather_than_as_it_arrived
    # The flexible decode was applied to the keep/drop decision and never to the
    # value, so a caller read a different TYPE from the contract's: the
    # reference re-emits the decoded integer, and "007" comes back as 7 while
    # the "basecamp" sentinel comes back as 0.
    { "7" => 7, "007" => 7, "+7" => 7, "-7" => -7,
      "basecamp" => 0, "" => 0, " 7" => 0, "7\n" => 0, "0x10" => 0 }.each do |wire, decoded|
      stub_get("/12345/comments/1", response_body: recording("creator" => { "id" => wire, "name" => "V" }))

      assert_equal decoded, summarize(event_type: "comment.created")["creator"]["id"],
        "a creator id of #{wire.inspect}"
      WebMock.reset!
    end

    # And every assignee, which is the same field one level down.
    stub_get("/12345/todos/1", response_body: recording(
      "type" => "Todo", "assignees" => [ { "id" => "007", "name" => "A" }, { "id" => 3, "name" => "B" } ]
    ))
    assignees = summarize(event_type: "todo.created")["assignees"]

    assert_equal [ 7, 3 ], assignees.map { |a| a["id"] }
  end

  def test_a_null_person_id_fails_the_read_where_an_absent_one_does_not
    # The one field where absent and null differ: the flexible decoder's own
    # UnmarshalJSON runs for a null and fails, while a missing key is the zero
    # value. Reachable through the creator and through every assignee.
    stub_get("/12345/comments/1", response_body: recording("creator" => { "id" => nil, "name" => "V" }))

    assert_raises(Basecamp::ApiError) { summarize(event_type: "comment.created") }
    WebMock.reset!

    stub_get("/12345/todos/1", response_body: recording("type" => "Todo", "assignees" => [ { "id" => nil } ]))

    assert_raises(Basecamp::ApiError) { summarize(event_type: "todo.created") }
    WebMock.reset!

    # An ABSENT id is the zero value for the keep/drop decision — this creator
    # is kept on its name — but the key is NOT synthesised. The reference emits
    # a whole typed struct here and this tier passes the object through, which
    # is the boundary #project states: normalising one field of a struct whose
    # other fields are passed through would be half a decode, and half a decode
    # is what produced the divergence this test's sibling row covers.
    stub_get("/12345/comments/1", response_body: recording("creator" => { "name" => "V" }))

    creator = summarize(event_type: "comment.created")["creator"]

    assert_equal({ "name" => "V" }, creator)
    assert_not creator.key?("id"), "an absent id is not invented"
  end

  def test_a_malformed_nested_member_fails_the_read_rather_than_travelling
    # "Kept so the reader that reports a malformed body sees it" was true only
    # of the bucket: a parent and a creator have no such reader, so keeping them
    # was under-refusal where the reference fails the whole decode.
    [ [ "creator", { "id" => [], "name" => "V" } ],
      [ "creator", { "id" => 1, "name" => 5 } ],
      [ "parent", { "id" => 1, "title" => {} } ],
      [ "parent", { "id" => 7.5 } ],
      [ "creator", "scalar" ],
      [ "parent", [ 1 ] ] ].each do |field, malformed|
      stub_get("/12345/comments/1", response_body: recording(field => malformed))

      assert_raises(Basecamp::ApiError, "#{field} of #{malformed.inspect}") do
        summarize(event_type: "comment.created")
      end
      WebMock.reset!
    end
  end

  def test_a_null_assignee_is_an_object_rather_than_a_nil
    # The reference decodes a null member as the zero Person and emits it as an
    # OBJECT, so passing nil through handed a consumer something that crashes on
    # assignee["id"] where the contract gives 0. This guard survived three
    # rounds of mutation because nothing asserted what a null member becomes.
    stub_get("/12345/todos/1", response_body: recording(
      "type" => "Todo", "assignees" => [ nil, { "id" => 3 } ]
    ))

    assignees = summarize(event_type: "todo.created")["assignees"]

    assert_equal 2, assignees.length
    assert_equal({}, assignees.first)
    assert_nil assignees.first["id"], "an empty object is indexable, which nil was not"
    assert_equal({ "id" => 3 }, assignees.last)
  end

  def test_an_empty_nested_object_is_omitted_as_the_reference_omits_it
    # The reference builds a bucket, parent or creator only when it has an id or
    # a name, so an empty object leaves the key out of its summary. This emitted
    # "{}" and a caller testing summary.key?("bucket") got a different answer
    # from the contract's.
    # TWO NAMED FIELDS, and the label differs by member: the reference tests
    # id-or-name for a bucket and a creator and id-or-TITLE for a parent. A
    # predicate over "any value present" kept {"type" => "Project"} and
    # {"url" => "u"}, which the reference drops — so the shapes that carry
    # something OTHER than the two named fields are the ones that pin the rule.
    stub_get("/12345/comments/1", response_body: recording(
      "bucket" => { "type" => "Project", "url" => "u" },
      "parent" => { "name" => "not a title", "type" => "Message" },
      "creator" => { "id" => 0, "name" => "" }
    ))

    summary = summarize(event_type: "comment.created")

    assert_not_includes summary.keys, "bucket"
    assert_not_includes summary.keys, "parent"
    assert_not_includes summary.keys, "creator"

    # And each member's own label keeps it: a parent with a title, a bucket
    # with a name. Swapping the two labels would drop both of these.
    WebMock.reset!
    stub_get("/12345/comments/1", response_body: recording(
      "bucket" => { "name" => "The Leto Laptop" }, "parent" => { "title" => "We won Leto!" },
      "creator" => { "name" => "Annie" }
    ))

    kept = summarize(event_type: "comment.created")

    assert_includes kept.keys, "bucket"
    assert_includes kept.keys, "parent"
    assert_includes kept.keys, "creator"

    WebMock.reset!
    stub_get("/12345/comments/1", response_body: recording(
      "bucket" => {}, "parent" => {}, "creator" => { "id" => 0, "name" => "" }
    ))

    empty = summarize(event_type: "comment.created")

    assert_not_includes empty.keys, "bucket"
    assert_not_includes empty.keys, "parent"
    assert_not_includes empty.keys, "creator"

    # But a member that is not an object at all is KEPT, so the reader that
    # refuses it still sees it — dropping it here removed a malformed bucket
    # before the cross-bucket check could object.
    WebMock.reset!
    stub_get("/12345/comments/1", response_body: recording("bucket" => 5))

    assert_raises(Basecamp::ApiError) { summarize(event_type: "comment.created") }
  end

  def test_the_recordings_own_id_is_typed_like_every_other_id
    # Ten shapes reached the projection verbatim while the boundary comment
    # claimed this field was where reproducing the decoder begins. The reference
    # holds a plain int64 here, and the check was already written four times in
    # the same file for the sibling id fields.
    [ "", "s", "1", 5.5, true, [], [ 1 ], {}, { "a" => 1 } ].each do |malformed|
      stub_get("/12345/comments/1", response_body: recording("id" => malformed))

      assert_raises(Basecamp::ApiError, "an id of #{malformed.inspect}") do
        summarize(event_type: "comment.created")
      end
      WebMock.reset!
    end

    # Absent is the zero value, as it is there.
    stub_get("/12345/comments/1", response_body: recording("id" => nil))

    assert_equal 0, summarize(event_type: "comment.created")["id"]
  end

  def test_a_title_or_content_candidate_is_typed_rather_than_coerced
    # first_non_empty's type check survived mutation: reverting it to
    # values.find { !v.to_s.empty? }.to_s left the suite green, because no test
    # drove a malformed candidate through the MULTI-candidate arms. A to-do's
    # title falls back to its content, and a card's content to its description.
    stub_get("/12345/todos/1", response_body: recording(
      "type" => "Todo", "title" => nil, "content" => [ "coerced" ]
    ))

    assert_raises(Basecamp::ApiError) { summarize(event_type: "todo.created") }
  end

  def test_absent_or_empty_assignees_are_omitted_rather_than_refused
    # The reference's +omitempty+ leaves an empty slice out of its summary too,
    # so there is nothing to report and nothing malformed about it.
    [ nil, [] ].each do |empty|
      stub_get("/12345/todos/1", response_body: recording("type" => "Todo", "assignees" => empty))

      assert_not_includes summarize(event_type: "todo.created").keys, "assignees"
      WebMock.reset!
    end
  end

  def test_the_dock_is_read_as_a_whole_rather_than_per_surviving_entry
    # Two rules in one, because they are the same rule. The reference decodes
    # the dock into a slice of typed items, so a dock that is not an array, an
    # item that is not an object, and a malformed id or name on ANY item — not
    # only on the chat ones — all fail the read there. A check placed after the
    # `name == "chat"` filter would never see the last of those, which would
    # make it a rule about this port's control flow rather than about the body.
    [ { "dock" => 5 },
      { "dock" => "x" },
      { "dock" => {} },
      { "dock" => [ 5 ] },
      { "dock" => [ [] ] },
      { "dock" => [ { "id" => 1, "name" => "chat" }, { "id" => "x", "name" => "schedule" } ] },
      { "dock" => [ { "id" => 1, "name" => "chat" }, { "id" => 2, "name" => 5 } ] },
      { "dock" => [ { "id" => true, "name" => "chat" } ] } ].each do |project|
      stub_get("/12345/projects/#{BUCKET}", response_body: { "id" => BUCKET }.merge(project))
      stub_get("/12345/chats.json", response_body: [])

      assert_raises(Basecamp::ApiError, "a dock of #{project.inspect}") do
        summarize(event_type: "chat.line.created")
      end
      WebMock.reset!
    end
  end

  def test_a_null_member_is_the_zero_value_the_reference_decodes_it_into
    # The reference's decoder never errors on a null — at any depth it writes
    # the zero value — so a null dock item is a nameless item that is simply not
    # "chat", and a null listing entry is a Campfire with no bucket. Refusing
    # them was this port adding a rule the contract does not have, in the
    # ACCEPTING-direction's mirror: a body the reference reads, refused here.
    stub_get("/12345/projects/#{BUCKET}", response_body: {
      "id" => BUCKET, "dock" => [ nil, { "id" => 77, "name" => "chat" } ]
    })
    stub_get("/12345/chats.json", response_body: [ nil, { "id" => 78, "bucket" => { "id" => BUCKET } } ])
    stub_line(77, status: 404, body: { "error" => "Record not found" })
    stub_line(78, status: 404, body: { "error" => "Record not found" })

    error = assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }

    assert_equal [ 77, 78 ], error.campfire_ids.sort
  end

  def test_a_null_body_is_a_zero_valued_summary_rather_than_a_refusal
    stub_get("/12345/comments/1", response_body: "null")

    summary = summarize(event_type: "comment.created")

    assert_equal "", summary["content"]
    assert_equal [], summary["mentioned_person_ids"]
    assert_not_includes summary.keys, "bucket"
  end

  def test_a_campfire_listing_that_is_not_a_list_fails_the_read
    # The listing ELEMENTS were guarded and the envelope was not, so the
    # pagination loop indexed whatever the body was: a scalar, a boolean or a
    # null raised NoMethodError out of a public method, and an OBJECT paginated
    # to zero entries — which this composite would then have reported as "no
    # visible Campfire", the one conclusion its own rules forbid it to reach
    # from something it could not read.
    [ "{}", '{"campfires":[]}', '"x"', "5", "true" ].each do |body|
      stub_dock([])
      stub_get("/12345/chats.json", response_body: body)

      assert_raises(Basecamp::ApiError, "a listing body of #{body}") do
        summarize(event_type: "chat.line.created")
      end
      WebMock.reset!
    end
  end

  def test_a_project_body_that_is_not_an_object_fails_the_discovery_read
    # The dock read has its own body guard, and nothing reached it: the
    # body-guard test only drives the comments arm, so deleting this one
    # survived the whole suite. A string body is the original fail-open reborn
    # — project["dock"] is a substring search answering nil, so discovery would
    # report "no visible Campfire" for a project it never managed to read.
    [ '"scalar"', "5", "true" ].each do |body|
      stub_get("/12345/projects/#{BUCKET}", response_body: body)
      stub_get("/12345/chats.json", response_body: [])

      assert_raises(Basecamp::ApiError, "a project body of #{body}") do
        summarize(event_type: "chat.line.created")
      end
      @account = create_account_client(account_id: "12345")
      WebMock.reset!
    end
  end

  def test_an_absent_dock_is_no_campfires_rather_than_a_malformed_one
    # A project need not have a dock, and the reference reads its zero value.
    stub_get("/12345/projects/#{BUCKET}", response_body: { "id" => BUCKET })
    stub_get("/12345/chats.json", response_body: [])

    assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }
  end

  def test_a_listed_campfire_is_read_as_a_whole_rather_than_per_surviving_entry
    # The id is read before the bucket filter for the reason the dock's is read
    # before its name filter. The bucket-id row is the one the previous rewrite
    # deleted its only coverage for: a mutation turning that raise into a `next`
    # left all 41 tests green.
    [ 5, "x", [ 1 ] ].each do |entry|
      assert_listing_refused([ entry ], "an entry of #{entry.inspect}")
    end
    # Each of these has a bucket the filter DOES drop, so only an id read
    # before that filter can see the bad id. The previous row used bucket 999,
    # which the filter keeps — so it passed under either ordering and pinned
    # nothing. A mutation moving the id read after the filter survived the whole
    # suite because of it.
    assert_listing_refused([ { "id" => "x", "bucket" => nil } ], "a bad id on a bucketless campfire")
    assert_listing_refused([ { "id" => "x", "bucket" => { "id" => 0 } } ], "a bad id on a zero bucket")
    assert_listing_refused([ { "id" => "x" } ], "a bad id with no bucket at all")
    assert_listing_refused([ { "id" => "x", "bucket" => { "id" => 999 } } ], "a bad id on another bucket")
    assert_listing_refused([ { "id" => 1, "bucket" => 5 } ], "a bucket that is not an object")
    assert_listing_refused([ { "id" => 1, "bucket" => { "id" => [] } } ], "a bucket id of the wrong type")
  end

  def test_a_listed_campfire_with_no_bucket_is_skipped_rather_than_refused
    # The reference skips on a nil bucket ("c.Bucket == nil || c.Bucket.ID == 0")
    # rather than failing the listing.
    stub_dock([])
    stub_get("/12345/chats.json", response_body: [ { "id" => 7, "bucket" => nil } ])

    assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }
  end

  def test_the_response_readers_are_not_public_api
    # They are prepended onto the generated service, so a helper left public
    # mints SDK surface by accident — and shadows any generated method that
    # later takes the name.
    service = @account.recordings

    %i[read_record read_bucket_id malformed_response read_assignees].each do |helper|
      assert_not service.respond_to?(helper), "#{helper} is public API"
    end
    # The two that are meant to be public still are.
    assert_respond_to service, :summarizable_recording_types
    assert_respond_to service, :summarizable_event_types
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

  # The listing refuses this body, with an empty dock so discovery reaches it.
  def assert_listing_refused(listing, message)
    stub_dock([])
    stub_get("/12345/chats.json", response_body: listing)

    assert_raises(Basecamp::ApiError, message) { summarize(event_type: "chat.line.created") }
  ensure
    WebMock.reset!
  end

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

  def test_a_chat_line_type_that_is_not_a_string_fails_the_read
    # The chat route READS the type to decide whether the line's content can
    # carry a mention, so by this composite's own boundary it has to be a value
    # it can trust. The reference holds a plain string there, so an array or an
    # object is a decode failure — and without the check such a line came back
    # SUCCESSFULLY with its mentions silently cleared, which is the worst of the
    # available outcomes: a plausible answer that is missing data.
    [ [ 1 ], { "a" => 1 }, 5, true ].each do |malformed|
      stub_dock([ 500 ])
      stub_line(500, body: {
        "id" => 1, "type" => malformed, "bucket" => { "id" => BUCKET }, "content" => "hi"
      })

      assert_raises(Basecamp::ApiError, "a type of #{malformed.inspect}") do
        summarize(event_type: "chat.line.created")
      end
      @account = create_account_client(account_id: "12345")
      WebMock.reset!
    end
  end

  def test_a_null_campfire_listing_is_no_candidates_rather_than_a_failed_read
    # A bare list decodes JSON null as the nil slice with no error in the
    # reference, so a null listing is "no rows" there. Rejecting it turned an
    # empty read into a failed one — and for discovery that is the difference
    # between "no visible Campfire" and an error the caller retries.
    stub_dock([])
    stub_get("/12345/chats.json", response_body: "null")

    assert_raises(Basecamp::UnresolvedRecordingError) { summarize(event_type: "chat.line.created") }
  end

  def test_a_null_chat_line_is_a_zero_valued_summary_like_every_other_route
    # project() normalizes a null body to a zero-valued summary, because the
    # reference decodes `null` as the zero value with no error — but the
    # chat-line route went on indexing the RAW response for its type, so a null
    # line raised NoMethodError out of a public method while every other route
    # handled it. The null rule was applied where it was found and not at the
    # one site that reads around the projection.
    stub_dock([ 500 ])
    stub_request(:get, "#{BASE_URL}/12345/chats/500/lines/1")
      .to_return(status: 200, body: "null", headers: { "Content-Type" => "application/json" })

    summary = summarize(event_type: "chat.line.created")

    assert_equal 500, summary["campfire_id"]
    assert_equal [], summary["mentioned_person_ids"]
    assert_equal "", summary["content"]
  end

  def test_only_a_rich_text_chat_line_reports_mentions
    # A Text line's content is HTML-escaped on the way out and a Code line's is
    # served verbatim, so a literal "<bc-attachment>" in either mentions nobody;
    # the two rich-text subtypes do.
    #
    # The sgid here is a REAL one. This test used sgid="whatever", which names
    # no person, so it asserted [] against a line that had no mention in it
    # either way — and a mutation removing the type filter entirely survived the
    # whole suite. A test for a filter has to use input the filter changes.
    sgid = person_sgid(42)
    tag = %(<bc-attachment sgid="#{sgid}"></bc-attachment>)

    { "Chat::Lines::Text" => [], "Chat::Lines::Code" => [],
      "Chat::Lines::RichText" => [ 42 ], "Chat::Lines::Integration" => [ 42 ] }.each do |type, expected|
      stub_dock([ 500 ])
      stub_line(500, body: {
        "id" => 1, "type" => type, "bucket" => { "id" => BUCKET }, "content" => tag
      })

      assert_equal expected, summarize(recording_type: type)["mentioned_person_ids"],
        "#{type} should report #{expected.inspect}"
      @account = create_account_client(account_id: "12345")
      WebMock.reset!
    end
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
    # The reason names both bounds rather than asserting which one fired:
    # meta.truncated? is set by the item cap AND by the client's max_pages
    # limit, and the reference conflates them the same way, so claiming one
    # reported a small two-page listing under max_pages: 1 as "exceeds 1000".
    assert_match(/truncated before it was complete/, error.message)
    assert_match(/#{Basecamp::CampfireIndex::MAX_LISTING}-item cap/, error.message)
    assert_match(/max_pages/, error.message)
    # Not cached: the next call re-reads rather than remembering the overflow.
    assert_raises(Basecamp::CampfireDiscoveryIncompleteError) do
      account.recordings.summarize(bucket_id: BUCKET, recording_id: 1, event_type: "chat.line.created")
    end
    assert_requested(:get, "#{BASE_URL}/12345/chats.json", times: 2)
  end
end
