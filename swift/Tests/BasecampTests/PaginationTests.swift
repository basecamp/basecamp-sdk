import XCTest
@testable import Basecamp

final class PaginationTests: XCTestCase {

    // MARK: - ListResult

    func testListResultEmpty() {
        let result = ListResult<Int>()
        XCTAssertEqual(result.count, 0)
        XCTAssertTrue(result.isEmpty)
        XCTAssertEqual(result.meta.totalCount, 0)
        XCTAssertFalse(result.meta.truncated)
    }

    func testListResultWithItems() {
        let result = ListResult([1, 2, 3], meta: ListMeta(totalCount: 10, truncated: true))
        XCTAssertEqual(result.count, 3)
        XCTAssertEqual(result[0], 1)
        XCTAssertEqual(result[1], 2)
        XCTAssertEqual(result[2], 3)
        XCTAssertEqual(result.meta.totalCount, 10)
        XCTAssertTrue(result.meta.truncated)
    }

    func testListResultSupportsForIn() {
        let result = ListResult([10, 20, 30], meta: ListMeta(totalCount: 3))
        var collected: [Int] = []
        for item in result {
            collected.append(item)
        }
        XCTAssertEqual(collected, [10, 20, 30])
    }

    func testListResultSupportsMap() {
        let result = ListResult(["a", "b", "c"], meta: ListMeta(totalCount: 3))
        let uppercased = result.map { $0.uppercased() }
        XCTAssertEqual(uppercased, ["A", "B", "C"])
    }

    func testListResultSupportsFilter() {
        let result = ListResult([1, 2, 3, 4, 5], meta: ListMeta(totalCount: 5))
        let evens = result.filter { $0 % 2 == 0 }
        XCTAssertEqual(evens, [2, 4])
    }

    func testListResultSupportsSubscriptRange() {
        let result = ListResult([10, 20, 30, 40], meta: ListMeta(totalCount: 4))
        let slice = result[1..<3]
        XCTAssertEqual(Array(slice), [20, 30])
    }

    // MARK: - parseNextLink

    func testParseNextLinkSimple() {
        let header = "<https://3.basecampapi.com/999/projects.json?page=2>; rel=\"next\""
        XCTAssertEqual(
            parseNextLink(header),
            "https://3.basecampapi.com/999/projects.json?page=2"
        )
    }

    func testParseNextLinkMultipleRels() {
        let header = """
        <https://example.com?page=1>; rel="prev", \
        <https://example.com?page=3>; rel="next"
        """
        XCTAssertEqual(parseNextLink(header), "https://example.com?page=3")
    }

    func testParseNextLinkNil() {
        XCTAssertNil(parseNextLink(nil))
    }

    func testParseNextLinkEmpty() {
        XCTAssertNil(parseNextLink(""))
    }

    func testParseNextLinkNoNext() {
        let header = "<https://example.com?page=1>; rel=\"prev\""
        XCTAssertNil(parseNextLink(header))
    }

    // MARK: - parseNextLink, adversarial input
    //
    // The Link header is attacker-influenced (isSameOrigin exists to stop SSRF
    // through a poisoned one), so malformed shapes are a contract, not a
    // curiosity. The same six cases exist in all six SDKs.

    func testParseNextLinkBracketNeverCloses() {
        XCTAssertNil(parseNextLink("<https://api.example.com/page2; rel=\"next\""))
    }

    func testParseNextLinkClosingBracketBeforeOpeningBracket() {
        // Was broken here: firstIndex(of: "<") and firstIndex(of: ">") both ran
        // from the start, so a ">" ahead of the "<" failed the start < end
        // guard and extraction silently returned nothing. The regex SDKs always
        // read this correctly.
        XCTAssertEqual(
            parseNextLink(">x<https://api.example.com/page2>; rel=\"next\""),
            "https://api.example.com/page2"
        )
    }

    func testParseNextLinkTruncatesURLAtFirstRawClosingBracket() {
        // Parity with the old <([^>]+)> spelling: [^>] cannot span a ">".
        XCTAssertEqual(
            parseNextLink("<https://api.example.com/page2?q=a>b>; rel=\"next\""),
            "https://api.example.com/page2?q=a"
        )
    }

