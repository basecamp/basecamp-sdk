import XCTest
@testable import Basecamp

/// Tests for the generated `GaugesService`: all seven wire operations, each
/// with a happy path and a mapped-error path.
///
/// Swift is the SDK that decodes gauges into *typed* models, so these tests
/// carry more than routing: they pin the `Gauge` / `GaugeNeedle` Codable
/// decode against the projections bc3 actually emits (`spec/fixtures/gauges/`),
/// including the singleton gauge URL — a gauge id never appears in its own
/// `url` — and the `previous_needle_position: null` a project with a single
/// needle sends.
///
/// Two routes deliberately carry no `.json` suffix (`/gauge_needles/{id}`);
/// the assertions below pin that explicitly rather than by omission.
final class GaugesServiceTests: XCTestCase {

    // MARK: - Fixtures
    //
    // Faithful to spec/fixtures/gauges/get.json and needle_get.json, trimmed to
    // the members these tests read plus every required key of the model graph.

    private static let bucketId = 2085958500
    private static let gaugeId = 1069479800
    private static let needleId = 1069479850

    private func attachmentJSON(floatSpelledWidth: Bool = true) -> [String: Any] {
        [
            "id": 1069480040,
            "sgid": "BAh7CEkiCGdpZAY6BkVUSSIs--gauge001",
            "filename": "burndown.png",
            "content_type": "image/png",
            "byte_size": 40960,
            "download_url": "https://3.basecampapi.com/999999999/blobs/gauge001/download/burndown.png",
            // bc3 sends image dimensions float-spelled; the model types them Int32?.
            "width": floatSpelledWidth ? 1024.0 : 1024,
            "height": 768,
            "previewable": true,
            "preview_url": "https://3.basecampapi.com/999999999/blobs/gauge001/previews/burndown.png",
            "thumbnail_url": "https://3.basecampapi.com/999999999/blobs/gauge001/thumbnails/burndown.png",
        ]
    }

    private func bucketJSON() -> [String: Any] {
        ["id": Self.bucketId, "name": "The Leto Laptop", "type": "Project"]
    }

    private func creatorJSON() -> [String: Any] {
        ["id": 1049715915, "name": "Victor Cooper", "personable_type": "User"]
    }

    /// A gauge is a project singleton: `url` names the project, never the
    /// gauge id, and there is no `/buckets/{id}/gauges/...` route in bc3.
    private func gaugeJSON(
        id: Int = GaugesServiceTests.gaugeId,
        bucketId: Int = GaugesServiceTests.bucketId,
        previousNeedlePosition: Any = 45
    ) -> [String: Any] {
        [
            "id": id,
            "status": "active",
            "visible_to_clients": false,
            "created_at": "2022-11-22T08:40:00.000Z",
            "updated_at": "2022-11-28T14:12:00.000Z",
            "title": "How far along are we?",
            "inherits_status": true,
            "type": "Gauge",
            "url": "https://3.basecampapi.com/999999999/projects/\(bucketId)/gauge.json",
            "app_url": "https://3.basecamp.com/999999999/projects/\(bucketId)/gauge",
            "bookmark_url": "https://3.basecampapi.com/999999999/my/bookmarks/gauge\(id).json",
            "bucket": bucketJSON(),
            "creator": creatorJSON(),
            "description": "<div>Shipped the new onboarding flow.</div>",
            "description_attachments": [attachmentJSON()],
            "enabled": true,
            "last_needle_color": "green",
            "last_needle_position": 72,
            "previous_needle_position": previousNeedlePosition,
        ]
    }

