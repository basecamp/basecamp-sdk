# frozen_string_literal: true

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
    stub_post("/12345/buckets/100/card_tables/cards/200/steps.json", response_body: new_step)

    step = @account.card_steps.create(
      project_id: 100,
      card_id: 200,
      title: "New step",
      due_on: "2024-12-15",
      assignees: [ 1, 2 ]
    )

    assert_equal 999, step["id"]
    assert_equal "New step", step["title"]
  end

  def test_update_step
    updated_step = sample_step(id: 200, title: "Updated step")
    stub_put("/12345/buckets/100/card_tables/steps/200.json", response_body: updated_step)

    step = @account.card_steps.update(
      project_id: 100,
      step_id: 200,
      title: "Updated step",
      due_on: "2024-12-20"
    )

    assert_equal "Updated step", step["title"]
  end

  def test_complete_step
    completed_step = sample_step(id: 200)
    completed_step["completed"] = true
    stub_put("/12345/buckets/100/card_tables/steps/200/completions.json", response_body: completed_step)

    step = @account.card_steps.complete(project_id: 100, step_id: 200)

    assert step["completed"]
  end

  def test_uncomplete_step
    uncompleted_step = sample_step(id: 200)
    stub_delete("/12345/buckets/100/card_tables/steps/200/completions.json")
    stub_request(:delete, "https://3.basecampapi.com/12345/buckets/100/card_tables/steps/200/completions.json")
      .to_return(status: 200, body: uncompleted_step.to_json, headers: { "Content-Type" => "application/json" })

    step = @account.card_steps.uncomplete(project_id: 100, step_id: 200)

    assert_not step["completed"]
  end

  def test_reposition_step
    stub_post("/12345/buckets/100/card_tables/cards/200/positions.json", response_body: {})

    result = @account.card_steps.reposition(
      project_id: 100,
      card_id: 200,
      step_id: 300,
      position: 2
    )

    assert_nil result
  end
end
