import XCTest
@testable import Basecamp

/// A dock tool's JSON projection is the **bare** `recordings/recording`
/// partial: `app/views/api/docks/tools/show.json.jbuilder` renders it and adds
/// nothing. Unlike `Todoset`/`Questionnaire`, whose own partials add
/// `json.name recording.recordable.name`, a tool response therefore carries no
/// `name` — and no `enabled` at any layer. Both were `@required` until #650,
/// which made `Tool` undecodable in Swift on every real response.
///
/// These tests pin the real projection so a regeneration that re-tightened
/// either key fails here rather than in a customer's app.
final class ToolDecodeTests: XCTestCase {
    /// Mirrors the key strategy `BaseService` configures, so these tests
    /// exercise the same decode path the SDK actually uses.
    private func makeDecoder() -> JSONDecoder {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        return decoder
    }

    /// `GET /dock/tools/2.json` verbatim from bc3's `doc/api/sections/tools.md`,
    /// which matches `spec/fixtures/tools/get.json`. Note what is NOT here:
    /// `name` and `enabled`.
    private let realProjection = """
    {
      "id": 1069479832,
      "status": "active",
      "visible_to_clients": false,
      "created_at": "2026-05-28T17:23:17.384Z",
      "updated_at": "2026-07-21T00:01:05.529Z",
      "title": "Chat",
      "inherits_status": true,
      "type": "Chat::Transcript",
      "url": "https://3.basecampapi.com/195539477/buckets/2085958505/chats/1069479832.json",
      "app_url": "https://3.basecamp.com/195539477/buckets/2085958505/chats/1069479832",
      "bookmark_url": "https://3.basecampapi.com/195539477/my/bookmarks/BAh7Bkki--7e5f099c.json",
      "subscription_url": "https://3.basecampapi.com/195539477/buckets/2085958505/recordings/1069479832/subscription.json",
      "position": 5,
      "bucket": { "id": 2085958505, "name": "The Leto Laptop", "type": "Project" },
      "creator": {
        "id": 1049715913,
        "name": "Victor Cooper",
        "personable_type": "User",
        "title": "Chief Strategist",
        "email_address": "victor@honchodesign.com",
        "admin": true,
        "owner": true,
        "client": false,
        "employee": true,
        "time_zone": "America/Chicago",
        "avatar_url": "https://3.basecampapi.com/195539477/people/BAhpBMlkkT4=--5fe7b70f/avatar",
        "company": { "id": 1033447817, "name": "Honcho Design" }
      }
    }
    """.data(using: .utf8)!

    /// The regression test for #650. Against the pre-#650 model — `let name:
    /// String`, `let enabled: Bool` — this throws
    /// `DecodingError.keyNotFound` on the very first real response, which is
    /// what "ships-broken" meant. It touches only members that existed before
    /// #650 so it can be run against either model.
    func testDecodesTheRealDockToolProjection() throws {
        let tool: Tool
        do {
            tool = try makeDecoder().decode(Tool.self, from: realProjection)
        } catch {
            // Name the failure rather than asserting a bare `throws`: a
            // keyNotFound for `name` or `enabled` is exactly issue #650.
            return XCTFail("bc3's real dock-tool projection must decode; got \(error)")
        }

        XCTAssertEqual(tool.id, 1069479832)
        XCTAssertEqual(tool.title, "Chat")
        XCTAssertEqual(tool.status, "active")
        XCTAssertEqual(tool.position, 5)
        XCTAssertEqual(tool.bucket?.name, "The Leto Laptop")
    }

    /// `name` and `enabled` are not absent-by-accident — the partial has no
    /// branch that emits either. Decoding must leave both nil, not fail.
    func testNameAndEnabledAreAbsentFromEveryToolResponse() throws {
        let tool = try makeDecoder().decode(Tool.self, from: realProjection)

        XCTAssertNil(tool.name, "docks/tools/show.json.jbuilder emits no `name`")
        XCTAssertNil(tool.enabled, "no layer of the tool projection emits `enabled`")
    }

