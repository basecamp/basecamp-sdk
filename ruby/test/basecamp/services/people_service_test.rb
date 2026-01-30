# frozen_string_literal: true

# Tests for the PeopleService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - me() renamed to my_profile()
# - list_project_people() renamed to list_for_project()
# - list_assignable() added (was in ReportsService)

require "test_helper"

class PeopleServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_person(id: 1, name: "Test User")
    {
      "id" => id,
      "name" => name,
      "email_address" => "#{name.downcase.tr(" ", ".")}@example.com",
      "admin" => false
    }
  end

  def test_list
    people = [ sample_person, sample_person(id: 2, name: "Another User") ]
    stub_get("/12345/people.json", response_body: people)

    result = @account.people.list.to_a

    assert_equal 2, result.length
    assert_equal "Test User", result[0]["name"]
  end

  def test_get
    stub_get("/12345/people/1", response_body: sample_person)

    result = @account.people.get(person_id: 1)

    assert_equal 1, result["id"]
    assert_equal "Test User", result["name"]
  end

  def test_my_profile
    me = sample_person(id: 99, name: "Current User")
    stub_get("/12345/my/profile.json", response_body: me)

    result = @account.people.my_profile

    assert_equal 99, result["id"]
    assert_equal "Current User", result["name"]
  end

  def test_list_pingable
    people = [ sample_person, sample_person(id: 3, name: "Pingable User") ]
    stub_get("/12345/circles/people.json", response_body: people)

    result = @account.people.list_pingable.to_a

    assert_equal 2, result.length
  end

  def test_list_for_project
    people = [ sample_person(id: 5, name: "Project Member") ]
    stub_get("/12345/projects/100/people.json", response_body: people)

    result = @account.people.list_for_project(project_id: 100).to_a

    assert_equal 1, result.length
    assert_equal "Project Member", result[0]["name"]
  end

  def test_update_project_access
    stub_put("/12345/projects/100/people/users.json", response_body: {})

    result = @account.people.update_project_access(
      project_id: 100,
      grant: [ 1, 2 ],
      revoke: [ 3 ]
    )

    # Generated service returns parsed JSON (empty hash for empty response)
    assert_kind_of Hash, result
  end

  def test_list_assignable
    people = [
      sample_person(id: 1, name: "Jane Doe"),
      sample_person(id: 2, name: "John Smith")
    ]
    stub_get("/12345/reports/todos/assigned.json", response_body: people)

    result = @account.people.list_assignable

    assert_kind_of Array, result
    assert_equal 2, result.length
  end
end
