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

  # The typed decode (Basecamp::Types::Comment → RichTextAttachment) carries the
  # rich-text content's inline files. Pixel dimensions arrive float-spelled
  # (1024.0) for images and null for non-image blobs; parse_integer decodes
  # both faithfully — 1024.0 → 1024 (to_i) and null → nil. This is a
  # decode-only assertion: re-encoding a nil dimension is out of scope here,
  # since to_h calls .compact and drops the nil key (an SDK-wide encoder
  # behavior documented in SPEC.md §10 Type Fidelity).
  def test_content_attachment_dimensions_decode
    comment = Basecamp::Types::Comment.new(
      "id" => 456,
      "content" => "<p>Great work!</p>",
      "content_attachments" => [
        {
          "id" => 1_069_480_010, "sgid" => "BAh-img", "filename" => "celebration.png",
          "content_type" => "image/png", "byte_size" => 284_111,
          "download_url" => "https://3.basecampapi.com/12345/buckets/1/blobs/img/download/celebration.png",
          "width" => 1024.0, "height" => 768, "previewable" => true,
          "preview_url" => "https://3.basecampapi.com/12345/buckets/1/blobs/img/previews/celebration.png",
          "thumbnail_url" => "https://3.basecampapi.com/12345/buckets/1/blobs/img/thumbnails/celebration.png"
        },
        {
          "id" => 1_069_480_011, "sgid" => "BAh-pdf", "filename" => "notes.pdf",
          "content_type" => "application/pdf", "byte_size" => 1_048_576,
          "download_url" => "https://3.basecampapi.com/12345/buckets/1/blobs/pdf/download/notes.pdf",
          "width" => nil, "height" => nil, "previewable" => false,
          "preview_url" => "https://3.basecampapi.com/12345/buckets/1/blobs/pdf/previews/notes.pdf",
          "thumbnail_url" => "https://3.basecampapi.com/12345/buckets/1/blobs/pdf/thumbnails/notes.pdf"
        }
      ]
    )

    image, pdf = comment.content_attachments

    # Float-spelled 1024.0 decodes to the integer 1024.
    assert_equal 1024, image.width
    assert_equal 768, image.height
    assert_equal "image/png", image.content_type

    # null dimensions decode to nil (not a sentinel 0).
    assert_nil pdf.width
    assert_nil pdf.height
  end
end
