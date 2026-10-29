import XCTest

@testable import Basecamp

/// A hill chart's progress updates (BC3 #13620, and posting one, BC3 #14214). Bodies are the shared
/// fixtures in `spec/fixtures/hill_charts/`: bc3's documented examples, plus
/// one dot whose todolist is gone, read relative to this file.
final class HillChartVersionTests: XCTestCase {

    private func fixture(_ name: String) throws -> Data {
        let repoRoot = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        return try Data(
            contentsOf: repoRoot.appendingPathComponent("spec/fixtures/hill_charts/\(name).json"))
    }

    func testGetCarriesVersionsUrl() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("get"))
        let account = makeTestAccountClient(transport: transport)

        let chart = try await account.hillCharts.get(todosetId: 1069479829)

        XCTAssertEqual(
            chart.versionsUrl,
            "https://3.basecampapi.com/195539477/buckets/2085958505/todosets/1069479829/hills/versions.json")
    }

    func testListVersionsDecodesTheDocumentedExample() async throws {
        let transport = MockTransport(
            statusCode: 200, data: try fixture("versions"), headers: ["X-Total-Count": "1"])
        let account = makeTestAccountClient(transport: transport)

        let versions = try await account.hillCharts.listVersions(todosetId: 1069479829)

        XCTAssertEqual(transport.lastRequest?.request.httpMethod, "GET")
        XCTAssertEqual(
            transport.lastRequest?.request.url?.path, "/999999999/todosets/1069479829/hills/versions.json")
        XCTAssertEqual(versions.meta.totalCount, 1)
        XCTAssertEqual(versions.count, 1)
        let version = try XCTUnwrap(versions.first)
        XCTAssertEqual(version.id, 1069480194)
        XCTAssertEqual(version.type, "Hill::Version")
        XCTAssertTrue(version.descriptionAttachments.isEmpty)
        XCTAssertEqual(version.dots.first?.id, 1069479863)
        XCTAssertEqual(version.dots.first?.position, 40)
    }

    func testGetVersionDecodesADotWhoseTodolistIsGone() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("version_todolist_gone"))
        let account = makeTestAccountClient(transport: transport)

        let version = try await account.hillCharts.getVersion(todosetId: 1069479829, versionId: 1069480194)

        XCTAssertEqual(
            transport.lastRequest?.request.url?.path, "/999999999/todosets/1069479829/hills/versions/1069480194")
        XCTAssertEqual(version.creator.name, "Victor Cooper")
        XCTAssertEqual(version.dots.count, 2)
        let gone = version.dots[1]
        XCTAssertNil(gone.id)
        XCTAssertNil(gone.url)
        XCTAssertNil(gone.appUrl)
        XCTAssertEqual(gone.label, "Old research list")
    }

    func testGetVersionNotFound() async throws {
        let transport = MockTransport(statusCode: 404, data: Data(#"{"error":"Not found"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.hillCharts.getVersion(todosetId: 1, versionId: 999)
            XCTFail("Expected a not-found error")
        } catch let error as BasecampError {
            guard case .notFound = error else { return XCTFail("Expected .notFound, got \(error)") }
        }
    }

    func testCreateVersionPostsTheDocumentedRequest() async throws {
        let transport = MockTransport(statusCode: 201, data: try fixture("create-version"))
        let account = makeTestAccountClient(transport: transport)

        let version = try await account.hillCharts.createVersion(
            todosetId: 1069479829,
            req: CreateHillChartVersionRequest(
                description: "<div>Research is done. Building starts Monday.</div>",
                dots: [HillChartDotPosition(id: 1069479863, position: 55)],
                subscriptions: [1049715913]))

        let request = try XCTUnwrap(transport.lastRequest?.request)
        XCTAssertEqual(request.httpMethod, "POST")
        XCTAssertEqual(request.url?.path, "/999999999/todosets/1069479829/hills/versions.json")
        let sent = try JSONSerialization.jsonObject(with: XCTUnwrap(request.httpBody)) as? NSDictionary
        let expected = try JSONSerialization.jsonObject(with: fixture("create-version-request")) as? NSDictionary
        XCTAssertEqual(sent, expected)
        XCTAssertEqual(version.id, 1069480194)
        XCTAssertEqual(version.dots.first?.position, 55)
    }

    func testCreateVersionWithNothingPostsAnEmptyObject() async throws {
        let transport = MockTransport(statusCode: 201, data: try fixture("create-version"))
        let account = makeTestAccountClient(transport: transport)

        _ = try await account.hillCharts.createVersion(todosetId: 1069479829, req: CreateHillChartVersionRequest())

        let body = try XCTUnwrap(transport.lastRequest?.request.httpBody)
        let sent = try JSONSerialization.jsonObject(with: body) as? NSDictionary
        XCTAssertEqual(sent, NSDictionary())
    }

    func testCreateVersionValidationError() async throws {
        let transport = MockTransport(
            statusCode: 422, data: Data(#"{"error":"Not tracked on the hill chart: 1069479511"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.hillCharts.createVersion(
                todosetId: 1069479829,
                req: CreateHillChartVersionRequest(dots: [HillChartDotPosition(id: 1069479511, position: 50)]))
            XCTFail("Expected a validation error")
        } catch let error as BasecampError {
            guard case .validation(let message, _, _, _, _) = error else {
                return XCTFail("Expected .validation, got \(error)")
            }
            XCTAssertEqual(message, "Not tracked on the hill chart: 1069479511")
        }
    }
}
