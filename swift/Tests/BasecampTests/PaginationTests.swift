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
}
