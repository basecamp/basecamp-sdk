# frozen_string_literal: true

# Tests for the PingsService (generated from OpenAPI spec)
#
# A Ping is a direct chat or a named group. Its id is the id of the
# bucket that holds it (bucket type "Circle", also the wire path), which is the
# `bucket.id` a Ping's recordings carry.

require "test_helper"

class PingsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list
    stub_get("/12345/circles.json", response_body: load_fixture("pings/list.json"))

    pings = @account.pings.list
    group, direct = pings

    assert_equal 2, pings.length
    assert_equal 2085958600, group["id"]
    assert_equal "Launch crew", group["title"]
    assert_equal true, group["named"]
    assert_equal [ "Annie Bryan", "Cheryl Walters" ], group["people"].map { |p| p["name"] }
    assert_equal false, direct["named"]
  end

  def test_list_empty
    stub_get("/12345/circles.json", response_body: [])

    assert_equal [], @account.pings.list
  end

  def test_get
    stub_get("/12345/circles/2085958600", response_body: load_fixture("pings/get.json"))

    ping = @account.pings.get(ping_id: 2085958600)

    assert_equal 2085958600, ping["id"]
    assert_equal "https://3.basecampapi.com/195539477/buckets/2085958600/chats/1069479600.json", ping["room_url"]
    assert_equal 2, ping["people"].length
  end

  def test_get_not_found
    stub_get("/12345/circles/999", status: 404, response_body: { "error" => "Record not found" })

    assert_raises(Basecamp::NotFoundError) { @account.pings.get(ping_id: 999) }
  end

  def test_get_forbidden_when_pings_are_disabled
    stub_get("/12345/circles/1", status: 403, response_body: { "error" => "Pings have been disabled" })

    assert_raises(Basecamp::ForbiddenError) { @account.pings.get(ping_id: 1) }
  end

  def test_create
    stub = stub_post("/12345/circles.json", response_body: load_fixture("pings/get.json"))
      .with(body: { user_ids: [ 1049715915, 1049715916 ], new_group: true }.to_json)

    ping = @account.pings.create(user_ids: [ 1049715915, 1049715916 ], new_group: true)

    assert_equal 2085958600, ping["id"]
    assert_requested(stub)
  end

  def test_create_omits_new_group_unless_set
    stub = stub_post("/12345/circles.json", response_body: load_fixture("pings/get.json"))
      .with(body: { user_ids: [ 1049715915 ] }.to_json)

    @account.pings.create(user_ids: [ 1049715915 ])

    assert_requested(stub)
  end

  def test_create_unprocessable
    stub_request(:post, "#{BASE_URL}/12345/circles.json").to_return(status: 422, body: "")

    assert_raises(Basecamp::ValidationError) { @account.pings.create(user_ids: [ 42 ]) }
  end
end
