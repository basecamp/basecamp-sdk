# frozen_string_literal: true

require "test_helper"

# External links (doors). Fixtures are bc3's documented examples, shared under
# spec/fixtures/doors/ and guarded by the fixture-completeness manifest.
class DoorsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list
    stub_request(:get, "https://3.basecampapi.com/12345/buckets/2085958504/dock/doors.json")
      .to_return(status: 200, body: load_fixture("doors/list.json").to_json,
                 headers: { "Content-Type" => "application/json" })

    doors = @account.doors.list(bucket_id: 2085958504).to_a

    assert_equal 1, doors.length
    assert_equal "Door", doors[0]["type"]
    assert_equal 8, doors[0]["position"]
  end

  def test_get_returns_the_full_shape
    stub_request(:get, "https://3.basecampapi.com/12345/dock/doors/1069480290")
      .to_return(status: 200, body: load_fixture("doors/get.json").to_json,
                 headers: { "Content-Type" => "application/json" })

    door = @account.doors.get(door_id: 1069480290)

    assert_equal "https://www.figma.com/file/abc123/Design-system", door["url"]
    assert_equal "figma", door["service"]["code"]
    assert_equal "<div>Shared Figma workspace</div>", door["description"]
  end

  def test_get_not_found
    stub_request(:get, "https://3.basecampapi.com/12345/dock/doors/9999")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) { @account.doors.get(door_id: 9999) }
  end

  def test_create_wraps_the_door_and_puts_visibility_beside_it
    request = load_fixture("doors/create-request.json")
    stub_request(:post, "https://3.basecampapi.com/12345/buckets/2085958504/dock/doors.json")
      .with(body: request.to_json)
      .to_return(status: 201, body: load_fixture("doors/get.json").to_json,
                 headers: { "Content-Type" => "application/json" })

    door = @account.doors.create(
      bucket_id: 2085958504,
      door: request["door"],
      visible_to_clients: request["visible_to_clients"]
    )

    assert_equal 1069480290, door["id"]
  end

  def test_create_omits_visibility_when_not_given
    stub_request(:post, "https://3.basecampapi.com/12345/buckets/1/dock/doors.json")
      .to_return(status: 201, body: load_fixture("doors/get.json").to_json,
                 headers: { "Content-Type" => "application/json" })

    @account.doors.create(bucket_id: 1, door: { "service" => "other", "url" => "https://example.com" })

    # WebMock treats a hash body as a subset match, so parse it instead.
    assert_requested(:post, "https://3.basecampapi.com/12345/buckets/1/dock/doors.json") do |req|
      JSON.parse(req.body) == { "door" => { "service" => "other", "url" => "https://example.com" } }
    end
  end

  def test_create_invalid_url
    stub_request(:post, "https://3.basecampapi.com/12345/buckets/1/dock/doors.json")
      .to_return(status: 422, body: { "errors" => { "url" => [ "is invalid" ] } }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::ValidationError) do
      @account.doors.create(bucket_id: 1, door: { "service" => "other", "url" => "javascript:alert(1)" })
    end
  end

  def test_update
    request = load_fixture("doors/update-request.json")
    stub_request(:put, "https://3.basecampapi.com/12345/dock/doors/1069480290")
      .with(body: request.to_json)
      .to_return(status: 200, body: load_fixture("doors/get.json").to_json,
                 headers: { "Content-Type" => "application/json" })

    door = @account.doors.update(
      door_id: 1069480290,
      door: request["door"],
      visible_to_clients: request["visible_to_clients"]
    )

    assert_equal 1069480290, door["id"]
  end

  def test_update_visibility_alone
    stub_request(:put, "https://3.basecampapi.com/12345/dock/doors/1069480290")
      .to_return(status: 200, body: load_fixture("doors/get.json").to_json,
                 headers: { "Content-Type" => "application/json" })

    @account.doors.update(door_id: 1069480290, visible_to_clients: true)

    assert_requested(:put, "https://3.basecampapi.com/12345/dock/doors/1069480290") do |req|
      JSON.parse(req.body) == { "visible_to_clients" => true }
    end
  end

  def test_update_forbidden
    stub_request(:put, "https://3.basecampapi.com/12345/dock/doors/1")
      .to_return(status: 403, body: { "error" => "Forbidden" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::ForbiddenError) do
      @account.doors.update(door_id: 1, door: { "title" => "x" })
    end
  end

  def test_delete
    stub_request(:delete, "https://3.basecampapi.com/12345/dock/doors/1069480290")
      .to_return(status: 204, body: "")

    assert_nil @account.doors.delete(door_id: 1069480290)
    assert_requested(:delete, "https://3.basecampapi.com/12345/dock/doors/1069480290")
  end

  def test_delete_not_found
    stub_request(:delete, "https://3.basecampapi.com/12345/dock/doors/9999")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) { @account.doors.delete(door_id: 9999) }
  end
end
