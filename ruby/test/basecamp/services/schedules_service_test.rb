# frozen_string_literal: true

# Tests for the SchedulesService.
#
# Two layers here:
#
# * the generated, spec-conformant surface — get, list_entries, create_entry,
#   get_entry_occurrence, update_settings, and the raw full-replace
#   +replace_entry+;
# * the hand-written merge-safe composites +update_entry+ and +edit_entry+,
#   prepended by the +on_load+ hook in basecamp.rb.
#
# BC3's Schedules::EntriesController#update rebuilds the recordable from the
# submitted params, so PUT /schedule_entries/{id} is a FULL REPLACE: a body that
# omits +description+ erases it, and one that omits +summary+ erases that too
# (the entry then reads back as "Untitled"). Three writable members are the
# exception — +participant_ids+, +url+ and +highlighted+ are seeded from the
# existing recordable when the request does not address them — which is why the
# composites split the writable set in two and why these tests assert on
# captured bodies, key by key, rather than on the response.
#
# Note: no trash_entry() — use recordings.trash() instead.

require "test_helper"

class SchedulesServiceTest < Minitest::Test
  include TestHelper

  ENTRY_PATH = "/12345/schedule_entries/789"
  ENTRY_URL = "#{BASE_URL}/12345/schedule_entries/789"

  # The two people the canonical fixture carries as participants.
  PARTICIPANT_IDS = [ 1049715914, 1049715915 ].freeze

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_schedule(id: 456)
    {
      "id" => id,
      "title" => "Schedule",
      "include_due_assignments" => true,
      "entries_count" => 5
    }
  end

  def sample_entry(id: nil, summary: nil)
    fixture = load_fixture("schedules/entry_get.json")
    fixture.merge "id" => id || fixture["id"], "summary" => summary || fixture["summary"]
  end

  # The canonical fixture with every writable field populated and distinct, so
  # "preserved", "cleared" and "echoed from the wrong key" are all
  # distinguishable in the PUT body.
  #
  # +url+ and +join_url+ carry deliberately different values: +url+ is the
  # entry's own Basecamp API URL (written by recordings/_recording, which
  # renders first) and +join_url+ is the join link. A composite that read +url+
  # would write the API URL into the join link, and only a fixture that
  # separates them can catch it.
  def full_entry(**overrides)
    load_fixture("schedules/entry_get.json").merge(
      "id" => 789,
      "summary" => "Team Meeting",
      "description" => "<div>Agenda in the doc.</div>",
      "starts_at" => "2026-06-05T06:00:00Z",
      "ends_at" => "2026-06-05T08:30:00Z",
      "all_day" => false,
      "url" => "https://3.basecampapi.com/12345/buckets/1/schedule_entries/789.json",
      "join_url" => "https://meet.example.com/team",
      "highlighted" => true
    ).merge(overrides)
  end

  # An all-day entry: the wire spells its bounds as bare dates, and its all_day
  # flag is the +true+ that a truthiness-based read would preserve by accident
  # and a defaulting one would destroy.
  def all_day_entry(**overrides)
    full_entry(
      "all_day" => true,
      "starts_at" => "2026-06-05",
      "ends_at" => "2026-06-05"
    ).merge(overrides)
  end

  # Captures every PUT body so a test can assert the exact request count and the
  # exact bytes, not just "a PUT happened".
  def capture_entry_put(response)
    captured = { bodies: [] }
    stub_request(:put, ENTRY_URL)
      .with { |req| captured[:bodies] << JSON.parse(req.body) }
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })
    captured
  end

  def stub_entry_get_and_put(entry: full_entry)
    stub_get(ENTRY_PATH, response_body: entry)
    capture_entry_put(entry)
  end

  # The five full-state members as the composite resends them off an untouched
  # read of +full_entry+.
  def preserved_full_state
    {
      "summary" => "Team Meeting",
      "starts_at" => "2026-06-05T06:00:00Z",
      "ends_at" => "2026-06-05T08:30:00Z",
      "description" => "<div>Agenda in the doc.</div>",
      "all_day" => false
    }
  end

  def assert_no_carve_outs(body)
    assert_not_includes body.keys, "participant_ids", "an unaddressed participant list must stay off the wire"
    assert_not_includes body.keys, "url", "an unaddressed join link must stay off the wire"
    assert_not_includes body.keys, "highlighted", "an unaddressed highlight must stay off the wire"
    assert_not_includes body.keys, "notify", "notify is a directive; it is never resent"
  end

  # ---------------------------------------------------------------------
  # The generated surface.
  # ---------------------------------------------------------------------

  def test_get
    stub_get("/12345/schedules/456", response_body: sample_schedule)

    result = @account.schedules.get(schedule_id: 456)

    assert_equal 456, result["id"]
    assert_equal "Schedule", result["title"]
  end

  def test_list_entries
    entries = [ sample_entry, sample_entry(id: 790, summary: "Another Event") ]
    stub_get("/12345/schedules/456/entries.json", response_body: entries)

    result = @account.schedules.list_entries(schedule_id: 456).to_a

    assert_equal 2, result.length
    assert_equal "Project Kickoff Meeting", result[0]["summary"]
  end

  def test_get_entry
    stub_get("/12345/schedule_entries/789", response_body: sample_entry(id: 789))

    result = @account.schedules.get_entry(entry_id: 789)

    assert_equal 789, result["id"]
    assert_equal "Project Kickoff Meeting", result["summary"]
  end

  def test_create_entry
    new_entry = sample_entry(id: 999, summary: "New Event")
    stub_post("/12345/schedules/456/entries.json", response_body: new_entry)

    result = @account.schedules.create_entry(
      schedule_id: 456,
      summary: "New Event",
      starts_at: "2024-12-20T14:00:00Z",
      ends_at: "2024-12-20T15:00:00Z"
    )

    assert_equal 999, result["id"]
    assert_equal "New Event", result["summary"]
  end

  def test_create_entry_with_all_options
    new_entry = sample_entry(id: 1000, summary: "Full Event")
    stub_post("/12345/schedules/456/entries.json", response_body: new_entry)

    result = @account.schedules.create_entry(
      schedule_id: 456,
      summary: "Full Event",
      starts_at: "2024-12-25T00:00:00Z",
      ends_at: "2024-12-25T23:59:59Z",
      description: "<p>Holiday party!</p>",
      participant_ids: [ 1, 2, 3 ],
      all_day: true,
      notify: true
    )

    assert_equal "Full Event", result["summary"]
  end

  def test_create_entry_with_subscriptions
    new_entry = sample_entry(id: 1001, summary: "Quiet Event")
    stub_post("/12345/schedules/456/entries.json", response_body: new_entry)

    @account.schedules.create_entry(
      schedule_id: 456,
      summary: "Quiet Event",
      starts_at: "2024-12-20T14:00:00Z",
      ends_at: "2024-12-20T15:00:00Z",
      subscriptions: [ 111, 222 ]
    )

    assert_requested(:post, "#{BASE_URL}/12345/schedules/456/entries.json",
      body: hash_including("subscriptions" => [ 111, 222 ]))
  end

  def test_get_entry_occurrence
    occurrence = sample_entry.merge("occurrence_date" => "2024-12-22")
    stub_get("/12345/schedule_entries/789/occurrences/2024-12-22", response_body: occurrence)

    result = @account.schedules.get_entry_occurrence(entry_id: 789, date: "2024-12-22")

    assert_equal "2024-12-22", result["occurrence_date"]
  end

  # The occurrence path ends in `{date}` (a string), not an `Id`-suffixed param.
  # resource_id must fall back to the entry id, never the date string.
  def test_get_entry_occurrence_operation_metadata
    events = []
    account = create_account_client(account_id: "12345", hooks: CapturingHooks.new(events))

    occurrence = sample_entry.merge("occurrence_date" => "2024-12-22")
    stub_get("/12345/schedule_entries/789/occurrences/2024-12-22", response_body: occurrence)

    account.schedules.get_entry_occurrence(entry_id: 789, date: "2024-12-22")

    event = events.find { |e| e[:event] == :on_operation_start }
    assert event, "Expected on_operation_start to fire"
    info = event[:info]
    assert_equal "schedules", info.service
    assert_equal "get_entry_occurrence", info.operation
    assert_equal 789, info.resource_id
  end

  def test_update_settings
    updated_schedule = sample_schedule.merge("include_due_assignments" => false)
    stub_put("/12345/schedules/456", response_body: updated_schedule)

    result = @account.schedules.update_settings(
      schedule_id: 456,
      include_due_assignments: false
    )

    assert_equal false, result["include_due_assignments"]
  end

  # ---------------------------------------------------------------------
  # replace_entry: the server-native verbatim PUT.
  #
  # Sharp by construction — every writable field the body omits, the server
  # clears, except the three it preserves. +replace_entry+ keeps that raw
  # operation reachable; the composites below blunt it.
  # ---------------------------------------------------------------------

  def test_replace_entry_sends_sparse_verbatim_with_no_get
    captured = capture_entry_put(full_entry("summary" => "Updated Meeting"))

    result = @account.schedules.replace_entry(
      entry_id: 789,
      summary: "Updated Meeting",
      starts_at: "2026-06-05T06:00:00Z",
      ends_at: "2026-06-05T08:30:00Z"
    )

    assert_equal "Updated Meeting", result["summary"]
    # One request, no read-before-write.
    assert_requested :put, ENTRY_URL, times: 1
    assert_not_requested :get, ENTRY_URL
    assert_equal 1, captured[:bodies].length
    # Omitted stays omitted: replace_entry never invents a carve-out, so the
    # server keeps the participants, join link and highlight it holds.
    assert_equal({ "summary" => "Updated Meeting",
                   "starts_at" => "2026-06-05T06:00:00Z",
                   "ends_at" => "2026-06-05T08:30:00Z" },
                 captured[:bodies].first)
  end

  def test_replace_entry_sends_explicitly_empty_carve_outs
    captured = capture_entry_put(full_entry)

    @account.schedules.replace_entry(
      entry_id: 789,
      summary: "Team Meeting",
      starts_at: "2026-06-05T06:00:00Z",
      ends_at: "2026-06-05T08:30:00Z",
      participant_ids: [],
      url: "",
      highlighted: false
    )

    body = captured[:bodies].first
    # compact_params strips only nil, so every stated empty survives: [] clears
    # the participants, "" clears the join link, false removes the highlight.
    assert_equal [], body["participant_ids"]
    assert_equal "", body["url"]
    assert_equal false, body["highlighted"]
  end

  # ---------------------------------------------------------------------
  # update_entry / edit_entry: the merge-safe composites (GET then PUT).
  # ---------------------------------------------------------------------

  def test_update_entry_merges_unset_full_state_fields
    captured = stub_entry_get_and_put

    result = @account.schedules.update_entry(entry_id: 789, summary: "Team Meeting & Kickoff")

    assert_equal 789, result["id"]
    assert_requested :get, ENTRY_URL, times: 1
    assert_equal 1, captured[:bodies].length
    # The four unmentioned full-state fields are carried out of the GET, and
    # nothing else rides along.
    assert_equal preserved_full_state.merge("summary" => "Team Meeting & Kickoff"),
                 captured[:bodies].first
  end

  # The carve-outs are the whole point: a populated read-back carries a join
  # link, a highlight and two participants, and NONE of them may be seeded onto
  # the wire. Resending them is redundant (BC3 preserves them) and echoing the
  # response's own +url+ would store the entry's API URL as the join link.
  def test_update_entry_never_echoes_the_read_back_carve_outs
    captured = stub_entry_get_and_put

    @account.schedules.update_entry(entry_id: 789, summary: "Team Meeting & Kickoff")

    body = captured[:bodies].first
    assert_no_carve_outs body
    assert_not_equal "https://3.basecampapi.com/12345/buckets/1/schedule_entries/789.json", body["url"]
    assert_not_equal "https://meet.example.com/team", body["url"]
  end

  def test_update_entry_addresses_carve_outs
    captured = stub_entry_get_and_put

    @account.schedules.update_entry(
      entry_id: 789, url: "https://meet.example.com/new-room", highlighted: true
    )

    body = captured[:bodies].first
    assert_equal "https://meet.example.com/new-room", body["url"]
    assert_equal true, body["highlighted"]
    # The full state still rides along untouched...
    assert_equal "Team Meeting", body["summary"]
    assert_equal "2026-06-05T06:00:00Z", body["starts_at"]
    # ...and the carve-out the caller did NOT name stays off the wire.
    assert_not_includes body.keys, "participant_ids"
  end

  # An explicitly empty carve-out is an ADDRESS, not an absence. All three have
  # a falsy-in-other-languages spelling, and all three must survive.
  def test_update_entry_clears_carve_outs_with_explicit_empties
    captured = stub_entry_get_and_put

    @account.schedules.update_entry(entry_id: 789, url: "", highlighted: false, participant_ids: [])

    body = captured[:bodies].first
    assert_includes body.keys, "url"
    assert_equal "", body["url"]
    assert_includes body.keys, "highlighted"
    assert_equal false, body["highlighted"]
    assert_includes body.keys, "participant_ids"
    assert_equal [], body["participant_ids"]
  end

  def test_update_entry_addresses_participant_ids
    captured = stub_entry_get_and_put

    @account.schedules.update_entry(entry_id: 789, participant_ids: [ 1049715914 ])

    assert_equal [ 1049715914 ], captured[:bodies].first["participant_ids"]
  end

  # notify is a directive, not state: it has nothing in the read-back to seed
  # it from, so it reaches the wire only when the caller says so.
  def test_update_entry_sends_notify_only_when_addressed
    captured = stub_entry_get_and_put

    @account.schedules.update_entry(entry_id: 789, summary: "Quiet change")
    @account.schedules.update_entry(entry_id: 789, summary: "Loud change", notify: true)

    assert_not_includes captured[:bodies].first.keys, "notify"
    assert_equal true, captured[:bodies].last["notify"]
  end

  # all_day is the field a truthiness-based read destroys: false is the value it
  # most needs to admit, and defaulting a missing one to false would turn an
  # all-day event into a midnight-to-midnight timed one.
  def test_update_entry_preserves_a_false_all_day
    captured = stub_entry_get_and_put

    @account.schedules.update_entry(entry_id: 789, summary: "Still timed")

    assert_includes captured[:bodies].first.keys, "all_day"
    assert_equal false, captured[:bodies].first["all_day"]
  end

  # The mirror: an all-day entry keeps its flag AND its bare-date bounds. The
  # wire spells an all-day entry's bounds as dates, so the composite must
  # round-trip the string verbatim rather than parse and reformat it.
  def test_update_entry_round_trips_an_all_day_entry_verbatim
    captured = stub_entry_get_and_put(entry: all_day_entry)

    @account.schedules.update_entry(entry_id: 789, summary: "Offsite")

    body = captured[:bodies].first
    assert_equal true, body["all_day"]
    assert_equal "2026-06-05", body["starts_at"]
    assert_equal "2026-06-05", body["ends_at"]
  end

  def test_update_entry_clears_description_with_explicit_empty_string
    captured = stub_entry_get_and_put

    @account.schedules.update_entry(entry_id: 789, description: "")

    body = captured[:bodies].first
    assert_includes body.keys, "description"
    assert_equal "", body["description"]
    assert_not_nil body["description"], "a clear must travel as \"\", never as JSON null"
    assert_equal "Team Meeting", body["summary"]
  end

  def test_update_entry_hooks_observe_get_entry_then_replace_entry
    events = []
    account = create_account_client(account_id: "12345", hooks: TrackingHooks.new(events))
    stub_entry_get_and_put

    account.schedules.update_entry(entry_id: 789, summary: "observed")

    # The composite composes the public get_entry and replace_entry, so hooks
    # see the two wire operations rather than one synthetic composite.
    starts = events.select { |e| e[:event] == :on_operation_start }
    assert_equal [ %w[schedules get_entry], %w[schedules replace_entry] ], \
                 starts.map { |e| [ e[:info].service, e[:info].operation ] }
  end

  def test_edit_entry_puts_full_state_back
    captured = stub_entry_get_and_put

    result = @account.schedules.edit_entry(entry_id: 789) do |entry|
      assert_equal "Team Meeting", entry.summary
      assert_equal "<div>Agenda in the doc.</div>", entry.description
      assert_equal false, entry.all_day
      entry.summary = "🚨 #{entry.summary}"
    end

    assert_equal 789, result["id"]
    assert_equal preserved_full_state.merge("summary" => "🚨 Team Meeting"), captured[:bodies].first
  end

  # The carve-out getters are seeded FOR READING, so a block can inspect the
  # current values before deciding. Reading alone must not put them on the wire.
  def test_edit_entry_exposes_the_read_back_carve_outs_without_sending_them
    captured = stub_entry_get_and_put

    @account.schedules.edit_entry(entry_id: 789) do |entry|
      # join_url, not url: the response's url is the entry's own API URL.
      assert_equal "https://meet.example.com/team", entry.url
      assert_equal true, entry.highlighted
      assert_equal PARTICIPANT_IDS, entry.participant_ids
      assert_nil entry.notify, "nothing in the response seeds a directive"
      entry.summary = "Team Sync"
    end

    body = captured[:bodies].first
    assert_equal "Team Sync", body["summary"]
    assert_no_carve_outs body
  end

  # Dirty tracking is by SETTER INVOCATION, not by diffing against the
  # read-back. Assigning a carve-out the value the GET just returned is an
  # address like any other: a diff would drop it, and the server would then hold
  # whatever a concurrent writer left there instead of the value the caller
  # stated.
  def test_edit_entry_sends_a_carve_out_assigned_its_own_read_back_value
    captured = stub_entry_get_and_put

    @account.schedules.edit_entry(entry_id: 789) do |entry|
      read_back_url = entry.url
      read_back_highlight = entry.highlighted
      entry.url = read_back_url
      entry.highlighted = read_back_highlight
    end

    body = captured[:bodies].first
    assert_equal "https://meet.example.com/team", body["url"]
    assert_equal true, body["highlighted"]
    assert_not_includes body.keys, "participant_ids"
  end

  def test_edit_entry_clears_a_carve_out
    captured = stub_entry_get_and_put

    @account.schedules.edit_entry(entry_id: 789) do |entry|
      entry.url = ""
      entry.highlighted = false
      entry.participant_ids = []
    end

    body = captured[:bodies].first
    assert_equal "", body["url"]
    assert_equal false, body["highlighted"]
    assert_equal [], body["participant_ids"]
  end

  def test_edit_entry_clears_description_present_and_empty
    captured = stub_entry_get_and_put

    @account.schedules.edit_entry(entry_id: 789) { |entry| entry.description = "" }

    body = captured[:bodies].first
    assert_includes body.keys, "description"
    assert_equal "", body["description"]
    assert_not_nil body["description"], "a clear must travel as \"\", never as JSON null"
    assert_equal "Team Meeting", body["summary"]
    assert_no_carve_outs body
  end

  # nil is the empty spelling for a String or Array member: compact_params would
  # drop it, turning a stated clear back into an omission, so put_entry_fields
  # normalises it.
  def test_edit_entry_nil_assignment_travels_as_the_empty_value
    captured = stub_entry_get_and_put

    @account.schedules.edit_entry(entry_id: 789) do |entry|
      entry.description = nil
      entry.url = nil
      entry.participant_ids = nil
    end

    body = captured[:bodies].first
    assert_equal "", body["description"]
    assert_equal "", body["url"]
    assert_equal [], body["participant_ids"]
  end

  # A boolean has no empty value, so nil there is caller error, not a clear.
  # Dropping it silently would hand the decision back to the server's rebuild.
  %i[all_day highlighted notify].each do |field|
    define_method("test_edit_entry_refuses_a_nil_#{field}") do
      captured = stub_entry_get_and_put

      error = assert_raises(Basecamp::UsageError) do
        @account.schedules.edit_entry(entry_id: 789) { |entry| entry.public_send("#{field}=", nil) }
      end

      assert_includes error.message, "schedule entry #{field} must be true or false, not nil"
      assert_equal Basecamp::ErrorCode::USAGE, error.code
      assert_not_requested :put, ENTRY_URL
      assert_empty captured[:bodies]
    end
  end

  def test_edit_entry_block_error_aborts_without_put
    captured = stub_entry_get_and_put

    assert_raises(RuntimeError) do
      @account.schedules.edit_entry(entry_id: 789) do |entry|
        entry.summary = "never written"
        raise "abort"
      end
    end

    assert_empty captured[:bodies]
    assert_not_requested :put, ENTRY_URL
  end

  def test_edit_entry_requires_a_block
    assert_raises(ArgumentError) { @account.schedules.edit_entry(entry_id: 789) }
  end

  def test_edit_entry_hooks_observe_get_entry_then_replace_entry
    events = []
    account = create_account_client(account_id: "12345", hooks: TrackingHooks.new(events))
    stub_entry_get_and_put

    account.schedules.edit_entry(entry_id: 789) { |entry| entry.summary = "observed" }

    starts = events.select { |e| e[:event] == :on_operation_start }
    assert_equal [ %w[schedules get_entry], %w[schedules replace_entry] ], \
                 starts.map { |e| [ e[:info].service, e[:info].operation ] }
  end

  # ---------------------------------------------------------------------
  # A malformed GET field must never reach the full-replace PUT (#576).
  #
  # update_entry/edit_entry GET the entry, read each writable field, and PUT the
  # full representation back, so every value read is written — including one the
  # caller never mentioned. Ruby's +||+ treats only nil and false as falsy, so a
  # plain <tt>body["description"] || ""</tt> ERASES the field on +false+ and
  # passes arrays, hashes, numbers and +true+ straight through to be written
  # verbatim. There is no typed decoder between the GET and the read: the
  # generated +get_entry+ returns a raw Hash.
  #
  # The assertion that matters is the ORDERING — assert_not_requested :put. A
  # guard that fires after the PUT has already lost the field.
  # ---------------------------------------------------------------------

  MALFORMED_VALUES = [ false, 0, [], {}, 42, true, [ "x" ], { "a" => 1 } ].freeze
  WRITABLE_STRINGS = %w[summary starts_at ends_at description].freeze

  WRITABLE_STRINGS.each do |field|
    MALFORMED_VALUES.each do |malformed|
      define_method("test_update_entry_refuses_a_malformed_#{field}_#{malformed.inspect}") do
        captured = stub_entry_get_and_put(entry: full_entry(field => malformed))

        # Addresses a carve-out only, so nothing the caller passed masks the
        # malformed full-state field.
        error = assert_raises(Basecamp::ApiError) do
          @account.schedules.update_entry(entry_id: 789, highlighted: true)
        end

        assert_includes error.message, "Schedule entry field #{field.inspect} is not a string"
        # api_error, not usage: the value arrived in a successful API response.
        assert_equal Basecamp::ErrorCode::API, error.code
        assert_requested :get, ENTRY_URL, times: 1
        assert_not_requested :put, ENTRY_URL
        assert_empty captured[:bodies]
      end
    end

    define_method("test_edit_entry_refuses_a_malformed_#{field}_before_writing") do
      captured = stub_entry_get_and_put(entry: full_entry(field => 42))

      error = assert_raises(Basecamp::ApiError) do
        @account.schedules.edit_entry(entry_id: 789) { |entry| entry.summary = "New summary" }
      end

      assert_includes error.message, "Schedule entry field #{field.inspect} is not a string"
      assert_not_requested :put, ENTRY_URL
      assert_empty captured[:bodies]
    end
  end

  # summary, starts_at and ends_at are @required on the response and BC3 can
  # never render them absent, null or blank: Schedule::Entry#summary is
  # <tt>super.presence || "Untitled"</tt>, and starts_at/ends_at are NOT NULL
  # columns every partial emits. Coalescing any of them to "" would blank a real
  # value on a call that never mentioned the field.
  REQUIRED_STRINGS = %w[summary starts_at ends_at].freeze
  MANGLERS = {
    "missing" => ->(entry, field) { entry.except(field) },
    "nil" => ->(entry, field) { entry.merge(field => nil) },
    "blank" => ->(entry, field) { entry.merge(field => "   ") }
  }.freeze

  REQUIRED_STRINGS.each do |field|
    MANGLERS.each do |label, mangle|
      define_method("test_update_entry_refuses_a_#{label}_#{field}_before_writing") do
        captured = stub_entry_get_and_put(entry: mangle.call(full_entry, field))

        error = assert_raises(Basecamp::ApiError) do
          @account.schedules.update_entry(entry_id: 789, highlighted: true)
        end

        assert_includes error.message, %(Schedule entry field "#{field}" is required)
        assert_equal Basecamp::ErrorCode::API, error.code
        assert_not_requested :put, ENTRY_URL
        assert_empty captured[:bodies]
      end

      define_method("test_edit_entry_refuses_a_#{label}_#{field}_before_writing") do
        captured = stub_entry_get_and_put(entry: mangle.call(full_entry, field))

        assert_raises(Basecamp::ApiError) do
          @account.schedules.edit_entry(entry_id: 789) { |entry| entry.summary = "New summary" }
        end

        assert_not_requested :put, ENTRY_URL
        assert_empty captured[:bodies]
      end
    end
  end

  # all_day is @required and NOT NULL DEFAULT false. Absent or null is a
  # malformed response, and the guard cannot be a truthiness test — false is the
  # value it most needs to admit.
  [ "missing", "nil" ].each do |label|
    define_method("test_update_entry_refuses_a_#{label}_all_day_before_writing") do
      entry = label == "missing" ? full_entry.except("all_day") : full_entry("all_day" => nil)
      captured = stub_entry_get_and_put(entry: entry)

      error = assert_raises(Basecamp::ApiError) do
        @account.schedules.update_entry(entry_id: 789, summary: "New summary")
      end

      assert_includes error.message, %(Schedule entry field "all_day" is required)
      assert_equal Basecamp::ErrorCode::API, error.code
      assert_not_requested :put, ENTRY_URL
      assert_empty captured[:bodies]
    end
  end

  # 0/1 and "true" are refused rather than coerced, for the same reason a
  # non-string field is: JSON has a boolean type and the server uses it.
  [ 0, 1, "true", "false", [], {} ].each do |malformed|
    define_method("test_update_entry_refuses_a_non_boolean_all_day_#{malformed.inspect}") do
      captured = stub_entry_get_and_put(entry: full_entry("all_day" => malformed))

      error = assert_raises(Basecamp::ApiError) do
        @account.schedules.update_entry(entry_id: 789, summary: "New summary")
      end

      assert_includes error.message, %(Schedule entry field "all_day" is not a boolean)
      assert_not_requested :put, ENTRY_URL
      assert_empty captured[:bodies]
    end
  end

  # The other half of the rule, for an OPTIONAL field: missing and nil are not
  # malformed, they are empty. "" is what the server already holds.
  { "missing" => ->(entry) { entry.except("description") },
    "nil" => ->(entry) { entry.merge("description" => nil) } }.each do |label, mangle|
    define_method("test_a_#{label}_description_stays_genuinely_empty") do
      captured = stub_entry_get_and_put(entry: mangle.call(full_entry))

      @account.schedules.update_entry(entry_id: 789, summary: "New summary")

      assert_equal "", captured[:bodies].first["description"]
      assert_equal "New summary", captured[:bodies].first["summary"]
    end
  end

  # The carve-out seeds are read through the same guards even though they only
  # ever reach the wire on an explicit address: a block inspecting a corrupt
  # value would decide on garbage.
  def test_update_entry_refuses_a_malformed_join_url
    captured = stub_entry_get_and_put(entry: full_entry("join_url" => 42))

    error = assert_raises(Basecamp::ApiError) do
      @account.schedules.update_entry(entry_id: 789, summary: "New summary")
    end

    assert_includes error.message, %(Schedule entry field "join_url" is not a string)
    assert_not_requested :put, ENTRY_URL
    assert_empty captured[:bodies]
  end

  [
    [ "a non-array", "nope", %(Schedule entry field "participants" is not an array) ],
    [ "a non-object element", [ 42 ], %(Schedule entry field "participants"[0] is not an object) ],
    [ "an element with no id", [ { "name" => "Victor" } ], %(Schedule entry field "participants"[0] has no "id") ],
    [ "a non-integer id", [ { "id" => "1049715914" } ], %(Schedule entry field "participants"[0].id is not an integer) ],
    [ "a boolean id", [ { "id" => true } ], %(Schedule entry field "participants"[0].id is not an integer) ]
  ].each do |label, participants, message|
    define_method("test_update_entry_refuses_#{label.tr(" ", "_")}_in_participants") do
      captured = stub_entry_get_and_put(entry: full_entry("participants" => participants))

      error = assert_raises(Basecamp::ApiError) do
        @account.schedules.update_entry(entry_id: 789, summary: "New summary")
      end

      assert_includes error.message, message
      assert_not_requested :put, ENTRY_URL
      assert_empty captured[:bodies]
    end
  end

  # One level up from the field guards: a successful GET can return a scalar, an
  # Array or nil, and <tt>body["summary"]</tt> would raise a raw TypeError on an
  # Integer or Array — or return a silent nil substring match on a String —
  # instead of the documented statusless api_error. It is also the shape a
  # recurring entry's 302 lands on, since the SDK reads a redirect target it did
  # not ask for rather than following it into an entry partial.
  [ 42, "nope", nil, [ "a" ], true ].each do |body|
    define_method("test_update_entry_refuses_a_non_object_response_body_#{body.inspect}") do
      # stub_get passes Strings through verbatim, so encode first: a bare `nope`
      # is not JSON and would fail transport decode before the guard.
      stub_get(ENTRY_PATH, response_body: body.to_json)
      captured = capture_entry_put(full_entry)

      error = assert_raises(Basecamp::ApiError) do
        @account.schedules.update_entry(entry_id: 789, summary: "New summary")
      end

      assert_includes error.message, "GetScheduleEntry returned"
      assert_includes error.message, "where a schedule entry object was expected"
      assert_equal Basecamp::ErrorCode::API, error.code
      assert_not_requested :put, ENTRY_URL
      assert_empty captured[:bodies]
    end

    define_method("test_edit_entry_refuses_a_non_object_response_body_#{body.inspect}") do
      stub_get(ENTRY_PATH, response_body: body.to_json)
      captured = capture_entry_put(full_entry)

      assert_raises(Basecamp::ApiError) do
        @account.schedules.edit_entry(entry_id: 789) { |entry| entry.summary = "New summary" }
      end

      assert_not_requested :put, ENTRY_URL
      assert_empty captured[:bodies]
    end
  end

  # The malformed value is interpolated into the message, so SPEC section 9's
  # 500-byte cap has to survive a huge body.
  def test_malformed_message_is_capped
    stub_entry_get_and_put(entry: full_entry("description" => [ "x" ] * 50_000))

    error = assert_raises(Basecamp::ApiError) do
      @account.schedules.update_entry(entry_id: 789, summary: "New summary")
    end

    assert_operator error.message.bytesize, :<=, 500
  end

  # The malformed-response errors point at the deliberate-overwrite escape
  # hatch, and it has to name a method that actually exists on the service.
  def test_malformed_error_names_the_escape_hatch
    stub_entry_get_and_put(entry: full_entry("description" => 42))

    error = assert_raises(Basecamp::ApiError) do
      @account.schedules.update_entry(entry_id: 789, summary: "New summary")
    end

    assert_includes error.hint, Basecamp::Services::SchedulesExtensions::ESCAPE_HATCH
    assert_respond_to @account.schedules, :replace_entry
    assert_not error.retryable, "re-requesting cannot repair a malformed body"
  end

  private

  # Captures the OperationInfo passed to on_operation_start so tests can assert
  # the metadata emitted by the real generated service call.
  class CapturingHooks
    include Basecamp::Hooks

    def initialize(events)
      @events = events
    end

    def on_operation_start(info)
      @events << { event: :on_operation_start, info: info }
    end
  end

  class TrackingHooks
    include Basecamp::Hooks

    def initialize(events)
      @events = events
    end

    def on_operation_start(info)
      @events << { event: :on_operation_start, info: info }
    end

    def on_operation_end(info, result)
      @events << { event: :on_operation_end, info: info, result: result }
    end
  end
end
