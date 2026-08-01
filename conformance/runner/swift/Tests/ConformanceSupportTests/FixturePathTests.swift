import XCTest

@testable import ConformanceSupport

/// The implicit path invariant. The scripted transport answers any URL, so
/// without this an operation pointed at the wrong endpoint still consumes the
/// queued responses and passes its retry, status and auth assertions.
final class FixturePathTests: XCTestCase {
    func testSubstitutesEveryPlaceholder() {
        XCTAssertEqual(
            .rendered("/buckets/456/todosets/700/todos.json"),
            renderFixturePath(
                "/buckets/{bucketId}/todosets/{todosetId}/todos.json",
                ["bucketId": "456", "todosetId": "700"])
        )
    }

    func testTemplateWithoutPlaceholdersIsUnchanged() {
        XCTAssertEqual(.rendered("/projects.json"), renderFixturePath("/projects.json", [:]))
    }

    func testMissingParameterIsNamedRatherThanLeftToMismatch() {
        XCTAssertEqual(
            .unsubstituted("bucketId"),
            renderFixturePath("/buckets/{bucketId}/webhooks.json", [:])
        )
    }

    func testExtraParametersAreIgnored() {
        XCTAssertEqual(
            .rendered("/todos/456"),
            renderFixturePath("/todos/{todoId}", ["todoId": "456", "unused": "1"])
        )
    }

    func testAccountRelativeAndAbsoluteFormsBothMatch() {
        // Most fixtures state the account-relative path and the SDK prefixes
        // the account id; the download fixtures state it already absolute.
        XCTAssertTrue(requestPathMatches(
            "/999/projects.json", fixturePath: "/projects.json", accountID: "999"))
        XCTAssertTrue(requestPathMatches(
            "/999999999/blobs/abcd/download/doc.pdf",
            fixturePath: "/999999999/blobs/abcd/download/doc.pdf", accountID: "999"))
    }

    func testASiblingEndpointDoesNotMatch() {
        XCTAssertFalse(requestPathMatches(
            "/999/todos.json", fixturePath: "/projects.json", accountID: "999"))
    }

    func testSuffixCollisionDoesNotMatch() {
        // The reason this is an equality test and not `hasSuffix`: an operation
        // that hit /999/my/projects.json instead of /999/projects.json would
        // otherwise pass, and both endpoints exist.
        XCTAssertFalse(requestPathMatches(
            "/999/my/projects.json", fixturePath: "/projects.json", accountID: "999"))
    }

    func testWrongAccountDoesNotMatch() {
        XCTAssertFalse(requestPathMatches(
            "/111/projects.json", fixturePath: "/projects.json", accountID: "999"))
    }

    func testUnsubstitutedTemplateCannotMatchARealPath() {
        XCTAssertFalse(requestPathMatches(
            "/999/buckets/456/webhooks.json",
            fixturePath: "/buckets/{bucketId}/webhooks.json", accountID: "999"))
    }
}
