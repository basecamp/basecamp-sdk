# frozen_string_literal: true

require "test_helper"

class ToolsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_get
    response = { "id" => 1, "name" => "Message Board", "enabled" => true }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/dock/tools/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.tools.get(tool_id: 2)
    assert_equal "Message Board", result["name"]
    assert_equal true, result["enabled"]
  end

  def test_clone
    response = { "id" => 2, "name" => "Message Board (Copy)" }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/dock/tools\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.tools.clone(source_recording_id: 2)
    assert_equal "Message Board (Copy)", result["name"]
  end

  def test_update
    response = { "id" => 1, "title" => "Team Updates" }

    stub_request(:put, %r{https://3\.basecampapi\.com/12345/dock/tools/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.tools.update(tool_id: 2, title: "Team Updates")
    assert_equal "Team Updates", result["title"]
  end

  def test_delete
    stub_request(:delete, %r{https://3\.basecampapi\.com/12345/dock/tools/\d+})
      .to_return(status: 204)

    result = @account.tools.delete(tool_id: 2)
    assert_nil result
  end

  def test_enable
    stub_request(:post, %r{https://3\.basecampapi\.com/12345/recordings/\d+/position\.json})
      .to_return(status: 204)

    result = @account.tools.enable(tool_id: 2)
    assert_nil result
  end

  def test_disable
    stub_request(:delete, %r{https://3\.basecampapi\.com/12345/recordings/\d+/position\.json})
      .to_return(status: 204)

    result = @account.tools.disable(tool_id: 2)
    assert_nil result
  end

  def test_reposition
    stub_request(:put, %r{https://3\.basecampapi\.com/12345/recordings/\d+/position\.json})
      .to_return(status: 204)

    result = @account.tools.reposition(tool_id: 2, position: 1)
    assert_nil result
  end
end
