# frozen_string_literal: true

require "test_helper"

class TodosetsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_get
    response = {
      "id" => 1,
      "name" => "To-dos",
      "todolists_count" => 5,
      "completed_ratio" => "25/100"
    }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todosets/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todosets.get(todoset_id: 2)
    assert_equal "To-dos", result["name"]
    assert_equal 5, result["todolists_count"]
  end
end