    /// A needle is a full recording: it carries the subscription/comments/boosts
    /// slots and a `parent` pointing back at the singleton gauge — none of
    /// which a `Gauge` itself carries.
    private func needleJSON(
        id: Int = GaugesServiceTests.needleId,
        bucketId: Int = GaugesServiceTests.bucketId,
        position: Int = 72
    ) -> [String: Any] {
        [
            "id": id,
            "status": "active",
            "visible_to_clients": false,
            "created_at": "2022-11-28T14:12:00.000Z",
            "updated_at": "2022-11-28T14:12:00.000Z",
            "title": "Moved the needle",
            "inherits_status": true,
            "type": "Gauge::Needle",
            "url": "https://3.basecampapi.com/999999999/projects/\(bucketId)/gauge/needles/\(id).json",
            "app_url": "https://3.basecamp.com/999999999/projects/\(bucketId)/gauge/needles/\(id)",
            "bookmark_url": "https://3.basecampapi.com/999999999/my/bookmarks/needle\(id).json",
            "subscription_url":
                "https://3.basecampapi.com/999999999/buckets/\(bucketId)/recordings/\(id)/subscription.json",
            "comments_count": 2,
            "comments_url":
                "https://3.basecampapi.com/999999999/buckets/\(bucketId)/recordings/\(id)/comments.json",
            "boosts_count": 3,
            "boosts_url":
                "https://3.basecampapi.com/999999999/buckets/\(bucketId)/recordings/\(id)/boosts.json",
            "parent": [
                "id": GaugesServiceTests.gaugeId,
                "title": "How far along are we?",
                "type": "Gauge",
                "url": "https://3.basecampapi.com/999999999/projects/\(bucketId)/gauge.json",
                "app_url": "https://3.basecamp.com/999999999/projects/\(bucketId)/gauge",
            ] as [String: Any],
            "bucket": bucketJSON(),
            "creator": creatorJSON(),
            "description": "<div>Shipped the new onboarding flow.</div>",
            "description_attachments": [attachmentJSON()],
            "color": "green",
            "position": position,
        ]
    }

    // MARK: - Request introspection helpers

    private func sentComponents(_ transport: MockTransport) -> URLComponents {
        URLComponents(string: transport.lastRequest!.request.url!.absoluteString)!
    }

    private func sentPath(_ transport: MockTransport) -> String {
        sentComponents(transport).path
    }

    private func sentQuery(_ transport: MockTransport) -> [String: String] {
        var query: [String: String] = [:]
        for item in sentComponents(transport).queryItems ?? [] {
            query[item.name] = item.value
        }
        return query
    }

    private func sentBody(_ transport: MockTransport) throws -> [String: Any] {
        let data = try XCTUnwrap(transport.lastRequest?.request.httpBody, "no request body was sent")
        return try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
    }

    private func nextLink(_ path: String, page: Int) -> String {
        "<https://3.basecampapi.com/999999999\(path)?page=\(page)>; rel=\"next\""
    }

    // MARK: - ListGauges (GET /reports/gauges.json)

    func testListGaugesDecodesTypedGauges() async throws {
        let data = try JSONSerialization.data(withJSONObject: [gaugeJSON()])
        let transport = MockTransport(statusCode: 200, data: data, headers: ["X-Total-Count": "1"])
        let account = makeTestAccountClient(transport: transport)

        let gauges = try await account.gauges.listGauges()

        XCTAssertEqual(sentPath(transport), "/999999999/reports/gauges.json")
        XCTAssertEqual(transport.lastRequest!.request.httpMethod, "GET")
        XCTAssertEqual(gauges.count, 1)
        XCTAssertEqual(gauges.meta.totalCount, 1)

        let gauge = gauges[0]
        XCTAssertEqual(gauge.id, Self.gaugeId)
        XCTAssertEqual(gauge.title, "How far along are we?")
        XCTAssertEqual(gauge.type, "Gauge")
        XCTAssertEqual(gauge.enabled, true)
        XCTAssertEqual(gauge.lastNeedleColor, "green")
        XCTAssertEqual(gauge.lastNeedlePosition, 72)
        XCTAssertEqual(gauge.previousNeedlePosition, 45)
        XCTAssertEqual(gauge.status, "active")
        XCTAssertEqual(gauge.inheritsStatus, true)
        XCTAssertEqual(gauge.visibleToClients, false)
        XCTAssertEqual(gauge.createdAt, "2022-11-22T08:40:00.000Z")
        XCTAssertEqual(gauge.updatedAt, "2022-11-28T14:12:00.000Z")
        XCTAssertEqual(gauge.bucket?.id, Self.bucketId)
        XCTAssertEqual(gauge.bucket?.type, "Project")
        XCTAssertEqual(gauge.creator?.name, "Victor Cooper")
        XCTAssertEqual(gauge.descriptionAttachments?.count, 1)
        XCTAssertEqual(gauge.descriptionAttachments?.first?.filename, "burndown.png")
        // Float-spelled dimension folds into Int32.
        XCTAssertEqual(gauge.descriptionAttachments?.first?.width, 1024)

        // The singleton URL names the project, not the gauge — a
        // /buckets/{id}/gauges/{id} shape would be a route that does not exist.
        XCTAssertEqual(
            gauge.url, "https://3.basecampapi.com/999999999/projects/\(Self.bucketId)/gauge.json")
        XCTAssertFalse(gauge.url!.contains("\(Self.gaugeId)"), "gauge id must not appear in its own url")
    }

