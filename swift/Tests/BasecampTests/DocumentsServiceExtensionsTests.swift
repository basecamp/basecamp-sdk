import XCTest

@testable import Basecamp

/// Thread-safe capture of requests seen by the mock transport.
private final class DocumentRequestLog: @unchecked Sendable {
    private let lock = NSLock()
    private var _methods: [String] = []
    private var _putBody: [String: Any]?

    var methods: [String] { lock.withLock { _methods } }
    var putBody: [String: Any]? { lock.withLock { _putBody } }

    func record(_ request: URLRequest) {
        lock.withLock {
            _methods.append(request.httpMethod ?? "?")
            if request.httpMethod == "PUT",
                let data = request.httpBody ?? request.documentBodyStreamData()
            {
                _putBody = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
            }
        }
    }
}

extension URLRequest {
    /// URLSession moves httpBody into a stream in some paths; drain it if needed.
    fileprivate func documentBodyStreamData() -> Data? {
        guard let stream = httpBodyStream else { return nil }
        stream.open()
        defer { stream.close() }
        var data = Data()
        let bufferSize = 4096
        let buffer = UnsafeMutablePointer<UInt8>.allocate(capacity: bufferSize)
        defer { buffer.deallocate() }
        while stream.hasBytesAvailable {
            let read = stream.read(buffer, maxLength: bufferSize)
            if read <= 0 { break }
            data.append(buffer, count: read)
        }
        return data
    }
}

private final class DocumentOperationRecorder: BasecampHooks, @unchecked Sendable {
    private let lock = NSLock()
    private var _operations: [String] = []

    var operations: [String] { lock.withLock { _operations } }

    func onOperationStart(_ info: OperationInfo) {
        lock.withLock { _operations.append(info.operation) }
    }
}

/// Full document JSON on wire (snake_case) keys, with both writable fields —
/// `title` and `content` — populated.
private func fullDocumentJSON(id: Int = 42) -> [String: Any] {
    [
        "id": id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-01-01T00:00:00Z",
        "title": "Kickoff notes",
        "inherits_status": true,
        "type": "Document",
        "url": "https://3.basecampapi.com/999999999/buckets/1/documents/\(id).json",
        "app_url": "https://3.basecamp.com/999999999/buckets/1/documents/\(id)",
        "parent": [
            "id": 2, "title": "Docs & Files", "type": "Vault",
            "url": "https://3.basecampapi.com/999999999/buckets/1/vaults/2.json",
            "app_url": "https://3.basecamp.com/999999999/buckets/1/vaults/2",
        ] as [String: Any],
        "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
        "creator": ["id": 1, "name": "Test User"] as [String: Any],
        "content_attachments": [],
        "content": "<p>From the kickoff</p>",
        "position": 1,
    ]
}

/// The merge-safe `update` / read-modify-write `edit` composites and the raw
/// `replace` they are built on.
///
/// `PUT /documents/{id}` is a full replace: BC3 rebuilds the Document from only
/// the permitted params, so a sparse PUT that omits `content` erases it and one
/// that omits `title` leaves the document reading back as "Untitled". Neither
/// omission is a 422 — both are a 200 that quietly clears. That is why the
/// composites always send BOTH writable fields, empties included: on this
/// endpoint `""` is how a clear is expressed, and omission is indistinguishable
/// from an accident.
final class DocumentsServiceExtensionsTests: XCTestCase {

