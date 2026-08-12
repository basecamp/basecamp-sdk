# frozen_string_literal: true

require "test_helper"

class ToolsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  # A dock tool's projection is the BARE recordings/recording partial:
  # app/views/api/docks/tools/show.json.jbuilder is one line —
  # `json.partial! "recordings/recording", recording: @recording` — and adds
  # nothing. Unlike Todoset/Questionnaire, whose own recordable partials add it,
  # a tool response carries no `name`, and no `enabled` at all. The fabricated
  # stub bodies these tests used to carry ("name" => ..., "enabled" => true) are
  # exactly how `name`/`enabled` stayed @required through six SDKs (#650), so
  # the stubs below load the shared, coverage-guarded fixtures
  # (spec/fixtures/manifest.yaml) and cannot drift back.
  def test_get
    response = load_fixture("tools/get.json")

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/dock/tools/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.tools.get(tool_id: response["id"])
    assert_equal response["id"], result["id"]
    assert_equal "Chat", result["title"]
    assert_equal "Chat::Transcript", result["type"]
    assert_equal false, result["visible_to_clients"]
    assert_equal true, result["inherits_status"]
    assert_equal "active", result["status"]
    assert_equal response["bookmark_url"], result["bookmark_url"]
    # Chat::Transcript overrides Recordable#subscribable?, so the partial's
    # `if recording.subscribable?` branch renders subscription_url here.
    assert_equal response["subscription_url"], result["subscription_url"]
    # Present because the tool is on the dock (`recording.positioned?`).
    assert_equal 5, result["position"]
    assert_equal response.dig("creator", "id"), result.dig("creator", "id")
    assert_equal "Victor Cooper", result.dig("creator", "name")
    assert_equal response.dig("bucket", "id"), result.dig("bucket", "id")
    assert_equal "Project", result.dig("bucket", "type")
  end

  # Regression guard for #650. `name` and `enabled` were @required on Tool, yet
  # BC3 emits neither key on ANY tool response — so this test's body is not an
  # edge case, it is every real response. The old stub fabricated both keys and
  # asserted them, which is why the bug never went red.
  def test_get_response_carries_neither_name_nor_enabled
    response = load_fixture("tools/get.json")

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/dock/tools/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.tools.get(tool_id: response["id"])

    assert_not result.key?("name"), "the bare recording partial emits no name"
    assert_not result.key?("enabled"), "the bare recording partial emits no enabled"
    # The keys BC3 does emit still flow through intact.
    assert_equal response["id"], result["id"]
    assert_equal "Chat", result["title"]
    assert_equal "Chat::Transcript", result["type"]
    assert_equal true, result["inherits_status"]
    assert_equal response.dig("creator", "id"), result.dig("creator", "id")
  end

  # A disabled tool is removed from the dock but NOT deleted, so
  # `recording.positioned?` is false and `position` is absent entirely —
  # absence of `position`, not `enabled => false`, is the disabled signal. This
  # one is also a Vault, which does not override Recordable#subscribable?
  # (default false), so `subscription_url` is absent too.
  def test_get_disabled_tool_has_no_position_and_no_subscription_url
    response = load_fixture("tools/disabled.json")

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/dock/tools/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.tools.get(tool_id: response["id"])

    assert_not result.key?("position"), "a disabled tool is off the dock, so position is not rendered"
    assert_not result.key?("subscription_url"), "Vault is not subscribable"
    assert_not result.key?("enabled")
    assert_not result.key?("name")
    assert_equal "Vault", result["type"]
    assert_equal "Docs & Files", result["title"]
    assert_equal response["bookmark_url"], result["bookmark_url"]
  end

  # `parent` is emitted only when `!recording.docked?`. The dock-tool lookup
  # scopes by recordable TYPE (Recordable::CORE_GROUPS["dock_tools"] includes
  # Vault) rather than by dock membership, so a vault nested inside another
  # vault resolves through GET /dock/tools/:id and does carry a parent — while
  # the docked tool above carries none.
  def test_get_nested_vault_carries_a_parent
    response = load_fixture("tools/nested_vault.json")

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/dock/tools/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.tools.get(tool_id: response["id"])

    assert_equal response.dig("parent", "id"), result.dig("parent", "id")
    assert_equal "Vault", result.dig("parent", "type")
    assert_equal "Docs & Files", result.dig("parent", "title")
    assert_equal "Vault", result["type"]
    assert_equal "Contracts", result["title"]
    assert_not result.key?("name")
    assert_not result.key?("enabled")
  end

  def test_get_docked_tool_has_no_parent
    response = load_fixture("tools/get.json")

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/dock/tools/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.tools.get(tool_id: response["id"])

    assert_not result.key?("parent"), "a docked tool has no parent"
    assert_equal 5, result["position"]
  end

  def test_create
    response = load_fixture("tools/create.json")

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/buckets/456/dock/tools\.json})
      .with(body: hash_including("tool_type" => "Chat::Transcript", "title" => "Q&A Chat"))
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.tools.create(bucket_id: 456, tool_type: "Chat::Transcript", title: "Q&A Chat")
    assert_equal response["id"], result["id"]
    assert_equal "Q&A Chat", result["title"]
    assert_equal "Chat::Transcript", result["type"]
    assert_equal true, result["visible_to_clients"]
    assert_equal 6, result["position"]
    assert_equal response.dig("creator", "id"), result.dig("creator", "id")
    # The create projection is the same bare partial as get: no name, no enabled.
    assert_not result.key?("name")
    assert_not result.key?("enabled")
  end

  def test_create_omits_title_when_not_provided
    response = load_fixture("tools/create.json")

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/buckets/456/dock/tools\.json})
      .with { |req| JSON.parse(req.body) == { "tool_type" => "Chat::Transcript" } }
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.tools.create(bucket_id: 456, tool_type: "Chat::Transcript")
    assert_equal response["id"], result["id"]
    assert_equal "Q&A Chat", result["title"]
  end

  # visible_to_clients is tri-state: unset omits the key (compact_params drops nil),
  # true/false are sent verbatim. An explicit false must reach the wire. Only
  # Chat::Transcript and Kanban::Board honor it; all other tool types ignore it.
  def test_create_omits_visible_to_clients_when_unset
    response = load_fixture("tools/create.json")
    stub_request(:post, %r{https://3\.basecampapi\.com/12345/buckets/456/dock/tools\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    @account.tools.create(bucket_id: 456, tool_type: "Chat::Transcript")

    assert_requested(:post, "https://3.basecampapi.com/12345/buckets/456/dock/tools.json") do |req|
      !JSON.parse(req.body).key?("visible_to_clients")
    end
  end

  def test_create_sends_visible_to_clients_true
    response = load_fixture("tools/create.json")
    stub_request(:post, %r{https://3\.basecampapi\.com/12345/buckets/456/dock/tools\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    @account.tools.create(bucket_id: 456, tool_type: "Chat::Transcript", visible_to_clients: true)

    assert_requested(:post, "https://3.basecampapi.com/12345/buckets/456/dock/tools.json",
      body: hash_including("visible_to_clients" => true))
  end

  def test_create_sends_visible_to_clients_false
    response = load_fixture("tools/create.json")
    stub_request(:post, %r{https://3\.basecampapi\.com/12345/buckets/456/dock/tools\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    @account.tools.create(bucket_id: 456, tool_type: "Chat::Transcript", visible_to_clients: false)

    assert_requested(:post, "https://3.basecampapi.com/12345/buckets/456/dock/tools.json",
      body: hash_including("visible_to_clients" => false))
  end

  def test_create_raises_validation_error_on_422
    stub_request(:post, %r{https://3\.basecampapi\.com/12345/buckets/456/dock/tools\.json})
      .to_return(
        status: 422,
        body: { "error" => "Tool type is not included in the list" }.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    assert_raises(Basecamp::ValidationError) do
      @account.tools.create(bucket_id: 456, tool_type: "Bogus::Tool")
    end
  end

  def test_update
    response = load_fixture("tools/update.json")

    stub_request(:put, %r{https://3\.basecampapi\.com/12345/dock/tools/\d+})
      .with(body: hash_including("title" => "Team Chat"))
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.tools.update(tool_id: response["id"], title: "Team Chat")
    assert_equal "Team Chat", result["title"]
    assert_equal "Chat::Transcript", result["type"]
    assert_equal true, result["inherits_status"]
    assert_equal 5, result["position"]
    # The update projection is the same bare partial as get: no name, no enabled.
    assert_not result.key?("name")
    assert_not result.key?("enabled")
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
