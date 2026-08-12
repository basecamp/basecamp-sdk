# frozen_string_literal: true

require "test_helper"

class MyAssignmentsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_get_my_assignments_decodes_assignees
    # bc3's people/_person_minimal partial renders id, name and avatar_url
    # unconditionally, so an assignee always carries all three keys (#659).
    stub_get("/12345/my/assignments.json", response_body: {
      "priorities" => [
        { "id" => 1, "content" => "Priority task",
          "assignees" => [ { "id" => 1049715914, "name" => "Victor Cooper",
                             "avatar_url" => "https://example.com/avatar" } ] }
      ],
      "non_priorities" => []
    })

    result = @account.my_assignments.get_my_assignments

    assignee = result["priorities"][0]["assignees"][0]
    assert_equal 1049715914, assignee["id"]
    assert_equal "Victor Cooper", assignee["name"]
    assert_equal "https://example.com/avatar", assignee["avatar_url"]
  end

  def test_prioritize_assignment_posts_the_recording_id
    stub_request(:post, "https://3.basecampapi.com/12345/my/priorities.json")
      .with(body: { "id" => 1069479801 }.to_json)
      .to_return(status: 204, body: "")

    @account.my_assignments.prioritize_assignment(id: 1069479801)

    assert_requested(:post, "https://3.basecampapi.com/12345/my/priorities.json")
  end

  def test_prioritize_assignment_raises_not_found
    stub_request(:post, "https://3.basecampapi.com/12345/my/priorities.json")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) do
      @account.my_assignments.prioritize_assignment(id: 999)
    end
  end

  def test_deprioritize_assignment_deletes_the_exact_recording
    stub_request(:delete, "https://3.basecampapi.com/12345/my/priorities/1069479801")
      .to_return(status: 204, body: "")

    @account.my_assignments.deprioritize_assignment(recording_id: 1069479801)

    assert_requested(:delete, "https://3.basecampapi.com/12345/my/priorities/1069479801")
  end

  def test_deprioritize_assignment_raises_forbidden
    stub_request(:delete, "https://3.basecampapi.com/12345/my/priorities/1069479801")
      .to_return(status: 403, body: { "error" => "Forbidden" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::ForbiddenError) do
      @account.my_assignments.deprioritize_assignment(recording_id: 1069479801)
    end
  end

  def test_reorder_up_next_posts_source_and_position
    stub_request(:post, "https://3.basecampapi.com/12345/my/priority_moves.json")
      .with(body: { "source_id" => 1069479801, "position" => 1 }.to_json)
      .to_return(status: 204, body: "")

    @account.my_assignments.reorder_up_next(source_id: 1069479801, position: 1)

    assert_requested(:post, "https://3.basecampapi.com/12345/my/priority_moves.json")
  end

  def test_reorder_up_next_raises_on_typed_400
    stub_request(:post, "https://3.basecampapi.com/12345/my/priority_moves.json")
      .to_return(status: 400, body: { "error" => "Position must be an integer." }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::Error) do
      @account.my_assignments.reorder_up_next(source_id: 1069479801, position: 2)
    end
  end

  def test_reorder_up_next_raises_validation_error_on_typed_422
    stub_request(:post, "https://3.basecampapi.com/12345/my/priority_moves.json")
      .to_return(status: 422, body: { "error" => "Position must be between 1 and 3." }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::ValidationError) do
      @account.my_assignments.reorder_up_next(source_id: 1069479801, position: 99)
    end
  end

  def test_reorder_up_next_raises_not_found_on_bare_404
    stub_request(:post, "https://3.basecampapi.com/12345/my/priority_moves.json")
      .to_return(status: 404, body: "")

    assert_raises(Basecamp::NotFoundError) do
      @account.my_assignments.reorder_up_next(source_id: 999, position: 1)
    end
  end
end
