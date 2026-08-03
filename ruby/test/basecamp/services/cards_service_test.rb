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

  def sample_card(id: nil, title: nil)
    fixture = load_fixture("cards/get.json")
    fixture.merge "id" => id || fixture["id"], "title" => title || fixture["title"]
  end

  def test_list_cards
    stub_get("/12345/card_tables/lists/200/cards.json",
             response_body: [ sample_card, sample_card(id: 2, title: "Another Card") ])

    cards = @account.cards.list(column_id: 200).to_a

    assert_equal 2, cards.length
    assert_equal "Implement user authentication", cards[0]["title"]
    assert_equal "Another Card", cards[1]["title"]
  end

  def test_get_card
    # Generated service: /card_tables/cards/{id} without .json
    stub_get("/12345/card_tables/cards/200", response_body: sample_card(id: 200))

    card = @account.cards.get(card_id: 200)

    assert_equal 200, card["id"]
    assert_equal "Implement user authentication", card["title"]
  end

  def test_create_card
    new_card = sample_card(id: 999, title: "New Feature")
    stub_post("/12345/card_tables/lists/200/cards.json", response_body: new_card)

    card = @account.cards.create(
      column_id: 200,
      title: "New Feature",
      content: "<p>Feature description</p>",
      due_on: "2024-12-31"
    )

    assert_equal 999, card["id"]
    assert_equal "New Feature", card["title"]
  end

  def test_update_verbatim_card
    # Generated service: /card_tables/cards/{id} without .json. The raw path
    # sends exactly one PUT with no read-before-write.
    updated_card = sample_card(id: 200, title: "Updated Title")
    stub_put("/12345/card_tables/cards/200", response_body: updated_card)

    card = @account.cards.update_verbatim(
      card_id: 200,
      title: "Updated Title",
      content: "<p>New content</p>"
    )

    assert_equal "Updated Title", card["title"]
  end

  def test_update_card_preserves_due_on
    # BC3 merges the body over `{ due_on: nil }`, so a sparse PUT erases the
    # due date. The merge-safe update reads first and resends it.
    current = sample_card(id: 200, title: "Old Title").merge("due_on" => "2024-02-01")
    updated_card = sample_card(id: 200, title: "Updated Title")
    stub_get("/12345/card_tables/cards/200", response_body: current)
    stub_put("/12345/card_tables/cards/200", response_body: updated_card)

    card = @account.cards.update(card_id: 200, title: "Updated Title")

    assert_equal "Updated Title", card["title"]
    assert_requested(:put, "#{BASE_URL}/12345/card_tables/cards/200") do |req|
      body = JSON.parse(req.body)
      body["due_on"] == "2024-02-01" &&
        body["title"] == "Updated Title" &&
        # Never echoed back: BC3 filters assignee ids through reachable_people.
        !body.key?("assignee_ids")
    end
  end

  def test_update_card_clears_due_on_by_omission
    updated_card = sample_card(id: 200, title: "Updated Title")
    stub_put("/12345/card_tables/cards/200", response_body: updated_card)

    @account.cards.update(card_id: 200, due_on: "")

    # Clearing is omission, never `{"due_on": null}` (SPEC section 18).
    assert_requested(:put, "#{BASE_URL}/12345/card_tables/cards/200") do |req|
      !JSON.parse(req.body).key?("due_on")
    end
  end

  def test_move_card
    stub_post("/12345/card_tables/cards/200/moves.json", response_body: {})

    result = @account.cards.move(card_id: 200, column_id: 300)

    assert_nil result
  end

  # --- #576: a malformed GET due_on must never reach the replacement PUT -----
  #
  # `update` reads the card only to resend its due date, so that one value is
  # the whole reason the composite exists. `compact_params` is `kwargs.compact`,
  # which removes only nil, so before the guard `false`, `0`, `[]`, `{}`, `42`,
  # `true` and `["x"]` all reached the wire and were written to the card. Ruby
  # has no typed decoder between the GET and the read: the generated `get`
  # returns a raw Hash.
  #
  # The assertion that matters is the ORDERING -- assert_not_requested :put.
  [ false, 0, [], {}, 42, true, [ "x" ], { "a" => 1 } ].each do |malformed|
    define_method("test_update_refuses_a_malformed_due_on_#{malformed.inspect}") do
      stub_get("/12345/card_tables/cards/200", response_body: sample_card(id: 200).merge("due_on" => malformed))
      stub_put("/12345/card_tables/cards/200", response_body: sample_card(id: 200))

      error = assert_raises(Basecamp::ApiError) do
        @account.cards.update(card_id: 200, title: "Updated Title")
      end

      assert_includes error.message, 'Card field "due_on" is not a string'
      # api_error, not usage: the value arrived in a successful response.
      assert_equal Basecamp::ErrorCode::API, error.code
      assert_requested :get, "#{BASE_URL}/12345/card_tables/cards/200", times: 1
      assert_not_requested :put, "#{BASE_URL}/12345/card_tables/cards/200"
    end
  end

  # The other half of the rule: a card with no due date is not malformed.
  [ [ "missing", :delete ], [ "nil", nil ] ].each do |label, mode|
    define_method("test_#{label}_due_on_stays_genuinely_empty") do
      current = sample_card(id: 200)
      mode == :delete ? current.delete("due_on") : current["due_on"] = nil
      stub_get("/12345/card_tables/cards/200", response_body: current)
      stub_put("/12345/card_tables/cards/200", response_body: sample_card(id: 200))

      @account.cards.update(card_id: 200, title: "Updated Title")

      assert_requested(:put, "#{BASE_URL}/12345/card_tables/cards/200") do |req|
        !JSON.parse(req.body).key?("due_on")
      end
    end
  end

  # One level up from the field guard: a successful GET can return a scalar, an
  # Array or nil, and `card["due_on"]` would raise a raw TypeError instead of
  # the documented statusless api_error.
  [ 42, "nope", nil, [ "a" ], true ].each do |body|
    define_method("test_update_refuses_a_non_object_response_body_#{body.inspect}") do
      stub_get("/12345/card_tables/cards/200", response_body: body.to_json)
      stub_put("/12345/card_tables/cards/200", response_body: sample_card(id: 200))

      error = assert_raises(Basecamp::ApiError) do
        @account.cards.update(card_id: 200, title: "Updated Title")
      end

      assert_includes error.message, "GetCard returned"
      assert_not_requested :put, "#{BASE_URL}/12345/card_tables/cards/200"
    end
  end
end
