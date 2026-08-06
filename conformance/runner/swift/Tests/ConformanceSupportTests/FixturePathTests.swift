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

/// Account scoping and Link parsing, the two rules that decide WHERE a hop was
/// allowed to go.
final class AccountScopingTests: XCTestCase {
    func testAccountRelativeFixturePathsAreNotTreatedAsScoped() {
        XCTAssertFalse(fixtureIsAccountScoped("/projects.json"))
        XCTAssertFalse(fixtureIsAccountScoped("/buckets/456/webhooks.json"))
        XCTAssertFalse(fixtureIsAccountScoped("/my/notes.json"))
    }

    func testDownloadFixturePathsAreTreatedAsScoped() {
        // The only shape that carries its own account segment; the SDK dials
        // these literally.
        XCTAssertTrue(fixtureIsAccountScoped("/999999999/blobs/abcd1234/download/logo.png"))
    }

    func testANumericSegmentMustBeWholeAndLeading() {
        XCTAssertFalse(fixtureIsAccountScoped("/999projects.json"))
        XCTAssertFalse(fixtureIsAccountScoped("/todos/456"))
        XCTAssertFalse(fixtureIsAccountScoped(""))
        XCTAssertFalse(fixtureIsAccountScoped("999/projects.json"))
    }

    func testAnUnscopedRequestFailsAnAccountRelativeFixture() {
        // Accepting either form let a dropped account prefix pass: the
        // transport serves any URL, so nothing else would have noticed.
        XCTAssertFalse(requestPathMatches(
            "/projects.json", fixturePath: "/projects.json", accountID: "999"))
        XCTAssertTrue(requestPathMatches(
            "/999/projects.json", fixturePath: "/projects.json", accountID: "999"))
    }

    func testAScopedFixtureRequiresTheLiteralPath() {
        let blob = "/999999999/blobs/abcd1234/download/logo.png"
        XCTAssertTrue(requestPathMatches(blob, fixturePath: blob, accountID: "999"))
        XCTAssertFalse(requestPathMatches("/999" + blob, fixturePath: blob, accountID: "999"))
    }

    func testExpectedRequestPathReportsTheFormThatWasRequired() {
        XCTAssertEqual("/999/projects.json", expectedRequestPath("/projects.json", accountID: "999"))
        XCTAssertEqual("/9/blobs/x", expectedRequestPath("/9/blobs/x", accountID: "999"))
    }
}

final class NextLinkTests: XCTestCase {
    func testExtractsARelativeNextTarget() {
        XCTAssertEqual("/projects.json?page=2", nextLinkTarget("</projects.json?page=2>; rel=\"next\""))
    }

    func testExtractsAnAbsoluteNextTarget() {
        XCTAssertEqual(
            "https://evil.example.com/projects.json?page=2",
            nextLinkTarget("<https://evil.example.com/projects.json?page=2>; rel=\"next\"")
        )
    }

    func testPicksNextOutOfSeveralLinks() {
        XCTAssertEqual(
            "/projects.json?page=3",
            nextLinkTarget("</projects.json?page=1>; rel=\"prev\", </projects.json?page=3>; rel=\"next\"")
        )
    }

    func testToleratesUnquotedRelAndExtraSpacing() {
        XCTAssertEqual("/p?page=2", nextLinkTarget("</p?page=2>;rel=next"))
        XCTAssertEqual("/p?page=2", nextLinkTarget("</p?page=2> ;  rel = \"NEXT\""))
    }

    func testHeaderWithoutANextRelYieldsNil() {
        XCTAssertNil(nextLinkTarget("</projects.json?page=1>; rel=\"prev\""))
        XCTAssertNil(nextLinkTarget(""))
        XCTAssertNil(nextLinkTarget("garbage"))
    }

    /// The adversarial headers from conformance/tests/pagination.json. This
    /// helper decides which requests the LINK and PATH invariants govern, so
    /// disagreeing with the SDKs' parser here fails a correct SDK — which is
    /// how these three arrived: the runner read `<>` as a target of "" and
    /// `>x</p>` as no target at all, and reported both as SDK bugs.
    func testAgreesWithTheSDKParserOnMalformedParts() {
        // An empty <> names no page: skip the part, keep scanning.
        XCTAssertEqual(
            "/projects.json?page=2",
            nextLinkTarget("<>; rel=\"next\", </projects.json?page=2>; rel=\"next\"")
        )
        // The ">" delimiting the URL is the first one AFTER the "<", not the
        // first one in the string.
        XCTAssertEqual(
            "/projects.json?page=2",
            nextLinkTarget(">x</projects.json?page=2>; rel=\"next\"")
        )
        // A "<" that never closes yields no target rather than the rest of
        // the header.
        XCTAssertNil(nextLinkTarget("<; rel=\"next\""))
        // An empty pair with nothing after it is not a target of "".
        XCTAssertNil(nextLinkTarget("<>; rel=\"next\""))
    }
}
