# frozen_string_literal: true

# Tests for the CommentsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Some paths without .json suffix (get, update)
# - No client-side validation (API validates)

require "test_helper"

class CommentsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_comment(id: 1, content: "<p>Great work!</p>")
    {
      "id" => id,
      "content" => content,
      "creator" => { "id" => 1, "name" => "Test User" },
      "created_at" => "2024-01-01T00:00:00Z"
    }
  end

  def test_list_comments
    stub_get("/12345/recordings/200/comments.json",
             response_body: [ sample_comment, sample_comment(id: 2, content: "<p>I agree!</p>") ])

    comments = @account.comments.list(recording_id: 200).to_a

    assert_equal 2, comments.length
    assert_equal "<p>Great work!</p>", comments[0]["content"]
    assert_equal "<p>I agree!</p>", comments[1]["content"]
  end

  def test_get_comment
    # Generated service: /comments/{id} without .json
    stub_get("/12345/comments/200", response_body: sample_comment(id: 200))

    comment = @account.comments.get(comment_id: 200)

    assert_equal 200, comment["id"]
    assert_equal "<p>Great work!</p>", comment["content"]
  end

  def test_create_comment
    new_comment = sample_comment(id: 999, content: "<p>New comment</p>")
    stub_post("/12345/recordings/200/comments.json", response_body: new_comment)

    comment = @account.comments.create(
      recording_id: 200,
      content: "<p>New comment</p>"
    )

    assert_equal 999, comment["id"]
    assert_equal "<p>New comment</p>", comment["content"]
  end

  def test_update_comment
    # Generated service: /comments/{id} without .json
    updated_comment = sample_comment(id: 200, content: "<p>Updated comment</p>")
    stub_put("/12345/comments/200", response_body: updated_comment)

    comment = @account.comments.update(
      comment_id: 200,
      content: "<p>Updated comment</p>"
    )

    assert_equal "<p>Updated comment</p>", comment["content"]
  end
end
