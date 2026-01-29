# frozen_string_literal: true

require "test_helper"

class ProjectsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list_projects
    stub_get("/12345/projects.json", response_body: [ sample_project, sample_project(id: 456, name: "Other") ])

    projects = @account.projects.list.to_a

    assert_equal 2, projects.length
    assert_equal "Test Project", projects[0]["name"]
    assert_equal "Other", projects[1]["name"]
  end

  def test_list_projects_with_status_filter
    stub_request(:get, "https://3.basecampapi.com/12345/projects.json")
      .with(query: { status: "archived" })
      .to_return(status: 200, body: [ sample_project ].to_json)

    projects = @account.projects.list(status: "archived").to_a

    assert_equal 1, projects.length
  end

  def test_get_project
    stub_get("/12345/projects/123.json", response_body: sample_project)

    project = @account.projects.get(123)

    assert_equal 123, project["id"]
    assert_equal "Test Project", project["name"]
  end

  def test_create_project
    stub_post("/12345/projects.json", response_body: sample_project(id: 999, name: "New Project"))

    project = @account.projects.create(name: "New Project", description: "A description")

    assert_equal 999, project["id"]
    assert_equal "New Project", project["name"]
  end

  def test_update_project
    stub_put("/12345/projects/123.json", response_body: sample_project(name: "Updated Name"))

    project = @account.projects.update(123, name: "Updated Name")

    assert_equal "Updated Name", project["name"]
  end

  def test_trash_project
    stub_delete("/12345/projects/123.json")

    result = @account.projects.trash(123)

    assert_nil result
  end
end
