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

  def test_update_todo
    # Generated service: /todos/{id} without .json
    updated = sample_todo(content: "Updated content")
    stub_put("/12345/todos/456", response_body: updated)

    todo = @account.todos.update(
      todo_id: 456,
      content: "Updated content"
    )

    assert_equal "Updated content", todo["content"]
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
end
