# frozen_string_literal: true

# Tests for the TodosService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Some paths without .json suffix (get, update, trash)
# - No client-side validation (API validates)

require "test_helper"

class TodosServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list_todos
    stub_get("/12345/todolists/200/todos.json",
             response_body: [ sample_todo, sample_todo(id: 789, content: "Another todo") ])

    todos = @account.todos.list(todolist_id: 200).to_a

    assert_equal 2, todos.length
    assert_equal "Test todo", todos[0]["content"]
  end

  def test_list_todos_with_completed_filter
    stub_request(:get, "https://3.basecampapi.com/12345/todolists/200/todos.json")
      .with(query: { completed: "true" })
      .to_return(status: 200, body: [ sample_todo ].to_json)

    todos = @account.todos.list(todolist_id: 200, completed: true).to_a

    assert_equal 1, todos.length
  end

  def test_get_todo
    # Generated service: /todos/{id} without .json
    stub_get("/12345/todos/456", response_body: sample_todo)

    todo = @account.todos.get(todo_id: 456)

    assert_equal 456, todo["id"]
    assert_equal "Test todo", todo["content"]
  end

  def test_create_todo
    new_todo = sample_todo(id: 999, content: "New task")
    stub_post("/12345/todolists/200/todos.json", response_body: new_todo)

    todo = @account.todos.create(
      todolist_id: 200,
      content: "New task",
      assignee_ids: [ 1, 2 ]
    )

    assert_equal 999, todo["id"]
    assert_equal "New task", todo["content"]
  end

  def full_todo(id: 456, **overrides)
    sample_todo(id: id).merge(
      "description" => "<p>From the store</p>",
      "due_on" => "2024-03-01",
      "starts_on" => "2024-02-01",
      "assignees" => [ { "id" => 100, "name" => "Jane Doe" } ],
      "completion_subscribers" => [ { "id" => 555, "name" => "Sub Scriber" } ]
    ).merge(overrides)
  end

  def stub_todo_get_and_put(todo: full_todo)
    captured = {}
    stub_get("/12345/todos/456", response_body: todo)
    stub_request(:put, "https://3.basecampapi.com/12345/todos/456")
      .with { |req| captured[:body] = JSON.parse(req.body) }
      .to_return(status: 200, body: todo.to_json, headers: { "Content-Type" => "application/json" })
    captured
  end

  def test_update_merges_unset_fields
    captured = stub_todo_get_and_put

    todo = @account.todos.update(todo_id: 456, content: "Updated content")

    assert_equal 456, todo["id"]
    body = captured[:body]
    assert_equal "Updated content", body["content"]
    assert_equal "<p>From the store</p>", body["description"]
    assert_equal "2024-03-01", body["due_on"]
    assert_equal "2024-02-01", body["starts_on"]
    assert_equal [ 100 ], body["assignee_ids"]
    assert_equal [ 555 ], body["completion_subscriber_ids"]
    assert_not_includes body.keys, "notify"
  end

  def test_update_explicit_empty_array_clears
    captured = stub_todo_get_and_put

    @account.todos.update(todo_id: 456, assignee_ids: [])

    body = captured[:body]
    assert_equal [], body["assignee_ids"]
    assert_equal [ 555 ], body["completion_subscriber_ids"]
    assert_equal "Test todo", body["content"]
  end

  def test_update_notify_only_when_true
    captured = stub_todo_get_and_put

    @account.todos.update(todo_id: 456, content: "ping", notify: true)

    assert_equal true, captured[:body]["notify"]
  end

  def test_update_hooks_observe_get_then_replace
    events = []
    account = create_account_client(account_id: "12345", hooks: TrackingHooks.new(events))
    stub_todo_get_and_put

    account.todos.update(todo_id: 456, content: "observed")

    starts = events.select { |e| e[:event] == :on_operation_start }
    assert_equal [ %w[todos get], %w[todos replace] ], \
                 starts.map { |e| [ e[:info].service, e[:info].operation ] }
  end

  def test_edit_puts_full_state_back
    captured = stub_todo_get_and_put

    todo = @account.todos.edit(todo_id: 456) do |t|
      assert_equal "Test todo", t.content
      t.content = "🚨 #{t.content}"
    end

    assert_equal 456, todo["id"]
    body = captured[:body]
    assert_equal "🚨 Test todo", body["content"]
    assert_equal "<p>From the store</p>", body["description"]
    assert_equal [ 100 ], body["assignee_ids"]
  end

  def test_edit_clears_date_by_omission
    captured = stub_todo_get_and_put

    @account.todos.edit(todo_id: 456) do |t|
      assert_equal "2024-03-01", t.due_on
      t.due_on = ""
    end

    body = captured[:body]
    assert_not_includes body.keys, "due_on"
    assert_equal "Test todo", body["content"]
  end

  def test_edit_clears_description_and_ids_present_and_empty
    captured = stub_todo_get_and_put

    @account.todos.edit(todo_id: 456) do |t|
      t.description = ""
      t.assignee_ids = []
      t.completion_subscriber_ids = []
    end

    body = captured[:body]
    assert_equal "", body["description"]
    assert_equal [], body["assignee_ids"]
    assert_equal [], body["completion_subscriber_ids"]
  end

  def test_edit_block_error_aborts_without_put
    captured = stub_todo_get_and_put

    assert_raises(RuntimeError) do
      @account.todos.edit(todo_id: 456) do |t|
        t.content = "never written"
        raise "abort"
      end
    end

    assert_nil captured[:body]
  end

  def test_edit_requires_a_block
    assert_raises(ArgumentError) { @account.todos.edit(todo_id: 456) }
  end

  def test_edit_nil_id_list_raises_usage_error_without_put
    captured = stub_todo_get_and_put

    error = assert_raises(Basecamp::UsageError) do
      @account.todos.edit(todo_id: 456) { |t| t.assignee_ids = nil }
    end

    assert_match(/use \[\] to clear/, error.message)
    assert_nil captured[:body]
  end

  def test_edit_hooks_observe_get_then_replace
    events = []
    account = create_account_client(account_id: "12345", hooks: TrackingHooks.new(events))
    stub_todo_get_and_put

    account.todos.edit(todo_id: 456) { |t| t.content = "observed" }

    starts = events.select { |e| e[:event] == :on_operation_start }
    assert_equal [ %w[todos get], %w[todos replace] ], \
                 starts.map { |e| [ e[:info].service, e[:info].operation ] }
  end

  def test_replace_sends_sparse_verbatim_with_no_get
    captured = {}
    stub_request(:put, "https://3.basecampapi.com/12345/todos/456")
      .with { |req| captured[:body] = JSON.parse(req.body) }
      .to_return(status: 200, body: full_todo.to_json, headers: { "Content-Type" => "application/json" })

    todo = @account.todos.replace(todo_id: 456, content: "the whole new todo")

    assert_equal 456, todo["id"]
    body = captured[:body]
    assert_equal "the whole new todo", body["content"]
    %w[description assignee_ids completion_subscriber_ids notify due_on starts_on].each do |field|
      assert_not_includes body.keys, field
    end
    assert_not_requested :get, "https://3.basecampapi.com/12345/todos/456"
  end

  def test_complete_todo
    stub_post("/12345/todos/456/completion.json", response_body: {}, status: 204)

    result = @account.todos.complete(todo_id: 456)

    assert_nil result
  end

  def test_uncomplete_todo
    stub_delete("/12345/todos/456/completion.json")

    result = @account.todos.uncomplete(todo_id: 456)

    assert_nil result
  end

  def test_reposition_todo
    stub_put("/12345/todos/456/position.json", response_body: {})

    result = @account.todos.reposition(todo_id: 456, position: 3)

    assert_nil result
  end

  def test_trash_todo
    # Generated service: /todos/{id} without .json
    stub_delete("/12345/todos/456")

    result = @account.todos.trash(todo_id: 456)

    assert_nil result
  end

  # The typed decode (Basecamp::Types::Todo → RichTextAttachment) carries the
  # rich-text description's inline files. Pixel dimensions arrive float-spelled
  # (1024.0) for images and null for non-image blobs; parse_integer decodes
  # both faithfully — 1024.0 → 1024 (to_i) and null → nil. This is a
  # decode-only assertion: re-encoding a nil dimension is out of scope here,
  # since to_h calls .compact and drops the nil key (an SDK-wide encoder
  # behavior documented in SPEC.md §10 Type Fidelity).
  def test_description_attachment_dimensions_decode
    todo = Basecamp::Types::Todo.new(
      "id" => 456,
      "content" => "Buy milk",
      "description_attachments" => [
        {
          "id" => 1_069_480_000, "sgid" => "BAh-img", "filename" => "leto-schematic.png",
          "content_type" => "image/png", "byte_size" => 284_111,
          "download_url" => "https://3.basecampapi.com/12345/buckets/1/blobs/img/download/leto-schematic.png",
          "width" => 1024.0, "height" => 768, "previewable" => true,
          "preview_url" => "https://3.basecampapi.com/12345/buckets/1/blobs/img/previews/leto-schematic.png",
          "thumbnail_url" => "https://3.basecampapi.com/12345/buckets/1/blobs/img/thumbnails/leto-schematic.png"
        },
        {
          "id" => 1_069_480_001, "sgid" => "BAh-pdf", "filename" => "leto-spec.pdf",
          "content_type" => "application/pdf", "byte_size" => 1_048_576,
          "download_url" => "https://3.basecampapi.com/12345/buckets/1/blobs/pdf/download/leto-spec.pdf",
          "width" => nil, "height" => nil, "previewable" => false,
          "preview_url" => "https://3.basecampapi.com/12345/buckets/1/blobs/pdf/previews/leto-spec.pdf",
          "thumbnail_url" => "https://3.basecampapi.com/12345/buckets/1/blobs/pdf/thumbnails/leto-spec.pdf"
        }
      ]
    )

    image, pdf = todo.description_attachments

    # Float-spelled 1024.0 decodes to the integer 1024.
    assert_equal 1024, image.width
    assert_equal 768, image.height
    assert_equal "image/png", image.content_type

    # null dimensions decode to nil (not a sentinel 0).
    assert_nil pdf.width
    assert_nil pdf.height
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
