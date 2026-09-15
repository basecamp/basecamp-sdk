# frozen_string_literal: true

require "test_helper"

# Tests for the mention-expanding comment writes (SPEC section 18, Appendix F).
#
# The conformance fixture pins the happy path — people read, then POST, with the
# tag inside the first block. What lives here is the trust boundary the fixture
# cannot reach: that the people read ALWAYS happens, that a failed read posts
# nothing, and that dedupe is on the exact sgid the read returned.
class CommentsMentionsTest < Minitest::Test
  include TestHelper

  RECORDING_ID = 1069479351

  def setup
    @account = create_account_client(account_id: "12345")
  end

  # The older Rails envelope, with a synthetic signature. Nothing verifies it.
  def person_sgid(id, signature: "abc123")
    gid = "gid://bc3/Person/#{id}"
    payload = "\x04\x08{\aI\"\bgid\x06:\x06ET" + marshal_string(gid) +
              "I\"\fpurpose\x06;\x00T" + marshal_string("attachable")
    "#{[ payload.b ].pack("m0").tr("+/", "-_").delete("=")}--#{signature}"
  end

  def marshal_string(value)
    bytes = value.b
    "I\"" + (bytes.bytesize + 5).chr + bytes + "\x06:\x06ET"
  end

  def stub_person(id, sgid: nil, status: 200)
    stub_request(:get, "#{BASE_URL}/12345/people/#{id}")
      .to_return(
        status: status,
        body: { "id" => id, "name" => "Person #{id}", "attachable_sgid" => sgid || person_sgid(id) }.to_json,
        headers: { "Content-Type" => "application/json" }
      )
  end

  def stub_comment_create
    stub_post("/12345/recordings/#{RECORDING_ID}/comments.json", response_body: { "id" => 1, "type" => "Comment" })
  end

  def test_expand_mentions_returns_content_unchanged_with_no_ids
    assert_equal "<div>hi</div>", @account.comments.expand_mentions(content: "<div>hi</div>", person_ids: nil)
    assert_equal "<div>hi</div>", @account.comments.expand_mentions(content: "<div>hi</div>", person_ids: [])
    assert_not_requested(:any, %r{\A#{BASE_URL}})
  end

  def test_expand_mentions_reads_each_distinct_id_once
    stub_person(101)

    expanded = @account.comments.expand_mentions(content: "<div>hi</div>", person_ids: [ 101, 101 ])

    assert_requested(:get, "#{BASE_URL}/12345/people/101", times: 1)
    assert_equal [ 101 ], Basecamp::Mentions.mentioned_person_ids(expanded)
  end

  def test_expand_mentions_always_reads_even_when_the_content_names_the_person
    # An sgid already in the content is unsigned and cannot prove the person is
    # mentioned, so it never stands in for the authoritative read. A forged or
    # stale tag naming the right id must not suppress the real mention.
    stale = %(<div><bc-attachment sgid="#{person_sgid(102, signature: "forged")}"></bc-attachment> hi</div>)
    stub_person(102)

    expanded = @account.comments.expand_mentions(content: stale, person_ids: [ 102 ])

    assert_requested(:get, "#{BASE_URL}/12345/people/102", times: 1)
    assert_equal 2, Basecamp::Mentions.bc_attachment_sgids(expanded).length
  end

  def test_expand_mentions_adds_nothing_for_the_exact_sgid_the_read_returned
    sgid = person_sgid(103)
    already = %(<div><bc-attachment sgid="#{sgid}"></bc-attachment> hi</div>)
    stub_person(103, sgid: sgid)

    assert_equal already, @account.comments.expand_mentions(content: already, person_ids: [ 103 ])
  end

  def test_expand_mentions_refuses_a_non_positive_id_before_any_read
    assert_raises(Basecamp::UsageError) do
      @account.comments.expand_mentions(content: "<div>hi</div>", person_ids: [ 0 ])
    end
    assert_raises(Basecamp::UsageError) do
      @account.comments.expand_mentions(content: "<div>hi</div>", person_ids: [ -1 ])
    end
    assert_not_requested(:any, %r{\A#{BASE_URL}})
  end

  def test_a_failed_person_read_posts_nothing
    # Nothing is posted on a partial mention list.
    stub_person(104)
    stub_request(:get, "#{BASE_URL}/12345/people/105")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })
    stub_comment_create

    assert_raises(Basecamp::NotFoundError) do
      @account.comments.create_with_mentions(
        recording_id: RECORDING_ID, content: "<div>hi</div>", person_ids: [ 104, 105 ]
      )
    end

    assert_not_requested(:post, "#{BASE_URL}/12345/recordings/#{RECORDING_ID}/comments.json")
  end

  def test_a_person_the_read_returns_without_an_sgid_fails_the_expansion
    # A projection from somewhere other than a people read carries no
    # attachable_sgid, and a mention cannot be written without one.
    stub_request(:get, "#{BASE_URL}/12345/people/106")
      .to_return(status: 200, body: { "id" => 106, "name" => "No sgid" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::UsageError) do
      @account.comments.expand_mentions(content: "<div>hi</div>", person_ids: [ 106 ])
    end
  end

  def test_create_with_mentions_reads_before_it_writes
    stub_person(107)
    stub_comment_create

    @account.comments.create_with_mentions(
      recording_id: RECORDING_ID, content: "<div>On it.</div>", person_ids: [ 107 ]
    )

    assert_requested(:get, "#{BASE_URL}/12345/people/107", times: 1)
    assert_requested(:post, "#{BASE_URL}/12345/recordings/#{RECORDING_ID}/comments.json", times: 1) do |request|
      JSON.parse(request.body)["content"] ==
        %(<div><bc-attachment sgid="#{person_sgid(107)}"></bc-attachment> On it.</div>)
    end
  end

  def test_create_with_mentions_posts_plain_content_when_nobody_is_mentioned
    stub_comment_create

    @account.comments.create_with_mentions(recording_id: RECORDING_ID, content: "<div>On it.</div>")

    assert_requested(:post, "#{BASE_URL}/12345/recordings/#{RECORDING_ID}/comments.json", times: 1) do |request|
      JSON.parse(request.body)["content"] == "<div>On it.</div>"
    end
  end

  def test_create_with_mentions_requires_content
    assert_raises(Basecamp::UsageError) do
      @account.comments.create_with_mentions(recording_id: RECORDING_ID, content: "", person_ids: [ 108 ])
    end
    assert_not_requested(:any, %r{\A#{BASE_URL}})
  end

  def test_the_generated_create_is_still_reachable_unchanged
    # The composite is prepended, not substituted: `create` is still the plain
    # single POST that writes exactly what it is given.
    stub_comment_create

    @account.comments.create(recording_id: RECORDING_ID, content: %(<div>@nobody</div>))

    assert_requested(:post, "#{BASE_URL}/12345/recordings/#{RECORDING_ID}/comments.json", times: 1)
  end
end