    private func makeDocumentsClient(
        log: DocumentRequestLog,
        hooks: (any BasecampHooks)? = nil
    ) throws -> AccountClient {
        let documentData = try JSONSerialization.data(withJSONObject: fullDocumentJSON())
        let transport = MockTransport { request in
            log.record(request)
            return (
                documentData,
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "application/json"]
                )
            )
        }
        return makeTestAccountClient(transport: transport, hooks: hooks)
    }

    // MARK: - update (merge-safe)

    /// BC3 can never render a blank title (`Document#title` is
    /// `super.presence || "Untitled"`), so `""` on a 2xx read is malformed.
    /// `Codable` already refuses an absent or null title because the model
    /// field is non-optional; `""` decodes fine and needs the hand-written
    /// check. The ordering is what matters: no PUT.
    func testUpdate_refusesABlankTitle() async throws {
        let log = DocumentRequestLog()
        var blank = fullDocumentJSON()
        blank["title"] = "   "
        let blankData = try JSONSerialization.data(withJSONObject: blank)
        let transport = MockTransport { request in
            log.record(request)
            return (
                blankData,
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "application/json"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.documents.update(
                documentId: 42, req: UpdateDocumentRequest(content: "<p>New body.</p>"))
            XCTFail("expected the call to fail, but it succeeded")
        } catch let error as BasecampError {
            guard case .api(_, let httpStatus, let hint, _, _) = error else {
                return XCTFail("expected .api, got \(error)")
            }
            XCTAssertNil(httpStatus, "a malformed 2xx body carries no status")
            XCTAssertNotNil(hint, "expected a hint naming the escape hatch")
        }

        XCTAssertEqual(log.methods, ["GET"], "the guard must fire before the PUT")
    }

    /// The composite restates the base layer's decode failure with its own
    /// escape hatch, and the restatement is still that decode failure: it
    /// carries `decodeFailure` forward (#750).
    ///
    /// Dropping it would say "this was not a malformed body after all" to
    /// everything downstream — the conformance runner included, where the answer
    /// decides between "repair the fixture body" and "hand this to the
    /// assertions". Read through the property, not by matching the case, which is
    /// what the migration entry steers callers to do.
    ///
    /// The blank-title refusal above is the control: it is a hand-written guard
    /// on a body that decoded fine, so its marker is nil.
    func testUpdate_restatementKeepsTheDecodeFailureMarker() async throws {
        let log = DocumentRequestLog()
        var wrongType = fullDocumentJSON()
        wrongType["title"] = 42
        let wrongTypeData = try JSONSerialization.data(withJSONObject: wrongType)
        let transport = MockTransport { request in
            log.record(request)
            return (
                wrongTypeData,
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "application/json"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.documents.update(
                documentId: 42, req: UpdateDocumentRequest(content: "<p>New body.</p>"))
            XCTFail("expected the call to fail, but it succeeded")
        } catch let error as BasecampError {
            XCTAssertNotNil(
                error.decodeFailure,
                "the composite's restatement must keep the marker")
            XCTAssertNotNil(error.hint, "and must add its own escape hatch")
        }

        XCTAssertEqual(log.methods, ["GET"], "the guard must fire before the PUT")
    }

    func testUpdate_blankTitleRefusalCarriesNoDecodeFailureMarker() async throws {
        let log = DocumentRequestLog()
        var blank = fullDocumentJSON()
        blank["title"] = "   "
        let blankData = try JSONSerialization.data(withJSONObject: blank)
        let transport = MockTransport { request in
            log.record(request)
            return (
                blankData,
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "application/json"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.documents.update(
                documentId: 42, req: UpdateDocumentRequest(content: "<p>New body.</p>"))
            XCTFail("expected the call to fail, but it succeeded")
        } catch let error as BasecampError {
            XCTAssertNil(
                error.decodeFailure,
                "a hand-written guard on a body that decoded is not a decode failure")
        }
    }

    func testUpdate_mergesUnsetFields() async throws {
        let log = DocumentRequestLog()
        let account = try makeDocumentsClient(log: log)

        let document = try await account.documents.update(
            documentId: 42, req: UpdateDocumentRequest(title: "Kickoff notes, revised"))

        XCTAssertEqual(document.id, 42)
        XCTAssertEqual(log.methods, ["GET", "PUT"])
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["title"] as? String, "Kickoff notes, revised")
        // content was never named, so the GET's value is written straight back
        // rather than left to the server's clear-by-default.
        XCTAssertEqual(body["content"] as? String, "<p>From the kickoff</p>")
    }

    func testUpdate_explicitEmptyStringClears() async throws {
        let log = DocumentRequestLog()
        let account = try makeDocumentsClient(log: log)

        _ = try await account.documents.update(documentId: 42, req: UpdateDocumentRequest(content: ""))

        let body = try XCTUnwrap(log.putBody)
        // An explicitly-passed "" is a set, not an unset: present and empty.
        XCTAssertNotNil(body["content"], "an explicit clear must be sent, not omitted")
        XCTAssertEqual(body["content"] as? String, "")
        XCTAssertEqual(body["title"] as? String, "Kickoff notes")
    }

    func testUpdate_hooksObserveGetThenReplace() async throws {
        let log = DocumentRequestLog()
        let recorder = DocumentOperationRecorder()
        let account = try makeDocumentsClient(log: log, hooks: recorder)

        _ = try await account.documents.update(
            documentId: 42, req: UpdateDocumentRequest(title: "observed"))

        // The composite is built from the public get/replace, so hooks see the
        // two wire operations, not a synthetic composite.
        XCTAssertEqual(recorder.operations, ["GetDocument", "ReplaceDocument"])
    }

    // MARK: - edit (read-modify-write closure)

    func testEdit_putsFullStateBack() async throws {
        let log = DocumentRequestLog()
        let account = try makeDocumentsClient(log: log)

        let document = try await account.documents.edit(documentId: 42) { fields in
            XCTAssertEqual(fields.title, "Kickoff notes")
            XCTAssertEqual(fields.content, "<p>From the kickoff</p>")
            fields.title = "🚨 " + fields.title
        }

        XCTAssertEqual(document.id, 42)
        XCTAssertEqual(log.methods, ["GET", "PUT"])
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["title"] as? String, "🚨 Kickoff notes")
        XCTAssertEqual(body["content"] as? String, "<p>From the kickoff</p>")
    }

    func testEdit_clearsContentPresentAndEmpty() async throws {
        let log = DocumentRequestLog()
        let account = try makeDocumentsClient(log: log)

        _ = try await account.documents.edit(documentId: 42) { fields in
            fields.content = ""
        }

        let body = try XCTUnwrap(log.putBody)
        // Clearing on a full-replace endpoint is an explicit "": never JSON
        // null (SPEC §18 body compaction), and never by omission, which would
        // leave the clear to the server and read as an accident.
        XCTAssertNotNil(body["content"], "a cleared content must be sent present-and-empty")
        XCTAssertEqual(body["content"] as? String, "")
        XCTAssertEqual(body["title"] as? String, "Kickoff notes")
    }

    func testEdit_closureErrorAbortsWithoutPut() async throws {
        struct Abort: Error {}
        let log = DocumentRequestLog()
        let account = try makeDocumentsClient(log: log)

        do {
            _ = try await account.documents.edit(documentId: 42) { fields in
                fields.title = "never written"
                throw Abort()
            }
            XCTFail("expected the closure error to propagate")
        } catch is Abort {
            // expected
        }

        XCTAssertEqual(log.methods, ["GET"], "no PUT after a closure error")
    }

    // MARK: - replace (server-native verbatim PUT)

    func testReplace_sendsSparseVerbatimWithNoGet() async throws {
        let log = DocumentRequestLog()
        let recorder = DocumentOperationRecorder()
        let account = try makeDocumentsClient(log: log, hooks: recorder)

        let document = try await account.documents.replace(
            documentId: 42, req: ReplaceDocumentRequest(title: "the whole new document"))

        XCTAssertEqual(document.id, 42)
        XCTAssertEqual(log.methods, ["PUT"], "replace must not GET")
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["title"] as? String, "the whole new document")
        // The raw path is destructive by design: what the caller left out stays
        // out, and the server clears it.
        XCTAssertNil(body["content"], "content must be omitted from a sparse replace")
        XCTAssertEqual(recorder.operations, ["ReplaceDocument"])
    }
}