    /// The seven keys bc3 emits that the spec omitted until #650.
    func testDecodesTheAbsorbedEnvelopeKeys() throws {
        let tool = try makeDecoder().decode(Tool.self, from: realProjection)

        XCTAssertEqual(tool.type, "Chat::Transcript")
        XCTAssertFalse(tool.visibleToClients)
        XCTAssertTrue(tool.inheritsStatus)
        XCTAssertEqual(tool.bookmarkUrl, "https://3.basecampapi.com/195539477/my/bookmarks/BAh7Bkki--7e5f099c.json")
        XCTAssertEqual(
            tool.subscriptionUrl,
            "https://3.basecampapi.com/195539477/buckets/2085958505/recordings/1069479832/subscription.json"
        )
        XCTAssertEqual(tool.creator.name, "Victor Cooper")
        XCTAssertEqual(tool.creator.title, "Chief Strategist")
        XCTAssertNil(tool.parent, "a docked tool is docked, so the partial emits no `parent`")
    }

    /// A tool disabled in the dock is removed from it, not deleted, so
    /// `recording.positioned?` is false and `position` is absent entirely —
    /// absence of `position`, not `enabled: false`, is the disabled signal.
    /// This one is also a Vault, which does not override
    /// `Recordable#subscribable?` (default false), so `subscription_url` is
    /// absent too.
    func testDecodesADisabledToolWithNoPositionAndNoSubscriptionUrl() throws {
        let disabled = """
        {
          "id": 1069479343,
          "status": "active",
          "visible_to_clients": false,
          "created_at": "2026-05-28T17:23:17.021Z",
          "updated_at": "2026-07-19T11:02:44.310Z",
          "title": "Docs & Files",
          "inherits_status": true,
          "type": "Vault",
          "url": "https://3.basecampapi.com/195539477/buckets/2085958505/vaults/1069479343.json",
          "app_url": "https://3.basecamp.com/195539477/buckets/2085958505/vaults/1069479343",
          "bookmark_url": "https://3.basecampapi.com/195539477/my/bookmarks/BAh7Bkki--9a8b7c6d.json",
          "bucket": { "id": 2085958505, "name": "The Leto Laptop", "type": "Project" },
          "creator": { "id": 1049715913, "name": "Victor Cooper" }
        }
        """.data(using: .utf8)!

        let tool = try makeDecoder().decode(Tool.self, from: disabled)

        XCTAssertNil(tool.position)
        XCTAssertNil(tool.subscriptionUrl)
        XCTAssertEqual(tool.type, "Vault")
    }

    /// `parent` is emitted only when `!recording.docked?`. The dock-tool lookup
    /// scopes by recordable TYPE (`Recordable::CORE_GROUPS["dock_tools"]`
    /// includes `Vault`) rather than by dock membership, so a vault nested
    /// inside another vault resolves through `GET /dock/tools/:id` and does
    /// carry a parent.
    ///
    /// It also carries a `position`: `Vault#auto_position?` is true and
    /// positioning is independent of dockedness, so an absent `position` means
    /// "disabled" only for a tool that is actually in the dock.
    func testDecodesANestedVaultCarryingAParent() throws {
        let nested = """
        {
          "id": 1069479562,
          "status": "active",
          "visible_to_clients": false,
          "created_at": "2026-06-02T14:08:51.744Z",
          "updated_at": "2026-07-20T08:19:33.612Z",
          "title": "Contracts",
          "inherits_status": true,
          "type": "Vault",
          "url": "https://3.basecampapi.com/195539477/buckets/2085958505/vaults/1069479562.json",
          "app_url": "https://3.basecamp.com/195539477/buckets/2085958505/vaults/1069479562",
          "bookmark_url": "https://3.basecampapi.com/195539477/my/bookmarks/BAh7Bkki--11223344.json",
          "position": 2,
          "parent": {
            "id": 1069479343,
            "title": "Docs & Files",
            "type": "Vault",
            "url": "https://3.basecampapi.com/195539477/buckets/2085958505/vaults/1069479343.json",
            "app_url": "https://3.basecamp.com/195539477/buckets/2085958505/vaults/1069479343"
          },
          "bucket": { "id": 2085958505, "name": "The Leto Laptop", "type": "Project" },
          "creator": { "id": 1049715913, "name": "Victor Cooper" }
        }
        """.data(using: .utf8)!

        let tool = try makeDecoder().decode(Tool.self, from: nested)

        XCTAssertEqual(tool.parent?.id, 1069479343)
        XCTAssertEqual(tool.parent?.title, "Docs & Files")
        XCTAssertEqual(tool.parent?.type, "Vault")
    }
}
