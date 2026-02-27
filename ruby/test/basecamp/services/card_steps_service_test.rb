# frozen_string_literal: true

# Tests for the CardStepsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Single-resource paths without .json (update)
# - reposition uses source_id instead of step_id

require "test_helper"

class CardStepsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_step(id: 1, title: "Review code")
    {
      "id" => id,
      "title" => title,
      "completed" => false,
      "due_on" => nil,
      "assignees" => []
    }
  end

  def test_create_step
    new_step = sample_step(id: 999, title: "New step")
    stub_post("/12345/card_tables/cards/200/steps.json", response_body: new_step)

    step = @account.card_steps.create(
      card_id: 200,
      title: "New step",
      due_on: "2024-12-15",
      assignees: [ 1, 2 ]
    )

    assert_equal 999, step["id"]
    assert_equal "New step", step["title"]
  end

  def test_update_step
    # Generated service: /card_tables/steps/{id} without .json
    updated_step = sample_step(id: 200, title: "Updated step")
    stub_put("/12345/card_tables/steps/200", response_body: updated_step)

    step = @account.card_steps.update(
      step_id: 200,
      title: "Updated step",
      due_on: "2024-12-20"
    )

    assert_equal "Updated step", step["title"]
  end

  def test_set_completion_on
    completed_step = sample_step(id: 200)
    completed_step["completed"] = true
    stub_put("/12345/card_tables/steps/200/completions.json", response_body: completed_step)

    step = @account.card_steps.set_completion(step_id: 200, completion: "on")

    assert step["completed"]
  end

  def test_set_completion_off
    uncompleted_step = sample_step(id: 200)
    stub_put("/12345/card_tables/steps/200/completions.json", response_body: uncompleted_step)

    step = @account.card_steps.set_completion(step_id: 200, completion: "")

    assert_not step["completed"]
  end

  def test_reposition_step
    # Generated service uses source_id instead of step_id
    stub_post("/12345/card_tables/cards/200/positions.json", response_body: {})

    result = @account.card_steps.reposition(
      card_id: 200,
      source_id: 300,
      position: 2
    )

    assert_nil result
  end
end