    func testParseNextLinkTakesFirstOfMultipleBracketPairs() {
        // Parity with the old spelling: leftmost match wins.
        XCTAssertEqual(
            parseNextLink("<https://api.example.com/a> <https://api.example.com/b>; rel=\"next\""),
            "https://api.example.com/a"
        )
    }

    func testParseNextLinkSkipsEmptyBracketPair() {
        // Parity with the old spelling: [^>]+ requires at least one character,
        // so an empty <> is not a match and the scan moves on. A naive search
        // for ">" after the "<" without this check would return "".
        XCTAssertEqual(
            parseNextLink("<> <https://api.example.com/page2>; rel=\"next\""),
            "https://api.example.com/page2"
        )
    }

    func testParseNextLinkKeepsScanningPastMalformedPart() {
        XCTAssertEqual(
            parseNextLink("<malformed; rel=\"next\", <https://api.example.com/page2>; rel=\"next\""),
            "https://api.example.com/page2"
        )
    }

    func testParseNextLinkPathologicalHeader() {
        // Many "<" start positions with no reachable ">" — the shape that
        // punishes a backtracking regex. Asserting behaviour and completion,
        // not elapsed time: this suite already has timing flakiness (#655) and
        // a duration bound would add more.
        let many = String(repeating: "<", count: 50_000)
        XCTAssertNil(parseNextLink("\(many); rel=\"next\""))
        // A ">" present but unreachable defeats the literal-prescan shortcut
        // some regex engines use to bail early.
        XCTAssertNil(parseNextLink(">\(many); rel=\"next\""))
    }

    func testParseNextLinkManyEmptyBracketPairs() {
        // The pathological case for the scan that replaced the regex, which is
        // a different shape from the one above: that header returns after a
        // single search for ">" and never takes the empty-<> branch, so the
        // skip loop's own worst case went untested. Every "<>" here advances
        // the cursor by one and goes round again — the only path where a
        // non-constant-time index lookup would compound into quadratic
        // behaviour. Behaviour and completion again, not elapsed time.
        let pairs = String(repeating: "<>", count: 50_000)
        // No non-empty pair anywhere: every iteration skips, then it runs out.
        XCTAssertNil(parseNextLink("\(pairs); rel=\"next\""))
        // Same prefix, but the skips have to land on a real pair at the end.
        XCTAssertEqual(
            parseNextLink("\(pairs)<https://api.example.com/page2>; rel=\"next\""),
            "https://api.example.com/page2"
        )
    }

    // MARK: - resolveURL

    func testResolveAbsoluteURL() {
        let resolved = resolveURL(base: "https://a.com/foo", target: "https://b.com/bar")
        XCTAssertEqual(resolved, "https://b.com/bar")
    }

    func testResolveRelativeURL() {
        let resolved = resolveURL(base: "https://a.com/foo/bar", target: "/baz")
        XCTAssertEqual(resolved, "https://a.com/baz")
    }

    // MARK: - isSameOrigin

    func testSameOriginSameURL() {
        XCTAssertTrue(isSameOrigin("https://a.com/foo", "https://a.com/bar"))
    }

    func testSameOriginDifferentHost() {
        XCTAssertFalse(isSameOrigin("https://a.com/foo", "https://b.com/foo"))
    }

    func testSameOriginDifferentScheme() {
        XCTAssertFalse(isSameOrigin("https://a.com", "http://a.com"))
    }

    func testSameOriginDefaultPort() {
        XCTAssertTrue(isSameOrigin("https://a.com", "https://a.com:443"))
    }

    func testSameOriginDifferentPort() {
        XCTAssertFalse(isSameOrigin("https://a.com:443", "https://a.com:8443"))
    }

    // MARK: - parseTotalCount

    func testParseTotalCountFromHeader() {
        let response = makeHTTPResponse(statusCode: 200, headers: ["X-Total-Count": "42"])
        XCTAssertEqual(parseTotalCount(response), 42)
    }

