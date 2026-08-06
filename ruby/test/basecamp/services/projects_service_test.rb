# frozen_string_literal: true

# Tests for the ProjectsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Single-resource paths without .json (get, update, trash)
# - Uses keyword argument project_id: instead of positional

require "test_helper"

class ProjectsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_project(id: 123, name: "Test Project")
    {
      "id" => id,
      "name" => name,
      "description" => "A test project",
      "status" => "active",
      "start_date" => "2024-01-01",
      "end_date" => "2024-03-31"
    }
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
    # Generated service: /projects/{id} without .json
    stub_get("/12345/projects/123", response_body: sample_project)

    project = @account.projects.get(project_id: 123)

    assert_equal 123, project["id"]
    assert_equal "Test Project", project["name"]
    assert_equal "2024-01-01", project["start_date"]
    assert_equal "2024-03-31", project["end_date"]
  end

  def test_create_project
    stub_post("/12345/projects.json", response_body: sample_project(id: 999, name: "New Project"))

    project = @account.projects.create(name: "New Project", description: "A description")

    assert_equal 999, project["id"]
    assert_equal "New Project", project["name"]
  end

  def test_update_project
    # Generated service: /projects/{id} without .json
    stub_put("/12345/projects/123", response_body: sample_project(name: "Updated Name"))

    project = @account.projects.update(project_id: 123, name: "Updated Name")

    assert_equal "Updated Name", project["name"]
  end

  def test_trash_project
    # Generated service: /projects/{id} without .json
    stub_delete("/12345/projects/123")

    result = @account.projects.trash(project_id: 123)

    assert_nil result
  end

  def test_archive_project
    stub_put("/12345/projects/123/status/archived.json", response_body: "", status: 204)

    result = @account.projects.archive(project_id: 123)

    assert_nil result
  end

  def test_unarchive_project
    stub_put("/12345/projects/123/status/active.json", response_body: "", status: 204)

    result = @account.projects.unarchive(project_id: 123)

    assert_nil result
  end

  # The admin pro pack can limit archiving to admins and the project's creator,
  # which bc3 answers with `head :forbidden`.
  def test_archive_project_forbidden
    stub_put("/12345/projects/123/status/archived.json", response_body: "", status: 403)

    error = assert_raises(Basecamp::ForbiddenError) do
      @account.projects.archive(project_id: 123)
    end

    assert_equal 403, error.http_status
  end

  # The only behavioural evidence for ProjectLimitError. A 507 is an account
  # limit, so it maps to limit_exceeded and is NOT retryable — no backoff frees
  # a project slot (SPEC.md §6, step 11).
  def test_unarchive_project_at_project_limit
    stub_put(
      "/12345/projects/123/status/active.json",
      response_body: { error: "The project limit for this account has been reached." },
      status: 507
    )

    error = assert_raises(Basecamp::LimitExceededError) do
      @account.projects.unarchive(project_id: 123)
    end

    assert_equal 507, error.http_status
    assert_equal Basecamp::ErrorCode::LIMIT_EXCEEDED, error.code
    assert_not error.retryable, "an account limit is never retryable"
  end
end
