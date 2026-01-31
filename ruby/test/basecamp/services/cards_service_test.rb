# frozen_string_literal: true

# Tests for the CardsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Single-resource paths without .json (get, update)

require "test_helper"

class CardsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_card(id: 1, title: "Task Card")
    {
      "id" => id,
      "title" => title,
      "content" => "<p>Card description</p>",
      "due_on" => "2024-12-31",
      "completed" => false,
      "assignees" => []
    }
  end

  def test_list_cards
    stub_get("/12345/buckets/100/card_tables/lists/200/cards.json",
             response_body: [ sample_card, sample_card(id: 2, title: "Another Card") ])

    cards = @account.cards.list(project_id: 100, column_id: 200).to_a

    assert_equal 2, cards.length
    assert_equal "Task Card", cards[0]["title"]
    assert_equal "Another Card", cards[1]["title"]
  end

  def test_get_card
    # Generated service: /card_tables/cards/{id} without .json
    stub_get("/12345/buckets/100/card_tables/cards/200", response_body: sample_card(id: 200))

    card = @account.cards.get(project_id: 100, card_id: 200)

    assert_equal 200, card["id"]
    assert_equal "Task Card", card["title"]
  end

  def test_create_card
    new_card = sample_card(id: 999, title: "New Feature")
    stub_post("/12345/buckets/100/card_tables/lists/200/cards.json", response_body: new_card)

    card = @account.cards.create(
      project_id: 100,
      column_id: 200,
      title: "New Feature",
      content: "<p>Feature description</p>",
      due_on: "2024-12-31"
    )

    assert_equal 999, card["id"]
    assert_equal "New Feature", card["title"]
  end

  def test_update_card
    # Generated service: /card_tables/cards/{id} without .json
    updated_card = sample_card(id: 200, title: "Updated Title")
    stub_put("/12345/buckets/100/card_tables/cards/200", response_body: updated_card)

    card = @account.cards.update(
      project_id: 100,
      card_id: 200,
      title: "Updated Title",
      content: "<p>New content</p>"
    )

    assert_equal "Updated Title", card["title"]
  end

  def test_move_card
    stub_post("/12345/buckets/100/card_tables/cards/200/moves.json", response_body: {})

    result = @account.cards.move(project_id: 100, card_id: 200, column_id: 300)

    assert_nil result
  end
end
