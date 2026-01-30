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
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.create_project(template_id: 1, name: "Q1 Project")
    assert_equal "processing", result["status"]
  end

  def test_get_construction
    response = { "id" => 1, "status" => "completed", "project" => { "id" => 100 } }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/templates/\d+/project_constructions/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.templates.get_construction(template_id: 1, construction_id: 1)
    assert_equal "completed", result["status"]
  end
end
