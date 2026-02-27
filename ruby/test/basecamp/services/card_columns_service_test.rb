# frozen_string_literal: true

# Tests for the CardColumnsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Single-resource paths without .json (get, update)

require "test_helper"

class CardColumnsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_column(id: 1, title: "To Do")
    {
      "id" => id,
      "title" => title,
      "description" => "Tasks to be done",
      "color" => "white",
      "cards_count" => 5
    }
  end

  def test_get_column
    # Generated service: /card_tables/columns/{id} without .json
    stub_get("/12345/card_tables/columns/200", response_body: sample_column(id: 200))

    column = @account.card_columns.get(column_id: 200)

    assert_equal 200, column["id"]
    assert_equal "To Do", column["title"]
  end

  def test_create_column
    new_column = sample_column(id: 999, title: "In Review")
    stub_post("/12345/card_tables/200/columns.json", response_body: new_column)

    column = @account.card_columns.create(
      card_table_id: 200,
      title: "In Review",
      description: "Waiting for review"
    )

    assert_equal 999, column["id"]
    assert_equal "In Review", column["title"]
  end

  def test_update_column
    # Generated service: /card_tables/columns/{id} without .json
    updated_column = sample_column(id: 200, title: "Updated Title")
    stub_put("/12345/card_tables/columns/200", response_body: updated_column)

    column = @account.card_columns.update(
      column_id: 200,
      title: "Updated Title",
      description: "New description"
    )

    assert_equal "Updated Title", column["title"]
  end

  def test_move_column
    stub_post("/12345/card_tables/200/moves.json", response_body: {})

    result = @account.card_columns.move(
      card_table_id: 200,
      source_id: 300,
      target_id: 400,
      position: 1
    )

    assert_nil result
  end

  def test_set_color
    colored_column = sample_column(id: 200)
    colored_column["color"] = "blue"
    stub_put("/12345/card_tables/columns/200/color.json", response_body: colored_column)

    column = @account.card_columns.set_color(column_id: 200, color: "blue")

    assert_equal "blue", column["color"]
  end

  def test_enable_on_hold
    column_with_hold = sample_column(id: 200)
    column_with_hold["on_hold"] = true
    stub_post("/12345/card_tables/columns/200/on_hold.json", response_body: column_with_hold)

    column = @account.card_columns.enable_on_hold(column_id: 200)

    assert column["on_hold"]
  end

  def test_disable_on_hold
    column_without_hold = sample_column(id: 200)
    column_without_hold["on_hold"] = false
    stub_delete("/12345/card_tables/columns/200/on_hold.json")
    stub_request(:delete, "https://3.basecampapi.com/12345/card_tables/columns/200/on_hold.json")
      .to_return(status: 200, body: column_without_hold.to_json, headers: { "Content-Type" => "application/json" })

    column = @account.card_columns.disable_on_hold(column_id: 200)

    assert_not column["on_hold"]
  end
end
