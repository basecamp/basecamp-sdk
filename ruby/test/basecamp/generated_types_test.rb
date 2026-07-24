# frozen_string_literal: true

require "test_helper"

class GeneratedTypesTest < Minitest::Test
  def test_project_type_parses_data
    data = {
      "id" => 12_345,
      "name" => "Test Project",
      "description" => "A test project",
      "status" => "active",
      "created_at" => "2024-01-01T00:00:00Z",
      "updated_at" => "2024-01-15T12:30:00Z"
    }

    project = Basecamp::Types::Project.new(data)

    assert_equal 12_345, project.id
    assert_equal "Test Project", project.name
    assert_equal "A test project", project.description
    assert_equal "active", project.status
    assert_instance_of Time, project.created_at
    assert_instance_of Time, project.updated_at
  end

  def test_project_type_to_h
    data = {
      "id" => 123,
      "name" => "My Project",
      "status" => "active"
    }

    project = Basecamp::Types::Project.new(data)
    hash = project.to_h

    assert_equal 123, hash["id"]
    assert_equal "My Project", hash["name"]
    assert_equal "active", hash["status"]
  end

  def test_project_type_to_json
    data = { "id" => 123, "name" => "JSON Project" }

    project = Basecamp::Types::Project.new(data)
    json = project.to_json

    parsed = JSON.parse(json)
    assert_equal 123, parsed["id"]
    assert_equal "JSON Project", parsed["name"]
  end

  def test_person_type_parses_data
    data = {
      "id" => 999,
      "name" => "John Doe",
      "email_address" => "john@example.com",
      "admin" => true,
      "owner" => false
    }

    person = Basecamp::Types::Person.new(data)

    assert_equal 999, person.id
    assert_equal "John Doe", person.name
    assert_equal "john@example.com", person.email_address
    assert_equal true, person.admin
    assert_equal false, person.owner
  end

  def test_todo_type_parses_data
    data = {
      "id" => 456,
      "content" => "Buy groceries",
      "completed" => false,
      "due_on" => "2024-02-01",
      "created_at" => "2024-01-01T00:00:00Z"
    }

    todo = Basecamp::Types::Todo.new(data)

    assert_equal 456, todo.id
    assert_equal "Buy groceries", todo.content
    assert_equal false, todo.completed
    assert_equal "2024-02-01", todo.due_on
    assert_instance_of Time, todo.created_at
  end

  def test_type_handles_nil_values
    data = { "id" => 123 }

    project = Basecamp::Types::Project.new(data)

    assert_equal 123, project.id
    assert_nil project.name
    assert_nil project.description
  end

  def test_type_handles_empty_data
    project = Basecamp::Types::Project.new({})

    assert_nil project.id
    assert_nil project.name
  end

  def test_type_compacts_nil_in_to_h
    data = { "id" => 123, "name" => nil }

    project = Basecamp::Types::Project.new(data)
    hash = project.to_h

    assert_equal 123, hash["id"]
    assert_not hash.key?("name")
  end

  # SearchType.key is required-and-nullable: the default metadata option sends
  # `{"key": null}`, and to_h must preserve that explicit null rather than
  # compacting it away, so consumers can distinguish the default from a real key.
  def test_search_type_preserves_null_key
    default_option = Basecamp::Types::SearchType.new("key" => nil, "value" => "Everything")
    hash = default_option.to_h

    assert hash.key?("key"), "required-nullable key must stay present"
    assert_nil hash["key"]
    assert_equal "Everything", hash["value"]

    real_option = Basecamp::Types::SearchType.new("key" => "Message", "value" => "Messages")
    assert_equal "Message", real_option.to_h["key"]
  end

  # Wormhole.color and Wormhole.destination_url are required-and-nullable: the bc3
  # jbuilder always emits them, null when unset/unlinked. to_h must preserve those
  # explicit nulls (the destination_url is the only field identifying the target),
  # not compact them away. Guards against a stale regeneration of the Wormhole block.
  def test_wormhole_preserves_null_color_and_destination_url
    unlinked = Basecamp::Types::Wormhole.new("id" => 1, "linked" => false, "color" => nil, "destination_url" => nil)
    hash = unlinked.to_h

    assert hash.key?("color"), "required-nullable color must stay present"
    assert_nil hash["color"]
    assert hash.key?("destination_url"), "required-nullable destination_url must stay present"
    assert_nil hash["destination_url"]

    linked = Basecamp::Types::Wormhole.new("id" => 2, "linked" => true, "color" => "#f5d76e", "destination_url" => "https://example.com/col.json")
    assert_equal "#f5d76e", linked.to_h["color"]
    assert_equal "https://example.com/col.json", linked.to_h["destination_url"]
  end
end