    func testParseTotalCountMissing() {
        let response = makeHTTPResponse(statusCode: 200)
        XCTAssertEqual(parseTotalCount(response), 0)
    }

    func testParseTotalCountNonNumeric() {
        let response = makeHTTPResponse(statusCode: 200, headers: ["X-Total-Count": "abc"])
        XCTAssertEqual(parseTotalCount(response), 0)
    }

    // MARK: - Multi-page End-to-End via MockTransport

    func testMultiPagePagination() async throws {
        let page1 = [
            ["id": 1, "name": "Project A", "status": "active",
             "app_url": "https://3.basecamp.com/1/projects/1", "url": "https://3.basecampapi.com/1/projects/1.json",
             "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"] as [String: Any],
        ]
        let page2 = [
            ["id": 2, "name": "Project B", "status": "active",
             "app_url": "https://3.basecamp.com/1/projects/2", "url": "https://3.basecampapi.com/1/projects/2.json",
             "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"] as [String: Any],
        ]

        let page1Data = try JSONSerialization.data(withJSONObject: page1)
        let page2Data = try JSONSerialization.data(withJSONObject: page2)

        let transport = MockTransport { request in
            let urlString = request.url!.absoluteString
            if urlString.contains("page=2") {
                let response = HTTPURLResponse(
                    url: request.url!, statusCode: 200,
                    httpVersion: "HTTP/1.1", headerFields: ["X-Total-Count": "2"]
                )!
                return (page2Data, response)
            } else {
                let response = HTTPURLResponse(
                    url: request.url!, statusCode: 200,
                    httpVersion: "HTTP/1.1",
                    headerFields: [
                        "Link": "<https://3.basecampapi.com/999999999/projects.json?page=2>; rel=\"next\"",
                        "X-Total-Count": "2",
                    ]
                )!
                return (page1Data, response)
            }
        }

        let account = makeTestAccountClient(transport: transport)
        let result: ListResult<Project> = try await account.projects.list()

        XCTAssertEqual(result.count, 2)
        XCTAssertEqual(result[0].name, "Project A")
        XCTAssertEqual(result[1].name, "Project B")
        XCTAssertEqual(result.meta.totalCount, 2)
        XCTAssertFalse(result.meta.truncated)
    }

    // MARK: - Wrapped Pagination (PersonProgress)

    func testWrappedPaginationAccumulatesAcrossPages() async throws {
        let page1 = [
            "person": ["id": 456, "name": "Jane Doe", "email_address": "jane@example.com"],
            "events": [
                ["id": 1, "action": "created", "target": "todo", "title": "Event 1",
                 "created_at": "2026-01-01T00:00:00Z"],
                ["id": 2, "action": "completed", "target": "todo", "title": "Event 2",
                 "created_at": "2026-01-02T00:00:00Z"],
            ]
        ] as [String: Any]
        let page2 = [
            "person": ["id": 456, "name": "Jane Doe", "email_address": "jane@example.com"],
            "events": [
                ["id": 3, "action": "updated", "target": "message", "title": "Event 3",
                 "created_at": "2026-01-03T00:00:00Z"],
            ]
        ] as [String: Any]

        let page1Data = try JSONSerialization.data(withJSONObject: page1)
        let page2Data = try JSONSerialization.data(withJSONObject: page2)

        let transport = MockTransport { request in
            let urlString = request.url!.absoluteString
            if urlString.contains("page=2") {
                let response = HTTPURLResponse(
                    url: request.url!, statusCode: 200,
                    httpVersion: "HTTP/1.1",
                    headerFields: [
                        "Content-Type": "application/json",
                    ]
                )!
                return (page2Data, response)
            } else {
                let response = HTTPURLResponse(
                    url: request.url!, statusCode: 200,
                    httpVersion: "HTTP/1.1",
                    headerFields: [
                        "Content-Type": "application/json",
                        "Link": "<https://3.basecampapi.com/999999999/reports/users/progress/456.json?page=2>; rel=\"next\"",
                        "X-Total-Count": "3",
                    ]
                )!
                return (page1Data, response)
            }
        }

        let account = makeTestAccountClient(transport: transport)
        let result = try await account.reports.personProgress(personId: 456)

        // Wrapper field preserved from page 1
        XCTAssertEqual(result.person.name, "Jane Doe")

        // Events accumulated across both pages
        XCTAssertEqual(result.events.count, 3)
        XCTAssertEqual(result.events[0].title, "Event 1")
        XCTAssertEqual(result.events[1].title, "Event 2")
        XCTAssertEqual(result.events[2].title, "Event 3")
        XCTAssertEqual(result.events.meta.totalCount, 3)
        XCTAssertFalse(result.events.meta.truncated)
    }

