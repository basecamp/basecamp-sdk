# frozen_string_literal: true

# Tests for the DocumentsService.
#
# Two layers here:
#
# * the generated, spec-conformant surface — list, get, create, and the raw
#   full-replace +replace+ — on single-resource paths without .json;
# * the hand-written merge-safe composites +update+ and +edit+, prepended by
#   the +on_load+ hook in basecamp.rb.
#
# BC3's DocumentsController#update rebuilds the recordable from only the
# permitted params, so PUT /documents/{id} is a FULL REPLACE: a body that omits
# +content+ erases it, and one that omits +title+ erases that too (the document
# then reads back as "Untitled"). Neither field is presence-validated, so
# neither omission is a 422 — both are a 200 that quietly clears, and nothing
# but the request body distinguishes a preserve from a clear. That is why the
# composites exist, and why these tests assert on captured bodies rather than
# on the response.

require "test_helper"

class DocumentsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_document(id: nil, title: nil)
    fixture = load_fixture("documents/get.json")
    fixture.merge "id" => id || fixture["id"], "title" => title || fixture["title"]
  end

  # The canonical fixture with a known title and content, so "preserved" and
  # "cleared" are distinguishable in the PUT body.
  def full_document(**overrides)
    load_fixture("documents/get.json").merge(
      "id" => 200,
      "title" => "Project Overview",
      "content" => "<div>From the store</div>"
    ).merge(overrides)
  end

  # Captures every PUT body so a test can assert the exact request count and
  # the exact bytes, not just "a PUT happened".
  def capture_put(response)
    captured = { bodies: [] }
    stub_request(:put, "#{BASE_URL}/12345/documents/200")
      .with { |req| captured[:bodies] << JSON.parse(req.body) }
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })
    captured
  end

  def stub_document_get_and_put(document: full_document)
    stub_get("/12345/documents/200", response_body: document)
    capture_put(document)
  end

  def test_list_documents
    stub_get("/12345/vaults/200/documents.json",
             response_body: [ sample_document, sample_document(id: 2, title: "Project Plan") ])

    documents = @account.documents.list(vault_id: 200).to_a

    assert_equal 2, documents.length
    assert_equal "Project Overview", documents[0]["title"]
    assert_equal "Project Plan", documents[1]["title"]
  end

  def test_get_document
    # Generated service: /documents/{id} without .json
    stub_get("/12345/documents/200", response_body: sample_document(id: 200))

    document = @account.documents.get(document_id: 200)

    assert_equal 200, document["id"]
    assert_equal "Project Overview", document["title"]
  end

  def test_create_document
    new_document = sample_document(id: 999, title: "New Document")
    stub_post("/12345/vaults/200/documents.json", response_body: new_document)

    document = @account.documents.create(
      vault_id: 200,
      title: "New Document",
      content: "<p>Document content</p>",
      status: "active"
    )

    assert_equal 999, document["id"]
    assert_equal "New Document", document["title"]
  end

  def test_create_draft_document
    draft_document = sample_document(id: 999, title: "Draft Document")
    draft_document["status"] = "drafted"
    stub_post("/12345/vaults/200/documents.json", response_body: draft_document)

    document = @account.documents.create(
      vault_id: 200,
      title: "Draft Document",
      status: "drafted"
    )

    assert_equal "drafted", document["status"]
  end

  def test_create_with_subscriptions
    new_document = sample_document(id: 999, title: "Quiet Doc")
    stub_post("/12345/vaults/200/documents.json", response_body: new_document)

    @account.documents.create(
      vault_id: 200, title: "Quiet Doc", subscriptions: [ 111, 222 ]
    )

    assert_requested(:post, "#{BASE_URL}/12345/vaults/200/documents.json",
      body: hash_including("subscriptions" => [ 111, 222 ]))
  end

  # ---------------------------------------------------------------------
  # replace: the server-native verbatim PUT.
  #
  # Sharp by construction — every field the body omits, the server clears.
  # +replace+ keeps that raw operation reachable; the composites below blunt it.
  # ---------------------------------------------------------------------

  def test_replace_sends_sparse_verbatim_with_no_get
    captured = capture_put(full_document("title" => "Updated Title", "content" => ""))

    document = @account.documents.replace(document_id: 200, title: "Updated Title")

    assert_equal "Updated Title", document["title"]
    # One request, no read-before-write.
    assert_requested :put, "#{BASE_URL}/12345/documents/200", times: 1
    assert_not_requested :get, "#{BASE_URL}/12345/documents/200"
    assert_equal 1, captured[:bodies].length
    # Omitted stays omitted: replace never invents a content, and the server
    # clears what the body leaves out.
    assert_equal({ "title" => "Updated Title" }, captured[:bodies].first)
  end

  def test_replace_sends_an_explicit_empty_content
    captured = capture_put(full_document("content" => ""))

    @account.documents.replace(document_id: 200, title: "Updated Title", content: "")

    # "" survives compact_params (which strips only nil), so a caller who
    # states the clear gets a present-and-empty key, never JSON null.
    assert_equal({ "title" => "Updated Title", "content" => "" }, captured[:bodies].first)
  end

  # ---------------------------------------------------------------------
  # update / edit: the merge-safe composites (GET then PUT).
  # ---------------------------------------------------------------------

  def test_update_merges_unset_fields
    captured = stub_document_get_and_put

    document = @account.documents.update(document_id: 200, title: "Updated Title")

    assert_equal 200, document["id"]
    assert_requested :get, "#{BASE_URL}/12345/documents/200", times: 1
    assert_equal 1, captured[:bodies].length
    # The writable set is exactly {title, content}: the unmentioned field is
    # carried out of the GET, and nothing else rides along.
    assert_equal({ "title" => "Updated Title", "content" => "<div>From the store</div>" },
                 captured[:bodies].first)
  end

  def test_update_merges_content_only
    captured = stub_document_get_and_put

    @account.documents.update(document_id: 200, content: "<div>new body</div>")

    # The mirror case: the title must survive, or the server resets it to
    # "Untitled" on a call that never mentioned it.
    assert_equal({ "title" => "Project Overview", "content" => "<div>new body</div>" },
                 captured[:bodies].first)
  end

  def test_update_clears_content_with_explicit_empty_string
    captured = stub_document_get_and_put

    # Ruby distinguishes "omitted" (nil) from "stated empty" (""), so unlike
    # Go — where "" is the zero value and reads as unset — the composite update
    # can clear.
    @account.documents.update(document_id: 200, content: "")

    body = captured[:bodies].first
    assert_includes body.keys, "content"
    assert_equal "", body["content"]
    assert_equal "Project Overview", body["title"]
  end

  def test_update_clears_title_with_explicit_empty_string
    captured = stub_document_get_and_put

    @account.documents.update(document_id: 200, title: "")

    body = captured[:bodies].first
    assert_includes body.keys, "title"
    assert_equal "", body["title"]
    assert_equal "<div>From the store</div>", body["content"]
  end

  def test_update_hooks_observe_get_then_replace
    events = []
    account = create_account_client(account_id: "12345", hooks: TrackingHooks.new(events))
    stub_document_get_and_put

    account.documents.update(document_id: 200, title: "observed")

    # The composite composes the public get and replace, so hooks see the two
    # wire operations rather than one synthetic composite.
    starts = events.select { |e| e[:event] == :on_operation_start }
    assert_equal [ %w[documents get], %w[documents replace] ], \
                 starts.map { |e| [ e[:info].service, e[:info].operation ] }
  end

  def test_edit_puts_full_state_back
    captured = stub_document_get_and_put

    document = @account.documents.edit(document_id: 200) do |doc|
      assert_equal "Project Overview", doc.title
      assert_equal "<div>From the store</div>", doc.content
      doc.title = "🚨 #{doc.title}"
    end

    assert_equal 200, document["id"]
    assert_equal({ "title" => "🚨 Project Overview", "content" => "<div>From the store</div>" },
                 captured[:bodies].first)
  end

  # A clear has to REACH THE WIRE as a present-and-empty key. Omitting it would
  # hand the clear back to the server's own rebuild — the same 200, but as an
  # accident rather than an intent — and JSON null is out (SPEC section 18 body
  # compaction, which is why put_fields normalises nil to "").
  def test_edit_clears_content_present_and_empty
    captured = stub_document_get_and_put

    @account.documents.edit(document_id: 200) { |doc| doc.content = "" }

    body = captured[:bodies].first
    assert_includes body.keys, "content"
    assert_equal "", body["content"]
    assert_not_nil body["content"], "a clear must travel as \"\", never as JSON null"
    assert_equal "Project Overview", body["title"]
  end

  def test_edit_clears_title_present_and_empty
    captured = stub_document_get_and_put

    @account.documents.edit(document_id: 200) { |doc| doc.title = "" }

    body = captured[:bodies].first
    assert_includes body.keys, "title"
    assert_equal "", body["title"]
    assert_not_nil body["title"], "a clear must travel as \"\", never as JSON null"
    assert_equal "<div>From the store</div>", body["content"]
  end

  # nil is the struct's own starting state, so clearing by assigning nil is
  # idiomatic; put_fields normalises it to "" rather than letting compact_params
  # drop the key.
  def test_edit_nil_assignment_travels_as_empty_string
    captured = stub_document_get_and_put

    @account.documents.edit(document_id: 200) { |doc| doc.content = nil }

    body = captured[:bodies].first
    assert_includes body.keys, "content"
    assert_equal "", body["content"]
  end

  def test_edit_block_error_aborts_without_put
    captured = stub_document_get_and_put

    assert_raises(RuntimeError) do
      @account.documents.edit(document_id: 200) do |doc|
        doc.title = "never written"
        raise "abort"
      end
    end

    assert_empty captured[:bodies]
    assert_not_requested :put, "#{BASE_URL}/12345/documents/200"
  end

  def test_edit_requires_a_block
    assert_raises(ArgumentError) { @account.documents.edit(document_id: 200) }
  end

  def test_edit_hooks_observe_get_then_replace
    events = []
    account = create_account_client(account_id: "12345", hooks: TrackingHooks.new(events))
    stub_document_get_and_put

    account.documents.edit(document_id: 200) { |doc| doc.title = "observed" }

    starts = events.select { |e| e[:event] == :on_operation_start }
    assert_equal [ %w[documents get], %w[documents replace] ], \
                 starts.map { |e| [ e[:info].service, e[:info].operation ] }
  end

  # ---------------------------------------------------------------------
  # A malformed GET field must never reach the full-replace PUT (#576).
  #
  # update/edit GET the document, read each writable field, and PUT the FULL
  # representation back, so every value read is written — including one the
  # caller never mentioned. Ruby's +||+ treats only nil and false as falsy, so a
  # plain <tt>body["content"] || ""</tt> ERASES the field on +false+ and passes
  # arrays, hashes, numbers and +true+ straight through to be written verbatim.
  # There is no typed decoder between the GET and the read: the generated +get+
  # returns a raw Hash. That is exactly why MergeSafe's guards exist in Ruby
  # (and Python and TypeScript) and nowhere else.
  #
  # The assertion that matters is the ORDERING — assert_not_requested :put. A
  # guard that fires after the PUT has already lost the field.
  # ---------------------------------------------------------------------

  MALFORMED_VALUES = [ false, 0, [], {}, 42, true, [ "x" ], { "a" => 1 } ].freeze
  WRITABLE_STRINGS = %w[title content].freeze
  # The other writable field, so a test can name one and probe the other.
  OTHER_FIELD = { "title" => :content, "content" => :title }.freeze

  WRITABLE_STRINGS.each do |field|
    MALFORMED_VALUES.each do |malformed|
      define_method("test_update_refuses_a_malformed_#{field}_#{malformed.inspect}") do
        captured = stub_document_get_and_put(document: full_document(field => malformed))

        # Names the OTHER field, so nothing the caller passed masks the
        # malformed one.
        args = { document_id: 200, OTHER_FIELD.fetch(field) => "New value" }
        error = assert_raises(Basecamp::ApiError) { @account.documents.update(**args) }

        assert_includes error.message, "Document field #{field.inspect} is not a string"
        # api_error, not usage: the value arrived in a successful API response.
        assert_equal Basecamp::ErrorCode::API, error.code
        assert_requested :get, "#{BASE_URL}/12345/documents/200", times: 1
        assert_not_requested :put, "#{BASE_URL}/12345/documents/200"
        assert_empty captured[:bodies]
      end
    end

    define_method("test_edit_refuses_a_malformed_#{field}_before_writing") do
      captured = stub_document_get_and_put(document: full_document(field => 42))

      error = assert_raises(Basecamp::ApiError) do
        @account.documents.edit(document_id: 200) { |doc| doc.title = "New value" }
      end

      assert_includes error.message, "Document field #{field.inspect} is not a string"
      assert_not_requested :put, "#{BASE_URL}/12345/documents/200"
      assert_empty captured[:bodies]
    end
  end

  # The other half of the rule, for an OPTIONAL field: missing and nil are not
  # malformed, they are empty. "" is what the server already holds, so there is
  # nothing to preserve and nothing to refuse.
  #
  # +content+ only. +title+ is <tt>@required</tt> in the spec and gets the
  # opposite treatment below.
  { "missing" => ->(doc) { doc.except("content") }, "nil" => ->(doc) { doc.merge("content" => nil) } }
    .each do |label, mangle|
    define_method("test_#{label}_content_stays_genuinely_empty") do
      captured = stub_document_get_and_put(document: mangle.call(full_document))

      @account.documents.update(document_id: 200, title: "New value")

      assert_equal "", captured[:bodies].first["content"]
      assert_equal "New value", captured[:bodies].first["title"]
    end

    # Document#title is <tt>super.presence || "Untitled"</tt> and the spec marks
    # the field <tt>@required</tt>, so BC3 can never render it blank: a missing
    # or nil title in a 2xx body is a MALFORMED RESPONSE, not an empty title.
    # Coalescing it to "" would blank the real title on a call that only touched
    # +content+ — the same defect class as a forwarded non-string, in the one
    # shape <tt>|| ""</tt> looks correct.
    define_method("test_update_refuses_a_#{label}_title_before_writing") do
      captured = stub_document_get_and_put(document: title_mangled(full_document, label))

      error = assert_raises(Basecamp::ApiError) do
        @account.documents.update(document_id: 200, content: "<div>New body.</div>")
      end

      assert_includes error.message, %(Document field "title" is required)
      assert_equal Basecamp::ErrorCode::API, error.code
      assert_not_requested :put, "#{BASE_URL}/12345/documents/200"
      assert_empty captured[:bodies]
    end

    define_method("test_edit_refuses_a_#{label}_title_before_writing") do
      captured = stub_document_get_and_put(document: title_mangled(full_document, label))

      error = assert_raises(Basecamp::ApiError) do
        @account.documents.edit(document_id: 200) { |doc| doc.content = "<div>New body.</div>" }
      end

      assert_includes error.message, %(Document field "title" is required)
      assert_not_requested :put, "#{BASE_URL}/12345/documents/200"
      assert_empty captured[:bodies]
    end
  end

  # Drops or nils the title, mirroring the "missing"/"nil" labels above.
  def title_mangled(document, label)
    label == "missing" ? document.except("title") : document.merge("title" => nil)
  end

  # One level up from the field guards: a successful GET can return a scalar,
  # an Array or nil, and <tt>body["title"]</tt> would raise a raw TypeError on
  # an Integer or Array — or return a silent nil substring match on a String —
  # instead of the documented statusless api_error.
  [ 42, "nope", nil, [ "a" ], true ].each do |body|
    define_method("test_update_refuses_a_non_object_response_body_#{body.inspect}") do
      # stub_get passes Strings through verbatim, so encode first: a bare
      # `nope` is not JSON and would fail transport decode before the guard.
      stub_get("/12345/documents/200", response_body: body.to_json)
      captured = capture_put(full_document)

      error = assert_raises(Basecamp::ApiError) do
        @account.documents.update(document_id: 200, title: "New Title")
      end

      assert_includes error.message, "GetDocument returned"
      assert_includes error.message, "where a document object was expected"
      assert_equal Basecamp::ErrorCode::API, error.code
      assert_not_requested :put, "#{BASE_URL}/12345/documents/200"
      assert_empty captured[:bodies]
    end

    define_method("test_edit_refuses_a_non_object_response_body_#{body.inspect}") do
      stub_get("/12345/documents/200", response_body: body.to_json)
      captured = capture_put(full_document)

      assert_raises(Basecamp::ApiError) do
        @account.documents.edit(document_id: 200) { |doc| doc.title = "New Title" }
      end

      assert_not_requested :put, "#{BASE_URL}/12345/documents/200"
      assert_empty captured[:bodies]
    end
  end

  # The malformed value is interpolated into the message, so SPEC section 9's
  # 500-byte cap has to survive a huge body.
  def test_malformed_message_is_capped
    stub_document_get_and_put(document: full_document("content" => [ "x" ] * 50_000))

    error = assert_raises(Basecamp::ApiError) do
      @account.documents.update(document_id: 200, title: "New Title")
    end

    assert_operator error.message.bytesize, :<=, 500
  end

  # The malformed-response errors point at the deliberate-overwrite escape
  # hatch, and it has to name a method that actually exists on the service.
  def test_malformed_error_names_the_escape_hatch
    stub_document_get_and_put(document: full_document("content" => 42))

    error = assert_raises(Basecamp::ApiError) do
      @account.documents.update(document_id: 200, title: "New Title")
    end

    assert_includes error.hint, Basecamp::Services::DocumentsExtensions::ESCAPE_HATCH
    assert_respond_to @account.documents, :replace
    assert_not error.retryable, "re-requesting cannot repair a malformed body"
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
