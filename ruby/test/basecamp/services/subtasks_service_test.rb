# frozen_string_literal: true

# Tests for the SubtasksService (generated from OpenAPI spec)
#
# A subtask is a CardStep on the wire — `type` stays "Kanban::Step" — reached
# through the canonical flat /subtasks routes bc3 documents (bc3#12659).

require "test_helper"

class SubtasksServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_subtask(id: 1, position: 1)
    {
      "id" => id,
      "status" => "active",
      "title" => "Hero shot on the desk",
      "type" => "Kanban::Step",
      "url" => "https://3.basecampapi.com/12345/buckets/1/subtasks/#{id}.json",
      "position" => position,
      "completed" => false,
      "parent" => { "id" => 200, "title" => "Shot list", "type" => "Todo" },
      "assignees" => [],
      "completion_url" => "https://3.basecampapi.com/12345/subtasks/#{id}/completion.json"
    }
  end

  def test_list
    stub_get("/12345/recordings/200/subtasks.json",
             response_body: [ sample_subtask(id: 1, position: 1), sample_subtask(id: 2, position: 2) ])

    subtasks = @account.subtasks.list(recording_id: 200).to_a

    assert_equal [ 1, 2 ], subtasks.map { |s| s["id"] }
    assert_equal "Kanban::Step", subtasks.first["type"]
  end

  def test_list_not_found
    stub_get("/12345/recordings/999/subtasks.json", status: 404, response_body: { "error" => "Not found" })

    assert_raises(Basecamp::NotFoundError) do
      @account.subtasks.list(recording_id: 999).to_a
    end
  end

  def test_get
    stub_get("/12345/subtasks/42", response_body: sample_subtask(id: 42))

    subtask = @account.subtasks.get(subtask_id: 42)

    assert_equal 42, subtask["id"]
    assert_equal "Hero shot on the desk", subtask["title"]
  end

  def test_get_not_found
    stub_get("/12345/subtasks/999", status: 404, response_body: { "error" => "Not found" })

    assert_raises(Basecamp::NotFoundError) do
      @account.subtasks.get(subtask_id: 999)
    end
  end

  def test_create
    stub = stub_post("/12345/recordings/200/subtasks.json", response_body: sample_subtask(id: 99))
      .with(body: { title: "Book the room", due_on: "2026-09-20", assignee_ids: [ 30068628 ] }.to_json)

    subtask = @account.subtasks.create(recording_id: 200, title: "Book the room", due_on: "2026-09-20", assignee_ids: [ 30068628 ])

    assert_equal 99, subtask["id"]
    assert_requested(stub)
  end

  def test_create_forbidden_on_a_recording_without_subtasks
    stub_post("/12345/recordings/300/subtasks.json", status: 403, response_body: { "error" => "Forbidden" })

    assert_raises(Basecamp::ForbiddenError) do
      @account.subtasks.create(recording_id: 300, title: "Nope")
    end
  end

  def test_update_sends_only_the_fields_given
    stub = stub_put("/12345/subtasks/42", response_body: sample_subtask(id: 42))
      .with(body: { title: "Book the big room" }.to_json)

    @account.subtasks.update(subtask_id: 42, title: "Book the big room")

    assert_requested(stub)
  end

  def test_update_clears_assignees_with_an_explicit_empty_list
    stub = stub_put("/12345/subtasks/42", response_body: sample_subtask(id: 42))
      .with(body: { assignee_ids: [] }.to_json)

    @account.subtasks.update(subtask_id: 42, assignee_ids: [])

    assert_requested(stub)
  end

  def test_update_validation_error
    stub_put("/12345/subtasks/42", status: 422,
             response_body: { "errors" => { "due_on" => [ "is not a valid date" ] } })

    assert_raises(Basecamp::ValidationError) do
      @account.subtasks.update(subtask_id: 42, due_on: "not-a-date")
    end
  end

  def test_complete_and_uncomplete
    complete = stub_post("/12345/subtasks/42/completion.json", response_body: "", status: 204)
    uncomplete = stub_delete("/12345/subtasks/42/completion.json")

    assert_nil @account.subtasks.complete(subtask_id: 42)
    assert_nil @account.subtasks.uncomplete(subtask_id: 42)

    assert_requested(complete)
    assert_requested(uncomplete)
  end

  def test_complete_not_found
    stub_post("/12345/subtasks/999/completion.json", status: 404, response_body: { "error" => "Not found" })

    assert_raises(Basecamp::NotFoundError) do
      @account.subtasks.complete(subtask_id: 999)
    end
  end

  def test_uncomplete_not_found
    stub_delete("/12345/subtasks/999/completion.json", status: 404)

    assert_raises(Basecamp::NotFoundError) do
      @account.subtasks.uncomplete(subtask_id: 999)
    end
  end

  def test_reposition
    stub = stub_put("/12345/subtasks/42/position.json", response_body: "", status: 204)
      .with(body: { position: 4 }.to_json)

    assert_nil @account.subtasks.reposition(subtask_id: 42, position: 4)

    assert_requested(stub)
  end

  def test_reposition_validation_error
    stub_put("/12345/subtasks/42/position.json", status: 422,
             response_body: { "errors" => { "position" => [ "must be greater than 0" ] } })

    assert_raises(Basecamp::ValidationError) do
      @account.subtasks.reposition(subtask_id: 42, position: 0)
    end
  end

  def test_delete
    stub_delete("/12345/subtasks/42")

    assert_nil @account.subtasks.delete(subtask_id: 42)
  end

  def test_delete_forbidden
    stub_delete("/12345/subtasks/42", status: 403)

    assert_raises(Basecamp::ForbiddenError) do
      @account.subtasks.delete(subtask_id: 42)
    end
  end
end
