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

  def test_update_card_omits_an_unaddressed_due_on
    # BC3's card update is presence-aware on the JSON representation
    # (bc3#12521), so an unmentioned due_on is left alone by the server. The
    # SDK sends no key and does not read the card first -- one PUT, no GET.
    updated_card = sample_card(id: 200, title: "Updated Title")
    stub_put("/12345/card_tables/cards/200", response_body: updated_card)

    card = @account.cards.update(card_id: 200, title: "Updated Title")

    assert_equal "Updated Title", card["title"]
    assert_requested(:put, "#{BASE_URL}/12345/card_tables/cards/200", times: 1) do |req|
      body = JSON.parse(req.body)
      !body.key?("due_on") &&
        body["title"] == "Updated Title" &&
        !body.key?("assignee_ids")
    end
    assert_not_requested :get, "#{BASE_URL}/12345/card_tables/cards/200"
  end

  def test_update_card_clears_due_on_with_an_explicit_empty_string
    updated_card = sample_card(id: 200, title: "Updated Title")
    stub_put("/12345/card_tables/cards/200", response_body: updated_card)

    @account.cards.update(card_id: 200, due_on: "")

    # Presence-aware means an omitted due_on is UNCHANGED, so a clear has to be
    # STATED. `""` is the spelling that travels; `{"due_on": null}` cannot reach
    # the wire at all, because compact_params drops nils (SPEC section 18).
    assert_requested(:put, "#{BASE_URL}/12345/card_tables/cards/200", times: 1) do |req|
      body = JSON.parse(req.body)
      body.key?("due_on") && body["due_on"] == ""
    end
    assert_not_requested :get, "#{BASE_URL}/12345/card_tables/cards/200"
  end

  def test_update_card_sends_a_stated_due_date
    stub_put("/12345/card_tables/cards/200", response_body: sample_card(id: 200))

    @account.cards.update(card_id: 200, due_on: "2024-12-31")

    assert_requested(:put, "#{BASE_URL}/12345/card_tables/cards/200", times: 1) do |req|
      JSON.parse(req.body)["due_on"] == "2024-12-31"
    end
    assert_not_requested :get, "#{BASE_URL}/12345/card_tables/cards/200"
  end

  # --- bc3#12521: the clear has to CLEAR, not just look right on the wire ----
  #
  # Asserting the body's shape pins the request, not the outcome. The stub below
  # is a stateful model of the deployed controller: `card_update_params` is
  # plain `card_params` for JSON, so a key the body omits is never assigned, and
  # Rails blank-casts `""` (and null) on a date column to nil. Every request the
  # SDK makes is replayed through it and the stored card is asserted afterwards.
  def stub_presence_aware_card_server(stored)
    stub_request(:get, "#{BASE_URL}/12345/card_tables/cards/200").to_return do |_req|
      { status: 200, body: stored.to_json, headers: { "Content-Type" => "application/json" } }
    end

    stub_request(:put, "#{BASE_URL}/12345/card_tables/cards/200").to_return do |req|
      params = JSON.parse(req.body)
      # Presence-aware: only the keys actually carried are assigned.
      params.each { |key, value| stored[key] = value }
      stored["due_on"] = nil if params.key?("due_on") && params["due_on"].to_s.empty?
      { status: 200, body: stored.to_json, headers: { "Content-Type" => "application/json" } }
    end
  end

  def test_explicit_clear_actually_clears_the_stored_due_date
    stored = sample_card(id: 200).merge("due_on" => "2024-02-01")
    stub_presence_aware_card_server(stored)

    card = @account.cards.update(card_id: 200, due_on: "")

    assert_nil stored["due_on"], "the card kept its due date -- the clear was a no-op"
    assert_nil card["due_on"]
  end

  def test_unaddressed_due_on_survives_the_update
    # The other half, and the pin on the model itself: without it the clear
    # above would pass just as happily against a mock that wiped the date on
    # every PUT -- i.e. against the OLD server -- and would prove nothing.
    stored = sample_card(id: 200).merge("due_on" => "2024-02-01")
    stub_presence_aware_card_server(stored)

    card = @account.cards.update(card_id: 200, title: "Updated Title")

    assert_equal "2024-02-01", stored["due_on"]
    assert_equal "2024-02-01", card["due_on"]
    assert_equal "Updated Title", stored["title"]
    # No key was sent: the date survived because the server left it alone, not
    # because the SDK read it and echoed it back.
    assert_requested(:put, "#{BASE_URL}/12345/card_tables/cards/200", times: 1) do |req|
      !JSON.parse(req.body).key?("due_on")
    end
    assert_not_requested :get, "#{BASE_URL}/12345/card_tables/cards/200"
  end

  def test_move_card
    stub_post("/12345/card_tables/cards/200/moves.json", response_body: {})

    result = @account.cards.move(card_id: 200, column_id: 300)

    assert_nil result
  end
end
