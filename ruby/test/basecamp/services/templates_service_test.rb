# frozen_string_literal: true

require "test_helper"

class TemplatesServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list
    response = [ { "id" => 1, "name" => "Project Template" } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/templates\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.list.to_a
    assert_kind_of Array, result
    assert_equal "Project Template", result.first["name"]
  end

  def test_get
    response = { "id" => 1, "name" => "Project Template" }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/templates/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.get(template_id: 1)
    assert_equal "Project Template", result["name"]
  end

  def test_create
    response = { "id" => 1, "name" => "New Template" }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/templates\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.create(name: "New Template")
    assert_equal "New Template", result["name"]
  end

  def test_update
    response = { "id" => 1, "name" => "Updated Template" }

    stub_request(:put, %r{https://3\.basecampapi\.com/12345/templates/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.update(template_id: 1, name: "Updated Template")
    assert_equal "Updated Template", result["name"]
  end

  def test_delete
    stub_request(:delete, %r{https://3\.basecampapi\.com/12345/templates/\d+})
      .to_return(status: 204)

    result = @account.templates.delete(template_id: 1)
    assert_nil result
  end

  def test_create_project
    response = { "id" => 1, "status" => "processing" }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/templates/\d+/project_constructions\.json})
      .with(body: { project: { name: "Q1 Project" } })
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.create_project(template_id: 1, project: { name: "Q1 Project" })
    assert_equal "processing", result["status"]
  end

  def test_create_project_with_start_date
    response = { "id" => 598194962, "status" => "pending" }

    stub_request(:post, "https://3.basecampapi.com/12345/templates/2085958507/project_constructions.json")
      .with(body: { project: { name: "Marketing Campaign", description: "For Client: Xyz Corp Conference", start_date: "2026-09-01" } })
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.create_project(
      template_id: 2085958507,
      project: { name: "Marketing Campaign", description: "For Client: Xyz Corp Conference", start_date: "2026-09-01" }
    )
    assert_equal "pending", result["status"]
  end

  def test_get_construction
    response = { "id" => 1, "status" => "completed", "project" => { "id" => 100 } }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/templates/\d+/project_constructions/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.get_construction(template_id: 1, construction_id: 1)
    assert_equal "completed", result["status"]
  end

  def test_get_library_todolists
    response = {
      "bucket" => { "id" => 1, "name" => "To-do List Templates", "type" => "TemplateLibrary" },
      "todoset" => { "id" => 2, "title" => "To-do List Templates", "type" => "Todoset" },
      "todolists" => [ { "id" => 3, "name" => "Project kickoff" } ]
    }

    stub_request(:get, "https://3.basecampapi.com/12345/template_library/todolists.json")
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.get_library_todolists
    assert_equal "TemplateLibrary", result.dig("bucket", "type")
    assert_equal "Project kickoff", result.dig("todolists", 0, "name")
  end

  def test_get_library_card_tables
    response = {
      "bucket" => { "id" => 1, "name" => "To-do List Templates", "type" => "TemplateLibrary" },
      "kanban_boardset" => { "id" => 2, "title" => "Card Table Templates", "type" => "Kanban::Boardset" },
      "card_tables" => [ { "id" => 3, "title" => "Client onboarding", "type" => "Kanban::Board" } ]
    }

    stub_request(:get, "https://3.basecampapi.com/12345/template_library/card_tables.json")
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.get_library_card_tables
    assert_equal 2, result.dig("kanban_boardset", "id")
    assert_equal "Client onboarding", result.dig("card_tables", 0, "title")
  end

  def test_get_library_card_tables_without_a_container
    response = {
      "bucket" => { "id" => 1, "name" => "To-do List Templates", "type" => "TemplateLibrary" },
      "kanban_boardset" => nil,
      "card_tables" => []
    }

    stub_request(:get, "https://3.basecampapi.com/12345/template_library/card_tables.json")
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.get_library_card_tables
    assert_nil result["kanban_boardset"]
    assert_empty result["card_tables"]
  end

  def test_create_library_card_table
    response = {
      "id" => 3, "title" => "Client onboarding", "type" => "Kanban::Board",
      "parent" => { "id" => 2, "title" => "Card Table Templates", "type" => "Kanban::Boardset" }
    }

    stub_request(:post, "https://3.basecampapi.com/12345/template_library/card_tables.json")
      .with(body: { name: "Client onboarding" }.to_json)
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.create_library_card_table(name: "Client onboarding")
    assert_equal "Client onboarding", result["title"]
    assert_equal 2, result.dig("parent", "id")
  end

  def test_create_library_todolist
    response = {
      "id" => 3, "name" => "Project kickoff", "type" => "Todolist",
      "description" => "<div>Everything to open a project</div>"
    }

    stub_request(:post, "https://3.basecampapi.com/12345/template_library/todolists.json")
      .with(body: { name: "Project kickoff", description: "<div>Everything to open a project</div>" }.to_json)
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.create_library_todolist(
      name: "Project kickoff",
      description: "<div>Everything to open a project</div>"
    )
    assert_equal 3, result["id"]
    assert_equal "Project kickoff", result["name"]
  end

  def test_create_library_todolist_raises_validation_error
    stub_request(:post, "https://3.basecampapi.com/12345/template_library/todolists.json")
      .to_return(status: 422, body: { error: "Name can't be blank" }.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::ValidationError) { @account.templates.create_library_todolist(name: "Project kickoff") }
    assert_equal 422, error.http_status
  end

  def test_get_library_card_tables_raises_forbidden_error
    stub_request(:get, "https://3.basecampapi.com/12345/template_library/card_tables.json")
      .to_return(status: 403, body: { error: "Forbidden" }.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::ForbiddenError) { @account.templates.get_library_card_tables }
    assert_equal 403, error.http_status
  end

  def test_get_library_todolists_raises_forbidden_error
    stub_request(:get, "https://3.basecampapi.com/12345/template_library/todolists.json")
      .to_return(status: 403, body: { error: "Forbidden" }.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::ForbiddenError) { @account.templates.get_library_todolists }
    assert_equal 403, error.http_status
  end

  def test_create_library_copy
    response = {
      "id" => 5,
      "status" => "pending",
      "source_recording_id" => 3,
      "destination_parent_id" => 9,
      "url" => "https://3.basecampapi.com/12345/template_library/copies/5.json"
    }

    stub_request(:post, "https://3.basecampapi.com/12345/template_library/copies.json")
      .with(body: {
        template_recording_id: 3,
        destination_parent_id: 9,
        adding_people_confirmed: true
      })
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.create_library_copy(
      template_recording_id: 3,
      destination_parent_id: 9,
      adding_people_confirmed: true
    )
    assert_equal "pending", result["status"]
    assert_not result.key?("destination_todolist")
  end

  def test_get_completed_library_copy
    response = {
      "id" => 5,
      "status" => "completed",
      "source_recording_id" => 3,
      "destination_parent_id" => 9,
      "url" => "https://3.basecampapi.com/12345/template_library/copies/5.json",
      "destination_todolist" => { "id" => 10, "name" => "Project kickoff" }
    }

    stub_request(:get, "https://3.basecampapi.com/12345/template_library/copies/5")
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.get_library_copy(copy_id: 5)
    assert_equal "completed", result["status"]
    assert_equal 10, result.dig("destination_todolist", "id")
  end

  def test_get_library_copy_raises_not_found_error
    stub_request(:get, "https://3.basecampapi.com/12345/template_library/copies/404")
      .to_return(status: 404, body: { error: "Not found" }.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::NotFoundError) { @account.templates.get_library_copy(copy_id: 404) }
    assert_equal 404, error.http_status
  end

  def test_create_library_copy_requires_people_confirmation
    response = {
      "error" => "Adding people requires confirmation",
      "people" => [ { "id" => 4, "name" => "Victor", "avatar_url" => "https://example.test/avatar.png" } ]
    }

    stub_request(:post, "https://3.basecampapi.com/12345/template_library/copies.json")
      .with(body: { template_recording_id: 3, destination_parent_id: 9 })
      .to_return(status: 422, body: response.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::PeopleConfirmationRequiredError) do
      @account.templates.create_library_copy(template_recording_id: 3, destination_parent_id: 9)
    end
    assert_equal 422, error.http_status
    assert_equal "Adding people requires confirmation", error.message
    assert_equal 1, error.people.length
    assert_equal 4, error.people.first.id
    assert_equal "Victor", error.people.first.name
  end

  def test_create_templatification_sends_no_optional_keys_when_unset
    response = {
      "id" => 7,
      "status" => "pending",
      "source_recording_id" => 2,
      "url" => "https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications/7.json"
    }

    stub_request(:post, "https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications.json")
      .with { |request| JSON.parse(request.body.to_s.empty? ? "{}" : request.body) == {} }
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.create_templatification(bucket_id: 1, recording_id: 2)
    assert_equal "pending", result["status"]
    assert_equal 2, result["source_recording_id"]
    assert_not result.key?("destination_parent_id")
  end

  def test_create_templatification_sends_the_optional_save_settings
    response = {
      "id" => 7,
      "status" => "pending",
      "source_recording_id" => 2,
      "url" => "https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications/7.json"
    }

    stub_request(:post, "https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications.json")
      .with(body: {
        template_name: "Client onboarding",
        copy_comments: true,
        copy_assignments: false,
        move_cards_to_triage: true
      }.to_json)
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.create_templatification(
      bucket_id: 1,
      recording_id: 2,
      template_name: "Client onboarding",
      copy_comments: true,
      copy_assignments: false,
      move_cards_to_triage: true
    )
    assert_equal 7, result["id"]
  end

  def test_create_templatification_raises_forbidden_error
    stub_request(:post, "https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications.json")
      .to_return(status: 403, body: { error: "Forbidden" }.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::ForbiddenError) do
      @account.templates.create_templatification(bucket_id: 1, recording_id: 2)
    end
    assert_equal 403, error.http_status
  end

  def test_get_completed_templatification_with_destination_todolist
    response = {
      "id" => 7,
      "status" => "completed",
      "source_recording_id" => 2,
      "url" => "https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications/7.json",
      "destination_todolist" => { "id" => 10, "name" => "Project kickoff" }
    }

    stub_request(:get, "https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications/7")
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.get_templatification(bucket_id: 1, recording_id: 2, templatification_id: 7)
    assert_equal "completed", result["status"]
    assert_equal 10, result.dig("destination_todolist", "id")
    assert_not result.key?("destination_card_table")
    assert_not result.key?("destination_parent_id")
  end

  def test_get_completed_templatification_with_destination_card_table
    response = {
      "id" => 8,
      "status" => "completed",
      "source_recording_id" => 2,
      "url" => "https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications/8.json",
      "destination_card_table" => { "id" => 11, "title" => "Client onboarding", "type" => "Kanban::Board" }
    }

    stub_request(:get, "https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications/8")
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.get_templatification(bucket_id: 1, recording_id: 2, templatification_id: 8)
    assert_equal 11, result.dig("destination_card_table", "id")
    assert_not result.key?("destination_todolist")
  end

  def test_get_templatification_raises_not_found_error
    stub_request(:get, "https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications/404")
      .to_return(status: 404, body: { error: "Not found" }.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::NotFoundError) do
      @account.templates.get_templatification(bucket_id: 1, recording_id: 2, templatification_id: 404)
    end
    assert_equal 404, error.http_status
  end
end
