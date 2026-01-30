# frozen_string_literal: true

# Tests for the CardTablesService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Single-resource paths without .json (get)

require "test_helper"

class CardTablesServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_card_table(id: 1, title: "Project Board")
    {
      "id" => id,
      "title" => title,
      "lists" => [
        { "id" => 100, "title" => "To Do", "cards_count" => 3 },
        { "id" => 101, "title" => "In Progress", "cards_count" => 2 },
        { "id" => 102, "title" => "Done", "cards_count" => 5 }
      ]
    }
  end

  def test_get_card_table
    # Generated service: /card_tables/{id} without .json
    stub_get("/12345/buckets/100/card_tables/200", response_body: sample_card_table(id: 200))

    table = @account.card_tables.get(project_id: 100, card_table_id: 200)

    assert_equal 200, table["id"]
    assert_equal "Project Board", table["title"]
    assert_equal 3, table["lists"].length
    assert_equal "To Do", table["lists"][0]["title"]
  end
end
