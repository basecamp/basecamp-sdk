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
            if case .api(let message, _, _, _) = error {
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