    /// bc3 sends `previous_needle_position: null` for a gauge with a single
    /// needle. The model types it `Int32?`, so an explicit null must decode to
    /// nil rather than throwing.
    func testListGaugesDecodesNullPreviousNeedlePosition() async throws {
        let data = try JSONSerialization.data(
            withJSONObject: [gaugeJSON(previousNeedlePosition: NSNull())])
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let gauges = try await account.gauges.listGauges()

        XCTAssertEqual(gauges.count, 1)
        XCTAssertNil(gauges[0].previousNeedlePosition)
        XCTAssertEqual(gauges[0].lastNeedlePosition, 72, "only the previous position is null")
    }

    func testListGaugesSendsBucketIdsQuery() async throws {
        let data = try JSONSerialization.data(withJSONObject: [gaugeJSON()])
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        _ = try await account.gauges.listGauges(
            options: ListGaugesGaugeOptions(bucketIds: "2085958500,2085958501"))

        XCTAssertEqual(sentPath(transport), "/999999999/reports/gauges.json")
        XCTAssertEqual(sentQuery(transport), ["bucket_ids": "2085958500,2085958501"])
    }

    /// SPEC section 8: a positive `page` selects exactly that page in exactly
    /// one request, and reports truncation from the `rel="next"` link it
    /// deliberately did not follow.
    func testListGaugesPageSelectsExactlyThatPage() async throws {
        let data = try JSONSerialization.data(
            withJSONObject: [gaugeJSON(id: 1), gaugeJSON(id: 2)])
        let transport = MockTransport(
            statusCode: 200, data: data,
            headers: ["Link": nextLink("/reports/gauges.json", page: 4)])
        let account = makeTestAccountClient(transport: transport)

        let gauges = try await account.gauges.listGauges(options: ListGaugesGaugeOptions(page: 3))

        XCTAssertEqual(transport.requests.count, 1, "a pinned page must not follow the Link chain")
        XCTAssertEqual(sentQuery(transport)["page"], "3")
        XCTAssertEqual(gauges.count, 2)
        XCTAssertTrue(
            gauges.meta.truncated, "the pinned page advertised a next page it did not follow")
    }

    func testListGaugesWithoutPageWalksTheLinkChain() async throws {
        let page1 = try JSONSerialization.data(withJSONObject: [gaugeJSON(id: 1)])
        let page2 = try JSONSerialization.data(withJSONObject: [gaugeJSON(id: 2)])
        let transport = MockTransport { request in
            let url = request.url!.absoluteString
            if url.contains("page=2") {
                return (page2, makeHTTPResponse(url: url, statusCode: 200))
            }
            XCTAssertFalse(url.contains("page="), "the first request must carry no page param")
            return (
                page1,
                makeHTTPResponse(
                    url: url, statusCode: 200,
                    headers: [
                        "Link": "<https://3.basecampapi.com/999999999/reports/gauges.json?page=2>; rel=\"next\""
                    ])
            )
        }
        let account = makeTestAccountClient(transport: transport)

        let gauges = try await account.gauges.listGauges()

        XCTAssertEqual(transport.requests.count, 2, "the walk should fetch both pages")
        XCTAssertEqual(gauges.map(\.id), [1, 2])
        XCTAssertFalse(gauges.meta.truncated, "the walk reached the last page")
    }

