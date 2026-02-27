# frozen_string_literal: true

require "test_helper"

class MessageTypesServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list
    types = [
      { "id" => 1, "name" => "Announcement", "icon" => "\u{1F4E2}" },
      { "id" => 2, "name" => "Update", "icon" => "\u{1F4DD}" }
    ]
    stub_get("/12345/categories.json", response_body: types)

    result = @account.message_types.list.to_a

    assert_equal 2, result.length
    assert_equal "Announcement", result[0]["name"]
    assert_equal "Update", result[1]["name"]
  end

  def test_get
    type = { "id" => 1, "name" => "Announcement", "icon" => "\u{1F4E2}" }
    stub_get("/12345/categories/1", response_body: type)

    result = @account.message_types.get(type_id: 1)

    assert_equal 1, result["id"]
    assert_equal "Announcement", result["name"]
  end

  def test_create
    type = { "id" => 3, "name" => "Question", "icon" => "\u{2753}" }
    stub_post("/12345/categories.json", response_body: type)

    result = @account.message_types.create(name: "Question", icon: "\u{2753}")

    assert_equal 3, result["id"]
    assert_equal "Question", result["name"]
  end

  def test_update
    updated_type = { "id" => 1, "name" => "Important Announcement", "icon" => "\u{1F4E3}" }
    stub_put("/12345/categories/1", response_body: updated_type)

    result = @account.message_types.update(type_id: 1, name: "Important Announcement")

    assert_equal "Important Announcement", result["name"]
  end

  def test_delete
    stub_delete("/12345/categories/1")

    result = @account.message_types.delete(type_id: 1)

    assert_nil result
  end
end