    // MARK: - SSRF Rejection

    func testSSRFRejectionOnDifferentOrigin() async throws {
        let page1 = [
            ["id": 1, "name": "Project", "status": "active",
             "app_url": "https://3.basecamp.com/1/projects/1", "url": "https://3.basecampapi.com/1/projects/1.json",
             "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"] as [String: Any],
        ]
        let page1Data = try JSONSerialization.data(withJSONObject: page1)

        let transport = MockTransport { request in
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1",
                headerFields: [
                    "Link": "<https://evil.com/steal-tokens?page=2>; rel=\"next\"",
                    "X-Total-Count": "10",
                ]
            )!
            return (page1Data, response)
        }

        let account = makeTestAccountClient(transport: transport)

        do {
            let _: ListResult<Project> = try await account.projects.list()
            XCTFail("Expected SSRF error for different-origin Link header")
        } catch let error as BasecampError {
            if case .api(let message, _, _, _, _) = error {
                XCTAssertTrue(message.contains("different origin"),
                              "Error should mention different origin, got: \(message)")
            } else {
                XCTFail("Expected .api error, got \(error)")
            }
        }
    }

    // MARK: - maxPages Cap

    func testMaxPagesCapTriggersTruncated() async throws {
        // Each page has 1 item and a Link to the next
        let transport = MockTransport { request in
            let urlString = request.url!.absoluteString
            let pageNum = urlString.contains("page=") ?
                Int(urlString.split(separator: "=").last!) ?? 1 : 1
            let item = [["id": pageNum, "name": "Project \(pageNum)", "status": "active",
                          "app_url": "https://3.basecamp.com/1/projects/\(pageNum)",
                          "url": "https://3.basecampapi.com/1/projects/\(pageNum).json",
                          "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"] as [String: Any]]
            let data = try! JSONSerialization.data(withJSONObject: item)
            let nextPage = pageNum + 1
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1",
                headerFields: [
                    "Link": "<https://3.basecampapi.com/999999999/projects.json?page=\(nextPage)>; rel=\"next\"",
                    "X-Total-Count": "100",
                ]
            )!
            return (data, response)
        }

        // Create client with maxPages = 3 (fetches page 1 + follows 2 more = 3 total)
        let client = BasecampClient(
            tokenProvider: StaticTokenProvider("test-token"),
            userAgent: "test-suite",
            config: BasecampConfig(
                baseURL: "https://3.basecampapi.com",
                enableRetry: false,
                enableCache: false,
                maxPages: 3
            ),
            transport: transport
        )
        let account = client.forAccount("999999999")

        let result: ListResult<Project> = try await account.projects.list()

        XCTAssertEqual(result.count, 3)
        XCTAssertTrue(result.meta.truncated, "Should be truncated when hitting maxPages cap")
    }

    // MARK: - maxItems Cap Boundary (#513)

    func testMaxItemsExactBoundaryOnLaterPageNotTruncated() async throws {
        // Page 1: 2 items + next Link; page 2: 1 item, NO Link.
        // maxItems = 3 lands exactly on the final item of the last page —
        // nothing was dropped and no next page exists, so truncated is false.
        let page1Data = projectPageData(ids: [1, 2])
        let page2Data = projectPageData(ids: [3])

        let transport = MockTransport { request in
            let urlString = request.url!.absoluteString
            if urlString.contains("page=2") {
                return (page2Data, pageResponse(url: request.url!))
            }
            return (page1Data, pageResponse(url: request.url!, link: nextProjectsLink(page: 2)))
        }

        let account = makeTestAccountClient(transport: transport)
        let result = try await account.projects.list(options: ListProjectOptions(maxItems: 3))

        XCTAssertEqual(result.count, 3)
        XCTAssertFalse(result.meta.truncated,
                       "Cap met exactly with no next Link — nothing was truncated")
    }

    func testMaxItemsDropOnFollowedPageIsTruncated() async throws {
        // Page 1: 1 item + next Link; page 2: 2 items, NO Link.
        // maxItems = 2 drops one collected item mid-page — truncated must be
        // true via the excess-items clause even though no next Link exists.
        let page1Data = projectPageData(ids: [1])
        let page2Data = projectPageData(ids: [2, 3])

        let transport = MockTransport { request in
            let urlString = request.url!.absoluteString
            if urlString.contains("page=2") {
                return (page2Data, pageResponse(url: request.url!))
            }
            return (page1Data, pageResponse(url: request.url!, link: nextProjectsLink(page: 2)))
        }

        let account = makeTestAccountClient(transport: transport)
        let result = try await account.projects.list(options: ListProjectOptions(maxItems: 2))

        XCTAssertEqual(result.count, 2)
        XCTAssertTrue(result.meta.truncated,
                      "An item was dropped by the cap — truncated even with no next Link")
    }

    func testMaxItemsExactBoundaryOnLaterPageWithNextLinkTruncated() async throws {
        // Control: same shape, but page 2 still advertises a next Link —
        // more results exist beyond the cap, so truncated is true.
        let page1Data = projectPageData(ids: [1, 2])
        let page2Data = projectPageData(ids: [3])
        let emptyData = try JSONSerialization.data(withJSONObject: [] as [Any])

        let transport = MockTransport { request in
            let urlString = request.url!.absoluteString
            if urlString.contains("page=3") {
                return (emptyData, pageResponse(url: request.url!))
            }
            if urlString.contains("page=2") {
                return (page2Data, pageResponse(url: request.url!, link: nextProjectsLink(page: 3)))
            }
            return (page1Data, pageResponse(url: request.url!, link: nextProjectsLink(page: 2)))
        }

        let account = makeTestAccountClient(transport: transport)
        let result = try await account.projects.list(options: ListProjectOptions(maxItems: 3))

        XCTAssertEqual(result.count, 3)
        XCTAssertTrue(result.meta.truncated,
                      "Next Link on the capping page means more results exist")
        XCTAssertEqual(transport.requests.count, 2,
                       "Page 3 must not be fetched once the cap is satisfied")
    }

    func testWrappedMaxItemsExactBoundaryOnLaterPageNotTruncated() async throws {
        // Wrapped twin (PersonProgress): page 1 has 2 events + next Link,
        // page 2 has the final event and no Link. maxItems = 3 -> not truncated.
        let page1: [String: Any] = [
            "person": ["id": 456, "name": "Jane Doe", "email_address": "jane@example.com"],
            "events": [
                ["id": 1, "action": "created", "target": "todo", "title": "Event 1",
                 "created_at": "2026-01-01T00:00:00Z"],
                ["id": 2, "action": "completed", "target": "todo", "title": "Event 2",
                 "created_at": "2026-01-02T00:00:00Z"],
            ],
        ]
        let page2: [String: Any] = [
            "person": ["id": 456, "name": "Jane Doe", "email_address": "jane@example.com"],
            "events": [
                ["id": 3, "action": "updated", "target": "message", "title": "Event 3",
                 "created_at": "2026-01-03T00:00:00Z"],
            ],
        ]
        let page1Data = try JSONSerialization.data(withJSONObject: page1)
        let page2Data = try JSONSerialization.data(withJSONObject: page2)

        let transport = MockTransport { request in
            let urlString = request.url!.absoluteString
            if urlString.contains("page=2") {
                return (page2Data, pageResponse(url: request.url!))
            }
            let link = "<https://3.basecampapi.com/999999999/reports/users/progress/456.json?page=2>; rel=\"next\""
            return (page1Data, pageResponse(url: request.url!, link: link))
        }

        let account = makeTestAccountClient(transport: transport)
        let result = try await account.reports.personProgress(
            personId: 456, options: PersonProgressReportOptions(maxItems: 3)
        )

        XCTAssertEqual(result.events.count, 3)
        XCTAssertFalse(result.events.meta.truncated,
                       "Cap met exactly with no next Link — nothing was truncated")
    }

    func testMaxItemsExactBoundaryOnFirstPage() async throws {
        // Pins the already-precise first-page early return: exactly maxItems
        // items with a next Link -> truncated true; without one -> false.
        // Either way the next page must never be fetched.
        let pageData = projectPageData(ids: [1, 2, 3])

        let linkedTransport = MockTransport { request in
            (pageData, pageResponse(url: request.url!, link: nextProjectsLink(page: 2)))
        }
        let linked = try await makeTestAccountClient(transport: linkedTransport)
            .projects.list(options: ListProjectOptions(maxItems: 3))
        XCTAssertEqual(linked.count, 3)
        XCTAssertTrue(linked.meta.truncated,
                      "Next Link on the first page means more results exist")
        XCTAssertEqual(linkedTransport.requests.count, 1,
                       "Page 2 must not be fetched once the cap is satisfied")

        let unlinkedTransport = MockTransport { request in
            (pageData, pageResponse(url: request.url!))
        }
        let unlinked = try await makeTestAccountClient(transport: unlinkedTransport)
            .projects.list(options: ListProjectOptions(maxItems: 3))
        XCTAssertEqual(unlinked.count, 3)
        XCTAssertFalse(unlinked.meta.truncated,
                       "Cap met exactly with no next Link — nothing was truncated")
        XCTAssertEqual(unlinkedTransport.requests.count, 1)
    }

    // MARK: - Empty First Page with Link Header

    func testEmptyFirstPageWithLinkHeader() async throws {
        let emptyArray = try JSONSerialization.data(withJSONObject: [] as [Any])

        let transport = MockTransport { request in
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1",
                headerFields: [
                    "Link": "<https://3.basecampapi.com/999999999/projects.json?page=2>; rel=\"next\"",
                    "X-Total-Count": "0",
                ]
            )!
            return (emptyArray, response)
        }

        let account = makeTestAccountClient(transport: transport)
        let result: ListResult<Project> = try await account.projects.list()

        // Empty first page but Link header exists — pagination should still work
        // The SDK will follow the link, get another empty page, etc.
        // Key thing: it shouldn't crash
        XCTAssertEqual(result.meta.totalCount, 0)
    }
}

// MARK: - Page-building helpers (free functions: safe to use in @Sendable transport closures)

/// Builds a JSON array of minimal Project payloads with the given ids.
private func projectPageData(ids: [Int]) -> Data {
    let items = ids.map { id in
        ["id": id, "name": "Project \(id)", "status": "active",
         "app_url": "https://3.basecamp.com/1/projects/\(id)",
         "url": "https://3.basecampapi.com/1/projects/\(id).json",
         "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"] as [String: Any]
    }
    return try! JSONSerialization.data(withJSONObject: items)
}

/// Builds a 200 response, optionally carrying a next `Link` header.
private func pageResponse(url: URL, link: String? = nil, totalCount: Int = 3) -> HTTPURLResponse {
    var headers = ["X-Total-Count": String(totalCount)]
    if let link {
        headers["Link"] = link
    }
    return HTTPURLResponse(url: url, statusCode: 200, httpVersion: "HTTP/1.1", headerFields: headers)!
}

/// A `rel="next"` Link header pointing at the given projects page.
private func nextProjectsLink(page: Int) -> String {
    "<https://3.basecampapi.com/999999999/projects.json?page=\(page)>; rel=\"next\""
}