    func testListGaugesForbidden() async throws {
        let body = Data(#"{"error":"You are not authorized to view gauges"}"#.utf8)
        let transport = MockTransport(
            statusCode: 403, data: body, headers: ["X-Request-Id": "req-gauges-403"])
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.gauges.listGauges()
            XCTFail("Expected a forbidden error")
        } catch let error as BasecampError {
            guard case .forbidden = error else { return XCTFail("Expected .forbidden, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 403)
            XCTAssertEqual(error.exitCode, 4)
            XCTAssertEqual(error.message, "You are not authorized to view gauges")
            XCTAssertEqual(error.requestId, "req-gauges-403")
            XCTAssertFalse(error.isRetryable)
            XCTAssertNil(error.fieldErrors)
        }
    }

    // MARK: - ListGaugeNeedles (GET /projects/{projectId}/gauge/needles.json)

    func testListGaugeNeedlesDecodesTypedNeedles() async throws {
        let data = try JSONSerialization.data(withJSONObject: [needleJSON()])
        let transport = MockTransport(statusCode: 200, data: data, headers: ["X-Total-Count": "1"])
        let account = makeTestAccountClient(transport: transport)

        let needles = try await account.gauges.listGaugeNeedles(projectId: Self.bucketId)

        XCTAssertEqual(sentPath(transport), "/999999999/projects/\(Self.bucketId)/gauge/needles.json")
        XCTAssertEqual(transport.lastRequest!.request.httpMethod, "GET")
        XCTAssertEqual(needles.count, 1)
        XCTAssertEqual(needles.meta.totalCount, 1)

        let needle = needles[0]
        XCTAssertEqual(needle.id, Self.needleId)
        XCTAssertEqual(needle.title, "Moved the needle")
        XCTAssertEqual(needle.type, "Gauge::Needle")
        XCTAssertEqual(needle.color, "green")
        XCTAssertEqual(needle.position, 72)
        XCTAssertEqual(needle.commentsCount, 2)
        XCTAssertEqual(needle.boostsCount, 3)
        XCTAssertNotNil(needle.subscriptionUrl)
        XCTAssertNotNil(needle.commentsUrl)
        XCTAssertNotNil(needle.boostsUrl)
        XCTAssertEqual(needle.parent?.id, Self.gaugeId)
        XCTAssertEqual(needle.parent?.type, "Gauge")
        XCTAssertEqual(needle.bucket?.id, Self.bucketId)
        XCTAssertEqual(needle.creator?.name, "Victor Cooper")
        XCTAssertEqual(needle.descriptionAttachments.count, 1)
        XCTAssertEqual(needle.descriptionAttachments[0].contentType, "image/png")
        XCTAssertEqual(
            needle.url,
            "https://3.basecampapi.com/999999999/projects/\(Self.bucketId)/gauge/needles/\(Self.needleId).json")
    }

    /// SPEC section 8, needles half: one request, that page, truncation from
    /// the unfollowed link.
    func testListGaugeNeedlesPageSelectsExactlyThatPage() async throws {
        let data = try JSONSerialization.data(
            withJSONObject: [needleJSON(id: 11), needleJSON(id: 12)])
        let transport = MockTransport(
            statusCode: 200, data: data,
            headers: [
                "Link": nextLink("/projects/\(Self.bucketId)/gauge/needles.json", page: 3)
            ])
        let account = makeTestAccountClient(transport: transport)

        let needles = try await account.gauges.listGaugeNeedles(
            projectId: Self.bucketId, options: ListGaugeNeedlesGaugeOptions(page: 2))

        XCTAssertEqual(transport.requests.count, 1, "a pinned page must not follow the Link chain")
        XCTAssertEqual(sentQuery(transport)["page"], "2")
        XCTAssertEqual(needles.map(\.id), [11, 12])
        XCTAssertTrue(needles.meta.truncated)
    }

    func testListGaugeNeedlesPinnedFinalPageIsNotTruncated() async throws {
        let data = try JSONSerialization.data(withJSONObject: [needleJSON(id: 21)])
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let needles = try await account.gauges.listGaugeNeedles(
            projectId: Self.bucketId, options: ListGaugeNeedlesGaugeOptions(page: 9))

        XCTAssertEqual(transport.requests.count, 1)
        XCTAssertEqual(sentQuery(transport)["page"], "9")
        XCTAssertFalse(needles.meta.truncated, "the pinned page carried no next link")
    }

    func testListGaugeNeedlesWithoutPageWalksTheLinkChain() async throws {
        let page1 = try JSONSerialization.data(withJSONObject: [needleJSON(id: 31)])
        let page2 = try JSONSerialization.data(withJSONObject: [needleJSON(id: 32)])
        let bucketId = Self.bucketId
        let transport = MockTransport { request in
            let url = request.url!.absoluteString
            if url.contains("page=2") {
                return (page2, makeHTTPResponse(url: url, statusCode: 200))
            }
            XCTAssertFalse(url.contains("page="), "the first request must carry no page param")
            return (
                page1,
                makeHTTPResponse(
                    url: url, statusCode: 200,
                    headers: [
                        "Link":
                            "<https://3.basecampapi.com/999999999/projects/\(bucketId)/gauge/needles.json?page=2>; rel=\"next\""
                    ])
            )
        }
        let account = makeTestAccountClient(transport: transport)

        let needles = try await account.gauges.listGaugeNeedles(projectId: bucketId)

        XCTAssertEqual(transport.requests.count, 2)
        XCTAssertEqual(needles.map(\.id), [31, 32])
        XCTAssertFalse(needles.meta.truncated)
    }

    func testListGaugeNeedlesNotFound() async throws {
        let body = Data(#"{"error":"Project not found"}"#.utf8)
        let transport = MockTransport(statusCode: 404, data: body)
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.gauges.listGaugeNeedles(projectId: 404404)
            XCTFail("Expected a notFound error")
        } catch let error as BasecampError {
            guard case .notFound = error else { return XCTFail("Expected .notFound, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 404)
            XCTAssertEqual(error.exitCode, 2)
            XCTAssertEqual(error.message, "Project not found")
            XCTAssertFalse(error.isRetryable)
        }
    }

    // MARK: - GetGaugeNeedle (GET /gauge_needles/{needleId})

    /// The needle member routes carry NO `.json` suffix — deliberate in the
    /// spec. Asserted positively (exact path) and negatively (no suffix), so a
    /// regeneration that "helpfully" appends one fails here.
    func testGaugeNeedleGetsSuffixlessPathAndDecodes() async throws {
        let data = try JSONSerialization.data(withJSONObject: needleJSON())
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let needle = try await account.gauges.gaugeNeedle(needleId: Self.needleId)

        XCTAssertEqual(sentPath(transport), "/999999999/gauge_needles/\(Self.needleId)")
        XCTAssertFalse(
            sentPath(transport).hasSuffix(".json"),
            "GetGaugeNeedle is suffixless by design; got \(sentPath(transport))")
        XCTAssertEqual(transport.lastRequest!.request.httpMethod, "GET")
        XCTAssertNil(transport.lastRequest!.request.httpBody)

        XCTAssertEqual(needle.id, Self.needleId)
        XCTAssertEqual(needle.title, "Moved the needle")
        XCTAssertEqual(needle.position, 72)
        XCTAssertEqual(needle.color, "green")
        XCTAssertEqual(needle.parent?.title, "How far along are we?")
    }

    func testGaugeNeedleNotFound() async throws {
        let body = Data(#"{"error":"Needle not found"}"#.utf8)
        let transport = MockTransport(
            statusCode: 404, data: body, headers: ["X-Request-Id": "req-needle-404"])
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.gauges.gaugeNeedle(needleId: 999)
            XCTFail("Expected a notFound error")
        } catch let error as BasecampError {
            guard case .notFound = error else { return XCTFail("Expected .notFound, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 404)
            XCTAssertEqual(error.exitCode, 2)
            XCTAssertEqual(error.message, "Needle not found")
            XCTAssertEqual(error.requestId, "req-needle-404")
            XCTAssertFalse(error.isRetryable)
        }
    }

    // MARK: - CreateGaugeNeedle (POST /projects/{projectId}/gauge/needles.json)

    func testCreateGaugeNeedleEncodesWrappedBodyAndDecodes() async throws {
        let data = try JSONSerialization.data(withJSONObject: needleJSON())
        let transport = MockTransport(statusCode: 201, data: data)
        let account = makeTestAccountClient(transport: transport)

        let req = CreateGaugeNeedleRequest(
            gaugeNeedle: GaugeNeedlePayload(
                position: 72, color: "green", description: "<div>Moved the needle</div>"),
            notify: "custom",
            subscriptions: [1049715915]
        )
        let needle = try await account.gauges.createGaugeNeedle(projectId: Self.bucketId, req: req)

        XCTAssertEqual(sentPath(transport), "/999999999/projects/\(Self.bucketId)/gauge/needles.json")
        XCTAssertEqual(transport.lastRequest!.request.httpMethod, "POST")

        // The payload is wrapped: {"gauge_needle": {...}} with notify and
        // subscriptions as top-level siblings.
        let body = try sentBody(transport)
        XCTAssertEqual(Set(body.keys), ["gauge_needle", "notify", "subscriptions"])
        let payload = try XCTUnwrap(body["gauge_needle"] as? [String: Any])
        XCTAssertEqual(Set(payload.keys), ["position", "color", "description"])
        XCTAssertEqual(payload["position"] as? Int, 72)
        XCTAssertEqual(payload["color"] as? String, "green")
        XCTAssertEqual(payload["description"] as? String, "<div>Moved the needle</div>")
        XCTAssertEqual(body["notify"] as? String, "custom")
        XCTAssertEqual(body["subscriptions"] as? [Int], [1049715915])

        XCTAssertEqual(needle.id, Self.needleId)
        XCTAssertEqual(needle.position, 72)
    }

    /// The optional payload members are omitted, not sent as null: a bare
    /// position must encode to exactly `{"gauge_needle":{"position":30}}`.
    func testCreateGaugeNeedleOmitsUnsetPayloadMembers() async throws {
        let data = try JSONSerialization.data(withJSONObject: needleJSON(position: 30))
        let transport = MockTransport(statusCode: 201, data: data)
        let account = makeTestAccountClient(transport: transport)

        _ = try await account.gauges.createGaugeNeedle(
            projectId: Self.bucketId,
            req: CreateGaugeNeedleRequest(gaugeNeedle: GaugeNeedlePayload(position: 30))
        )

        let body = try sentBody(transport)
        XCTAssertEqual(Set(body.keys), ["gauge_needle"])
        let payload = try XCTUnwrap(body["gauge_needle"] as? [String: Any])
        XCTAssertEqual(payload as? [String: Int], ["position": 30])
    }

    func testCreateGaugeNeedleValidationErrorCarriesFieldErrors() async throws {
        let body = Data(
            #"{"error":"Validation failed","errors":{"position":["must be between 0 and 100"]}}"#.utf8)
        let transport = MockTransport(
            statusCode: 422, data: body, headers: ["X-Request-Id": "req-needle-422"])
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.gauges.createGaugeNeedle(
                projectId: Self.bucketId,
                req: CreateGaugeNeedleRequest(gaugeNeedle: GaugeNeedlePayload(position: 500))
            )
            XCTFail("Expected a validation error")
        } catch let error as BasecampError {
            guard case .validation = error else {
                return XCTFail("Expected .validation, got \(error)")
            }
            XCTAssertEqual(error.httpStatusCode, 422)
            XCTAssertEqual(error.exitCode, 9)
            XCTAssertEqual(
                error.message, "Validation failed (position: must be between 0 and 100)")
            XCTAssertEqual(error.fieldErrors, ["position": ["must be between 0 and 100"]])
            XCTAssertEqual(error.requestId, "req-needle-422")
            XCTAssertFalse(error.isRetryable)
        }
    }

    // MARK: - UpdateGaugeNeedle (PUT /gauge_needles/{needleId})

    func testUpdateGaugeNeedleSendsSuffixlessPutAndWrappedBody() async throws {
        let data = try JSONSerialization.data(withJSONObject: needleJSON())
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        var payload = GaugeNeedleUpdatePayload()
        payload.description = "<div>Revised note</div>"
        let needle = try await account.gauges.updateGaugeNeedle(
            needleId: Self.needleId, req: UpdateGaugeNeedleRequest(gaugeNeedle: payload))

        XCTAssertEqual(sentPath(transport), "/999999999/gauge_needles/\(Self.needleId)")
        XCTAssertFalse(
            sentPath(transport).hasSuffix(".json"),
            "UpdateGaugeNeedle is suffixless by design; got \(sentPath(transport))")
        XCTAssertEqual(transport.lastRequest!.request.httpMethod, "PUT")

        let body = try sentBody(transport)
        XCTAssertEqual(Set(body.keys), ["gauge_needle"])
        XCTAssertEqual(
            body["gauge_needle"] as? [String: String], ["description": "<div>Revised note</div>"])

        XCTAssertEqual(needle.id, Self.needleId)
        XCTAssertEqual(needle.title, "Moved the needle")
    }

    func testUpdateGaugeNeedleNotFound() async throws {
        let body = Data(#"{"error":"Needle not found"}"#.utf8)
        let transport = MockTransport(statusCode: 404, data: body)
        let account = makeTestAccountClient(transport: transport)

        var payload = GaugeNeedleUpdatePayload()
        payload.description = "<div>Revised note</div>"

        do {
            _ = try await account.gauges.updateGaugeNeedle(
                needleId: 999, req: UpdateGaugeNeedleRequest(gaugeNeedle: payload))
            XCTFail("Expected a notFound error")
        } catch let error as BasecampError {
            guard case .notFound = error else { return XCTFail("Expected .notFound, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 404)
            XCTAssertEqual(error.exitCode, 2)
            XCTAssertEqual(error.message, "Needle not found")
            XCTAssertFalse(error.isRetryable)
        }
    }

    // MARK: - DestroyGaugeNeedle (DELETE /gauge_needles/{needleId}, 204)

    func testDestroyGaugeNeedleSendsSuffixlessDelete() async throws {
        let transport = MockTransport(statusCode: 204, data: Data())
        let account = makeTestAccountClient(transport: transport)

        try await account.gauges.destroyGaugeNeedle(needleId: Self.needleId)

        XCTAssertEqual(sentPath(transport), "/999999999/gauge_needles/\(Self.needleId)")
        XCTAssertFalse(
            sentPath(transport).hasSuffix(".json"),
            "DestroyGaugeNeedle is suffixless by design; got \(sentPath(transport))")
        XCTAssertEqual(transport.lastRequest!.request.httpMethod, "DELETE")
        XCTAssertNil(transport.lastRequest!.request.httpBody)
    }

    func testDestroyGaugeNeedleForbidden() async throws {
        let body = Data(#"{"error":"You cannot delete this needle"}"#.utf8)
        let transport = MockTransport(statusCode: 403, data: body)
        let account = makeTestAccountClient(transport: transport)

        do {
            try await account.gauges.destroyGaugeNeedle(needleId: Self.needleId)
            XCTFail("Expected a forbidden error")
        } catch let error as BasecampError {
            guard case .forbidden = error else { return XCTFail("Expected .forbidden, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 403)
            XCTAssertEqual(error.exitCode, 4)
            XCTAssertEqual(error.message, "You cannot delete this needle")
            XCTAssertFalse(error.isRetryable)
        }
    }

    // MARK: - ToggleGauge (PUT /projects/{projectId}/gauge.json, 200 + empty body)

    /// bc3 answers `head :ok` here: a 200 with an EMPTY body, not a 204. A
    /// void-decode of an empty 200 is exactly the thing that can regress, so
    /// the stub reproduces it literally.
    func testToggleGaugeEnablesWithEmpty200Body() async throws {
        let transport = MockTransport(statusCode: 200, data: Data())
        let account = makeTestAccountClient(transport: transport)

        try await account.gauges.toggleGauge(
            projectId: Self.bucketId, req: ToggleGaugeRequest(gauge: GaugeTogglePayload(enabled: true)))

        XCTAssertEqual(sentPath(transport), "/999999999/projects/\(Self.bucketId)/gauge.json")
        XCTAssertEqual(transport.lastRequest!.request.httpMethod, "PUT")

        let body = try sentBody(transport)
        XCTAssertEqual(Set(body.keys), ["gauge"])
        let payload = try XCTUnwrap(body["gauge"] as? [String: Any])
        XCTAssertEqual(Set(payload.keys), ["enabled"])
        XCTAssertEqual(payload["enabled"] as? Bool, true)
    }

    func testToggleGaugeDisablesWithFalse() async throws {
        let transport = MockTransport(statusCode: 200, data: Data())
        let account = makeTestAccountClient(transport: transport)

        try await account.gauges.toggleGauge(
            projectId: Self.bucketId,
            req: ToggleGaugeRequest(gauge: GaugeTogglePayload(enabled: false)))

        let payload = try XCTUnwrap(try sentBody(transport)["gauge"] as? [String: Any])
        XCTAssertEqual(payload["enabled"] as? Bool, false)
    }

    func testToggleGaugeForbidden() async throws {
        let body = Data(#"{"error":"Only admins can toggle the gauge"}"#.utf8)
        let transport = MockTransport(
            statusCode: 403, data: body, headers: ["X-Request-Id": "req-toggle-403"])
        let account = makeTestAccountClient(transport: transport)

        do {
            try await account.gauges.toggleGauge(
                projectId: Self.bucketId,
                req: ToggleGaugeRequest(gauge: GaugeTogglePayload(enabled: true)))
            XCTFail("Expected a forbidden error")
        } catch let error as BasecampError {
            guard case .forbidden = error else { return XCTFail("Expected .forbidden, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 403)
            XCTAssertEqual(error.exitCode, 4)
            XCTAssertEqual(error.message, "Only admins can toggle the gauge")
            XCTAssertEqual(error.requestId, "req-toggle-403")
            XCTAssertFalse(error.isRetryable)
        }
    }

    // MARK: - Operation metadata

    /// The gauge operations split on scope: the project-scoped ones carry
    /// `projectId`, the needle member routes carry `resourceId`, and the
    /// account-wide report carries neither.
    func testOperationMetadataScopes() async throws {
        let spy = GaugeSpyHooks()
        let needleData = try JSONSerialization.data(withJSONObject: needleJSON())
        let listData = try JSONSerialization.data(withJSONObject: [gaugeJSON()])

        let listTransport = MockTransport(statusCode: 200, data: listData)
        _ = try await makeTestAccountClient(transport: listTransport, hooks: spy).gauges.listGauges()

        let needleTransport = MockTransport(statusCode: 200, data: needleData)
        _ = try await makeTestAccountClient(transport: needleTransport, hooks: spy)
            .gauges.gaugeNeedle(needleId: Self.needleId)

        let createTransport = MockTransport(statusCode: 201, data: needleData)
        _ = try await makeTestAccountClient(transport: createTransport, hooks: spy)
            .gauges.createGaugeNeedle(
                projectId: Self.bucketId,
                req: CreateGaugeNeedleRequest(gaugeNeedle: GaugeNeedlePayload(position: 72)))

        XCTAssertEqual(spy.operationStarts.map(\.operation), ["ListGauges", "GetGaugeNeedle", "CreateGaugeNeedle"])

        XCTAssertNil(spy.operationStarts[0].projectId, "the gauges report is account-wide")
        XCTAssertNil(spy.operationStarts[0].resourceId)
        XCTAssertFalse(spy.operationStarts[0].isMutation)

        XCTAssertEqual(spy.operationStarts[1].resourceId, Self.needleId)
        XCTAssertNil(spy.operationStarts[1].projectId, "the needle member route is not project-scoped")

        XCTAssertEqual(spy.operationStarts[2].projectId, Self.bucketId)
        XCTAssertTrue(spy.operationStarts[2].isMutation)
    }
}

/// Records operation-start callbacks so tests can assert emitted metadata.
private final class GaugeSpyHooks: BasecampHooks, @unchecked Sendable {
    private let lock = NSLock()
    private var _operationStarts: [OperationInfo] = []

    var operationStarts: [OperationInfo] {
        lock.withLock { _operationStarts }
    }

    func onOperationStart(_ info: OperationInfo) {
        lock.withLock { _operationStarts.append(info) }
    }
}
