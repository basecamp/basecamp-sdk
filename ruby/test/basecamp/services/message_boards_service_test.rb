# frozen_string_literal: true

require "test_helper"

class MessageBoardsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_get
    board = {
      "id" => 456,
      "title" => "Message Board",
      "messages_count" => 10,
      "created_at" => "2024-01-01T00:00:00Z"
    }
    stub_get("/12345/buckets/100/message_boards/456", response_body: board)

    result = @account.message_boards.get(project_id: 100, board_id: 456)

    assert_equal 456, result["id"]
    assert_equal "Message Board", result["title"]
    assert_equal 10, result["messages_count"]
  end
end
