# frozen_string_literal: true

# Tests for the WormholesService (generated from OpenAPI spec)
#
# Note: Generated service is spec-conformant:
# - create nests under the board: /buckets/{id}/card_tables/{id}/wormholes.json
# - update/delete are wormhole-scoped without .json

require "test_helper"

class WormholesServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_wormhole(id: 1069479400, linked: true)
    {
      "id" => id,
      "status" => "active",
      "visible_to_clients" => false,
      "title" => "Design → Marketing backlog",
      "type" => "Kanban::Wormhole",
      "color" => "#f5d76e",
      "linked" => linked,
      "destination_url" => linked ? "https://3.basecampapi.com/12345/buckets/2085958500/card_tables/columns/1069479500.json" : nil
    }
  end

  def test_create_wormhole
    stub_post("/12345/buckets/2085958499/card_tables/1069479345/wormholes.json",
              response_body: sample_wormhole(id: 99))

    wormhole = @account.wormholes.create(
      bucket_id: 2085958499,
      card_table_id: 1069479345,
      destination_recording_id: 1069479500
    )

    assert_equal 99, wormhole["id"]
    assert_equal true, wormhole["linked"]
    assert_not_nil wormhole["destination_url"]
  end

  def test_create_wormhole_raises_validation_error_at_limit
    stub_post("/12345/buckets/2085958499/card_tables/1069479345/wormholes.json",
              response_body: { "error" => "Limit reached" }, status: 422)

    assert_raises(Basecamp::ValidationError) do
      @account.wormholes.create(
        bucket_id: 2085958499,
        card_table_id: 1069479345,
        destination_recording_id: 1069479500
      )
    end
  end

  def test_create_wormhole_raises_not_found_for_bad_destination
    stub_post("/12345/buckets/2085958499/card_tables/1069479345/wormholes.json",
              response_body: { "error" => "Not found" }, status: 404)

    assert_raises(Basecamp::NotFoundError) do
      @account.wormholes.create(
        bucket_id: 2085958499,
        card_table_id: 1069479345,
        destination_recording_id: 999
      )
    end
  end

  def test_update_wormhole
    stub_put("/12345/buckets/2085958499/card_tables/wormholes/1069479400",
             response_body: sample_wormhole(id: 1069479400))

    wormhole = @account.wormholes.update(
      bucket_id: 2085958499,
      wormhole_id: 1069479400,
      destination_recording_id: 1069479501
    )

    assert_equal 1069479400, wormhole["id"]
  end

  def test_update_wormhole_not_found
    stub_put("/12345/buckets/2085958499/card_tables/wormholes/999",
             response_body: { "error" => "Not found" }, status: 404)

    assert_raises(Basecamp::NotFoundError) do
      @account.wormholes.update(
        bucket_id: 2085958499,
        wormhole_id: 999,
        destination_recording_id: 1
      )
    end
  end

  def test_delete_wormhole
    stub_delete("/12345/buckets/2085958499/card_tables/wormholes/1069479400")

    result = @account.wormholes.delete(bucket_id: 2085958499, wormhole_id: 1069479400)

    assert_nil result
  end

  def test_delete_wormhole_forbidden
    stub_delete("/12345/buckets/2085958499/card_tables/wormholes/1069479400", status: 403)

    assert_raises(Basecamp::ForbiddenError) do
      @account.wormholes.delete(bucket_id: 2085958499, wormhole_id: 1069479400)
    end
  end

  def test_delete_wormhole_not_found
    stub_delete("/12345/buckets/2085958499/card_tables/wormholes/999", status: 404)

    assert_raises(Basecamp::NotFoundError) do
      @account.wormholes.delete(bucket_id: 2085958499, wormhole_id: 999)
    end
  end
end
