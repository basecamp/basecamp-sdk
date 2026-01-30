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
end
