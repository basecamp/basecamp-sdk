# frozen_string_literal: true

# Tests for the TodolistsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Uses :id instead of :todolist_id for single-resource operations
# - No .json extension for single-resource paths

require "test_helper"

class TodolistsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  # The canonical fixture with a non-empty description, so "preserved" and
  # "cleared" are distinguishable in the PUT body.
  def full_todolist
    load_fixture("todolists/get.json").merge(
      "id" => 2,
      "name" => "Hardware",
      "description" => "<p>Ship the hardware</p>"
    )
  end

  # A todolist GROUP as BC3 renders it on the same route, taken from the shared
  # fixture rather than hand-rolled: todolists/groups/{index,show}.json.jbuilder
  # render todolists/_todolist.json.jbuilder, so a group IS a Todolist and
  # since #544 it is the same modelled shape — description included. What
  # differs is structural, never the type string: the parent is a Todolist, so
  # group_position_url stands in for groups_url. Renumbered onto id 2 so it
  # answers the same stubbed route as full_todolist.
  def group_shaped_todolist
    load_fixture("todolist_groups/get.json").merge("id" => 2, "name" => "Peripherals")
  end

  # Captures every PUT body so a test can assert the exact request count and
  # the exact bytes, not just "a PUT happened".
  def capture_put(response)
    captured = { bodies: [] }
    stub_request(:put, "#{BASE_URL}/12345/todolists/2")
      .with { |req| captured[:bodies] << JSON.parse(req.body) }
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })
    captured
  end

  def stub_todolist_get_and_put(todolist: full_todolist)
    stub_get("/12345/todolists/2", response_body: todolist)
    capture_put(todolist)
  end

  def test_list
    response = [ { "id" => 1, "name" => "Sprint Tasks", "completed_ratio" => "3/10", "description_attachments" => [] } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todosets/\d+/todolists\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.list(todoset_id: 2).to_a
    assert_kind_of Array, result
    assert_equal "Sprint Tasks", result.first["name"]
  end

  def test_list_with_status
    response = [ { "id" => 1, "name" => "Archived List", "status" => "archived", "description_attachments" => [] } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todosets/\d+/todolists\.json\?status=archived})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.list(todoset_id: 2, status: "archived").to_a
    assert_equal "archived", result.first["status"]
  end

  # Uses the canonical fixture rather than a hand-rolled hash: Todolist has a
  # dozen required members (bubble_up_url among them) and a minimal stub is a
  # payload BC3 cannot produce. spec/fixtures is validated by
  # `make check-fixture-coverage`, so this stub cannot drift from the contract.
  def test_get
    response = load_fixture("todolists/get.json")

    # Generated service uses /todolists/{id} without .json
    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todolists/\d+$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.get(id: 2)
    assert_equal response["name"], result["name"]
    # bubble_up_url is @required on Todolist: todolists/_todolist.json.jbuilder
    # renders the shared recording partial with bubbleupable: true
    # unconditionally, so every projection of this shape carries it.
    assert result.key?("bubble_up_url"), "required bubble_up_url must be present"
  end

  def test_create
    response = { "id" => 1, "name" => "New List", "description_attachments" => [] }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/todosets/\d+/todolists\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.create(todoset_id: 2, name: "New List")
    assert_equal "New List", result["name"]
  end

  # ---------------------------------------------------------------------
  # replace: the server-native verbatim PUT.
  #
  # BC3's TodolistsController#update rebuilds the recordable from only the
  # permitted params, so this PUT is a FULL REPLACE and every omitted field is
  # cleared. That destructiveness is the whole reason `update` below is a
  # composite; `replace` keeps the raw operation reachable.
  # ---------------------------------------------------------------------

  def test_replace_sends_exactly_one_verbatim_put
    response = { "id" => 2, "name" => "Updated List", "description" => "", "description_attachments" => [] }
    captured = capture_put(response)

    result = @account.todolists.replace(id: 2, name: "Updated List")

    assert_equal "Updated List", result["name"]
    # One request, no read-before-write.
    assert_requested :put, "#{BASE_URL}/12345/todolists/2", times: 1
    assert_not_requested :get, "#{BASE_URL}/12345/todolists/2"
    assert_equal 1, captured[:bodies].length
    # Omitted stays omitted: replace never invents a description, and the
    # server clears what the body leaves out.
    assert_equal({ "name" => "Updated List" }, captured[:bodies].first)
  end

  def test_replace_sends_an_explicit_empty_description
    response = { "id" => 2, "name" => "Updated List", "description" => "", "description_attachments" => [] }
    captured = capture_put(response)

    @account.todolists.replace(id: 2, name: "Updated List", description: "")

    # "" survives compact_params (which strips only nil), so a caller who
    # states the clear gets a present-and-empty key, never JSON null.
    assert_equal({ "name" => "Updated List", "description" => "" }, captured[:bodies].first)
  end

  # ---------------------------------------------------------------------
  # update / edit: the merge-safe composites (GET then PUT).
  # ---------------------------------------------------------------------

  # A malformed writable field must abort before the PUT, never be coerced.
  #
  # Ruby has no typed decoder between the GET and the field read, so a plain
  # `|| ""` turns `false` into `""` and passes arrays, hashes and numbers
  # straight through. This endpoint is full-replace, so either outcome is
  # written back over the real value — the composite erases or corrupts the
  # field it exists to preserve, on a call that never mentioned it. The shipped
  # Todos analogue is tracked in #576.
  [ false, 0, [], {}, 42, true, [ "x" ], { "a" => 1 } ].each do |malformed|
    define_method("test_update_refuses_a_malformed_description_#{malformed.inspect}") do
      stub_todolist_get_and_put(todolist: full_todolist.merge("description" => malformed))

      error = assert_raises(Basecamp::ApiError) do
        @account.todolists.update(id: 2, name: "Renamed list")
      end

      assert_includes error.message, '"description" is not a string'
      assert_requested :get, "#{BASE_URL}/12345/todolists/2", times: 1
      assert_not_requested :put, "#{BASE_URL}/12345/todolists/2"
    end
  end

  # Row 9: the malformed BODY, one level up from malformed fields — and since
  # #544 the only level above them, the outer object having replaced the
  # object-in-an-arm the union used to model.
  [ 42, "nope", nil, [ "a" ], true ].each do |body|
    define_method("test_refuses_a_non_object_response_body_#{body.inspect}") do
      # stub_get passes Strings through verbatim, so encode first: a bare
      # `nope` is not JSON and would fail transport decode before the guard.
      stub_get("/12345/todolists/2", response_body: body.to_json)
      capture_put(full_todolist)

      error = assert_raises(Basecamp::ApiError) do
        @account.todolists.update(id: 2, name: "Renamed")
      end

      assert_includes error.message, "where a todolist object was expected"
      assert_not_requested :put, "#{BASE_URL}/12345/todolists/2"
    end
  end

  # The former envelope arms, now unmodelled wrappers.
  #
  # #544 collapsed the todolist/group oneOf into one flat structure, so the arm
  # guard is gone and these are no longer "a body with a bad arm" — they are
  # bodies with no name, which is precisely what the required-field guard is
  # for. The read path lost a LEVEL (object -> scalar, where it was object ->
  # object -> scalar); it did not lose the guard. Ruby still has no decoder
  # between the GET and the field read, flat shape or not.
  [ { "todolist" => nil }, { "todolist" => 42 }, { "todolist" => [ "a" ] },
    { "group" => nil }, { "group" => "nope" } ].each_with_index do |body, i|
    define_method("test_refuses_an_unmodelled_wrapper_#{i}") do
      stub_get("/12345/todolists/2", response_body: body)
      captured = capture_put(full_todolist)

      error = assert_raises(Basecamp::ApiError) do
        @account.todolists.update(id: 2, name: "Renamed")
      end

      assert_equal 'Todolist field "name" is missing from the response', error.message
      assert_not_requested :put, "#{BASE_URL}/12345/todolists/2"
      assert_empty captured[:bodies]
    end
  end

  # The Ruby-specific hazard on the shortened path: `body["name"]` on a String
  # is a SUBSTRING SEARCH, not a hash miss, so it does not raise — it lies.
  # This body answers "name" for name and "description" for description, so
  # without require_hash the composite would PUT the literal string "name" over
  # the record's real name and report success. That is why the defect outlived
  # eight review passes on #574, and why flattening the shape does not retire
  # the body-shape guard.
  def test_refuses_a_string_body_whose_text_contains_the_field_names
    stub_get("/12345/todolists/2", response_body: "name and description are words here".to_json)
    captured = capture_put(full_todolist)

    error = assert_raises(Basecamp::ApiError) do
      @account.todolists.update(id: 2, description: "<p>Revised</p>")
    end

    assert_includes error.message, "where a todolist object was expected"
    assert_not_requested :put, "#{BASE_URL}/12345/todolists/2"
    assert_empty captured[:bodies]
  end

  # Row 10: the guard's own error path must not throw. inspect is user code.
  def test_reports_an_unrenderable_caller_value_without_losing_the_diagnosis
    hostile = Class.new do
      def inspect = raise("nope")
    end.new
    stub_todolist_get_and_put

    error = assert_raises(Basecamp::UsageError) do
      @account.todolists.edit(id: 2) { |list| list.description = hostile }
    end

    assert_includes error.message, "must be a String"
    assert_not_requested :put, "#{BASE_URL}/12345/todolists/2"
  end

  # SPEC section 9 caps error messages; the value is embedded in them.
  def test_malformed_message_is_capped
    stub_todolist_get_and_put(todolist: full_todolist.merge("description" => [ "x" ] * 50_000))

    error = assert_raises(Basecamp::ApiError) do
      @account.todolists.update(id: 2, name: "Renamed")
    end

    assert_operator error.message.bytesize, :<=, 500
  end

  def test_edit_refuses_a_malformed_name_before_writing
    stub_todolist_get_and_put(todolist: full_todolist.merge("name" => 42))

    error = assert_raises(Basecamp::ApiError) do
      @account.todolists.edit(id: 2) { |list| list.description = "<p>New</p>" }
    end

    assert_includes error.message, '"name" is not a string'
    assert_not_requested :put, "#{BASE_URL}/12345/todolists/2"
  end

  # `name` is required and presence-validated, so absent, nil and "" from the
  # wire are all malformed. Classification is by ORIGIN: this name came off the
  # wire, so it is an ApiError; the caller supplying an empty name is a
  # UsageError, asserted separately below.
  [ [ "absent", nil ], [ "nil", :nil ], [ "empty", "" ] ].each do |label, value|
    define_method("test_#{label}_name_from_the_wire_is_a_malformed_response") do
      body = full_todolist.dup
      case value
      when nil then body.delete("name")
      when :nil then body["name"] = nil
      else body["name"] = value
      end
      stub_todolist_get_and_put(todolist: body)

      error = assert_raises(Basecamp::ApiError) do
        @account.todolists.update(id: 2, description: "<p>New</p>")
      end

      assert_includes error.message, '"name"'
      assert_not_requested :put, "#{BASE_URL}/12345/todolists/2"
    end
  end

  # The mirror of the read step: caller provenance, so UsageError. +edit+ yields
  # a mutable view of the full writable state and Ruby enforces nothing about
  # what comes back, so a block assigning a non-String would otherwise walk
  # straight into the full-replace PUT.
  [ 42, true, [], {}, [ "x" ], { "a" => 1 }, 0 ].each do |bad|
    define_method("test_edit_refuses_a_caller_supplied_#{bad.inspect}") do
      stub_todolist_get_and_put

      error = assert_raises(Basecamp::UsageError) do
        @account.todolists.edit(id: 2) { |list| list.description = bad }
      end

      assert_includes error.message, "must be a String"
      assert_not_requested :put, "#{BASE_URL}/12345/todolists/2"
    end
  end

  # The mirror case: same value, caller origin, so UsageError not ApiError.
  def test_caller_supplied_empty_name_is_a_usage_error
    stub_todolist_get_and_put

    assert_raises(Basecamp::UsageError) do
      @account.todolists.update(id: 2, name: "")
    end

    assert_not_requested :put, "#{BASE_URL}/12345/todolists/2"
  end

  def test_update_treats_absent_and_nil_description_as_empty
    [ full_todolist.except("description"), full_todolist.merge("description" => nil) ].each do |body|
      captured = stub_todolist_get_and_put(todolist: body)

      @account.todolists.update(id: 2, name: "Renamed list")

      assert_equal({ "name" => "Renamed list", "description" => "" }, captured[:bodies].first)
      WebMock.reset!
    end
  end

  def test_update_name_only_preserves_the_description
    captured = stub_todolist_get_and_put

    todolist = @account.todolists.update(id: 2, name: "Renamed list")

    assert_equal 2, todolist["id"]
    assert_requested :get, "#{BASE_URL}/12345/todolists/2", times: 1
    assert_requested :put, "#{BASE_URL}/12345/todolists/2", times: 1
    assert_equal({ "name" => "Renamed list", "description" => "<p>Ship the hardware</p>" }, \
                 captured[:bodies].first)
  end

  def test_update_description_only_preserves_the_name
    captured = stub_todolist_get_and_put

    @account.todolists.update(id: 2, description: "<p>Revised</p>")

    assert_equal({ "name" => "Hardware", "description" => "<p>Revised</p>" }, captured[:bodies].first)
  end

  def test_update_explicit_empty_description_clears_it
    captured = stub_todolist_get_and_put

    @account.todolists.update(id: 2, description: "")

    # Present-and-empty, never absent and never null: nil is the "keep it"
    # signal, so a clear has to travel as "" (SPEC section 18).
    assert_equal({ "name" => "Hardware", "description" => "" }, captured[:bodies].first)
  end

  # The composite is deliberately variant-agnostic: a todolist GROUP answers
  # the same route with a group-shaped body (parent is a Todolist,
  # group_position_url instead of groups_url) and must round-trip through the
  # exact same {name, description} overlay, with no type sniffing.
  #
  # The premise is asserted first, because it is the thing #544 repaired: the
  # group projection carries a description AT ALL. The pre-#544 model gave
  # groups their own shape with no description member and the shared fixture
  # matched it, so the round-trip below had nothing to preserve.
  def test_update_preserves_a_group_description_without_type_sniffing
    group = group_shaped_todolist
    assert_not_empty group["description"], "a group projection must carry a description to preserve"
    assert group.key?("group_position_url"), "a group's parent is a Todolist"
    assert_not group.key?("groups_url"), "only a todoset-parented list carries groups_url"
    assert_equal "Todolist", group["type"], "a group reports type Todolist; discrimination is structural"

    captured = stub_todolist_get_and_put(todolist: group)

    @account.todolists.update(id: 2, name: "Renamed group")

    assert_equal({ "name" => "Renamed group", "description" => group["description"] }, \
                 captured[:bodies].first)
  end

  # #544 removed the todolist/group envelope from the model, and with it the
  # arm lookup that used to unwrap one. A wrapped body is now a malformed
  # response rather than a second supported shape: unwrapping an unmodelled key
  # would take the WRAPPER's name and description and PUT them over the record,
  # on a call that never mentioned either. Refusing costs the caller an error;
  # guessing costs them the record.
  def test_update_refuses_the_former_envelope_instead_of_unwrapping_it
    captured = stub_todolist_get_and_put(todolist: { "todolist" => full_todolist })

    error = assert_raises(Basecamp::ApiError) do
      @account.todolists.update(id: 2, name: "Renamed list")
    end

    assert_equal 'Todolist field "name" is missing from the response', error.message
    assert_not_requested :put, "#{BASE_URL}/12345/todolists/2"
    assert_empty captured[:bodies]
  end

  # No key in the body diverts the read: the composite reads the flat top
  # level, always. The arm lookup this replaces preferred a `todolist`/`group`
  # key over the real fields, so a body carrying one had its wrapper's values
  # written over the record — and reported success, which is the silent
  # wrong-write class rather than an error the caller could see.
  def test_update_reads_the_flat_body_not_a_nested_group_key
    decoy = full_todolist.merge("name" => "Decoy", "description" => "<p>Decoy description</p>")
    captured = stub_todolist_get_and_put(todolist: full_todolist.merge("group" => decoy))

    @account.todolists.update(id: 2, name: "Renamed list")

    assert_equal({ "name" => "Renamed list", "description" => "<p>Ship the hardware</p>" }, \
                 captured[:bodies].first)
  end

  def test_update_hooks_observe_get_then_replace
    events = []
    account = create_account_client(account_id: "12345", hooks: CapturingHooks.new(events))
    stub_todolist_get_and_put

    account.todolists.update(id: 2, name: "observed")

    starts = events.select { |e| e[:event] == :on_operation_start }
    assert_equal [ %w[todolists get], %w[todolists replace] ], \
                 starts.map { |e| [ e[:info].service, e[:info].operation ] }
  end

  def test_edit_puts_the_full_state_back
    captured = stub_todolist_get_and_put

    todolist = @account.todolists.edit(id: 2) do |list|
      assert_equal "Hardware", list.name
      assert_equal "<p>Ship the hardware</p>", list.description
      list.name = "🚨 #{list.name}"
    end

    assert_equal 2, todolist["id"]
    assert_equal({ "name" => "🚨 Hardware", "description" => "<p>Ship the hardware</p>" }, \
                 captured[:bodies].first)
  end

  def test_edit_clears_the_description_and_keeps_the_name
    captured = stub_todolist_get_and_put

    @account.todolists.edit(id: 2) { |list| list.description = "" }

    assert_equal({ "name" => "Hardware", "description" => "" }, captured[:bodies].first)
  end

  # A field set to nil in the block has no meaning in a full write; it is the
  # same statement as "", not a preserve.
  def test_edit_nil_description_clears_present_and_empty
    captured = stub_todolist_get_and_put

    @account.todolists.edit(id: 2) { |list| list.description = nil }

    assert_equal({ "name" => "Hardware", "description" => "" }, captured[:bodies].first)
  end

  def test_edit_block_error_aborts_without_put
    captured = stub_todolist_get_and_put

    error = assert_raises(RuntimeError) do
      @account.todolists.edit(id: 2) do |list|
        list.name = "never written"
        raise "abort"
      end
    end

    assert_equal "abort", error.message
    assert_requested :get, "#{BASE_URL}/12345/todolists/2", times: 1
    assert_not_requested :put, "#{BASE_URL}/12345/todolists/2"
    assert_empty captured[:bodies]
  end

  def test_edit_requires_a_block
    error = assert_raises(ArgumentError) { @account.todolists.edit(id: 2) }

    assert_equal "edit requires a block", error.message
    assert_not_requested :get, "#{BASE_URL}/12345/todolists/2"
  end

  # name is presence-validated server-side, so blanking it is a 422 rather
  # than a preserve. The SDK refuses before spending the write.
  def test_edit_blank_name_raises_usage_error_without_put
    captured = stub_todolist_get_and_put

    error = assert_raises(Basecamp::UsageError) do
      @account.todolists.edit(id: 2) { |list| list.name = "" }
    end

    assert_equal "name must be present; a full write has no nil state and BC3 rejects a blank name with 422", \
                 error.message
    assert_not_requested :put, "#{BASE_URL}/12345/todolists/2"
    assert_empty captured[:bodies]
  end

  # This test previously asserted UsageError, which enshrined the bug: the
  # caller never mentioned the name, so blaming them for a nameless response was
  # backwards. Classification is by ORIGIN — the name came off the wire, so this
  # is an ApiError. The caller-supplied empty name stays a UsageError, covered by
  # test_caller_supplied_empty_name_is_a_usage_error.
  def test_update_of_a_nameless_todolist_reports_a_malformed_response_without_put
    captured = stub_todolist_get_and_put(todolist: full_todolist.merge("name" => nil))

    error = assert_raises(Basecamp::ApiError) do
      @account.todolists.update(id: 2, description: "<p>Revised</p>")
    end

    assert_equal 'Todolist field "name" is missing from the response', error.message
    assert_includes error.hint, "presence-validated server-side"
    assert_not_requested :put, "#{BASE_URL}/12345/todolists/2"
    assert_empty captured[:bodies]
  end

  def test_reposition
    captured = {}
    stub_request(:put, "https://3.basecampapi.com/12345/todosets/todolists/2/position.json")
      .with { |req| captured[:body] = JSON.parse(req.body) }
      .to_return(status: 204)

    assert_nil @account.todolists.reposition(todolist_id: 2, position: 3)
    assert_equal({ "position" => 3 }, captured[:body])
  end

  def test_reposition_not_found
    stub_request(:put, "https://3.basecampapi.com/12345/todosets/todolists/999/position.json")
      .to_return(status: 404, body: "")

    assert_raises(Basecamp::NotFoundError) do
      @account.todolists.reposition(todolist_id: 999, position: 1)
    end
  end

  # The get/update path label is the unsuffixed `{id}`. resource_id must still
  # carry it (predicate is `end_with?("Id") || == "id"`); a suffix-only
  # regression would silently drop resource_id here.
  def test_get_operation_metadata
    events = []
    account = create_account_client(account_id: "12345", hooks: CapturingHooks.new(events))

    response = { "id" => 2, "name" => "Sprint Tasks", "description_attachments" => [] }
    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todolists/\d+$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    account.todolists.get(id: 2)

    event = events.find { |e| e[:event] == :on_operation_start }
    assert event, "Expected on_operation_start to fire"
    info = event[:info]
    assert_equal "todolists", info.service
    assert_equal "get", info.operation
    assert_equal 2, info.resource_id
  end

  # The wire operation is `replace`, not `update`: `update` is the composite
  # in TodolistsExtensions and emits no operation of its own.
  def test_replace_operation_metadata
    events = []
    account = create_account_client(account_id: "12345", hooks: CapturingHooks.new(events))

    response = { "id" => 2, "name" => "Updated List", "description_attachments" => [] }
    stub_request(:put, %r{https://3\.basecampapi\.com/12345/todolists/\d+$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    account.todolists.replace(id: 2, name: "Updated List")

    event = events.find { |e| e[:event] == :on_operation_start }
    assert event, "Expected on_operation_start to fire"
    info = event[:info]
    assert_equal "todolists", info.service
    assert_equal "replace", info.operation
    assert_equal 2, info.resource_id
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
