// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct PollEventsEventFeedOptions: Sendable {
    /// Entry point: a decimal event id (start after it; `0` replays served history back to the epoch), or the literal `now` (skip history). Mutually exclusive with `position` in practice; omit both to enter at the present.
    public var since: String?
    /// Resume token from a previous page's `position`. Opaque and signed; never constructed or parsed client-side.
    public var position: String?
    /// Comma-separated event types from the catalog (e.g. `message.created,comment.created`).
    public var types: String?
    /// Comma-separated bucket (project) ids, at most 100.
    public var buckets: String?
    /// Comma-separated creator person ids, at most 100.
    public var creators: String?
    /// Comma-separated effective-performer ids (the agent on a delegated action, else the creator), at most 100. The literal `self` means the request's own effective actor and is resolved server-side before filtering.
    public var performers: String?
    /// Comma-separated effective-performer ids to exclude, at most 100; `self` as on `performers`. `exclude_performers=self` is the loop guard for an agent that acts on what it hears.
    public var excludePerformers: String?
    /// Comma-separated actor kinds: `agent`, `person`, or both. A filter, not a default — agent activity is real account activity.
    public var actorTypes: String?

    public init(
        since: String? = nil,
        position: String? = nil,
        types: String? = nil,
        buckets: String? = nil,
        creators: String? = nil,
        performers: String? = nil,
        excludePerformers: String? = nil,
        actorTypes: String? = nil
    ) {
        self.since = since
        self.position = position
        self.types = types
        self.buckets = buckets
        self.creators = creators
        self.performers = performers
        self.excludePerformers = excludePerformers
        self.actorTypes = actorTypes
    }
}

public struct PollInboxEventFeedOptions: Sendable {
    /// Entry point: `0` (earliest retained), `now` (present), or a decimal item id to start after.
    public var since: String?
    /// Resume token from a previous inbox page's `position`.
    public var position: String?
    /// Comma-separated addressing reasons: `mentioned`, `assigned`, `subscribed`, `watched`, `pinged`, `boosted`.
    public var reasons: String?
    /// Comma-separated event types, as a narrowing filter.
    public var types: String?
    /// Comma-separated bucket ids, as a narrowing filter (at most 100).
    public var buckets: String?

    public init(
        since: String? = nil,
        position: String? = nil,
        reasons: String? = nil,
        types: String? = nil,
        buckets: String? = nil
    ) {
        self.since = since
        self.position = position
        self.reasons = reasons
        self.types = types
        self.buckets = buckets
    }
}


public final class EventFeedService: BaseService, @unchecked Sendable {
    public func createStreamTicket() async throws -> CreateStreamTicketResponseContent {
        return try await request(
            OperationInfo(service: "EventFeed", operation: "CreateStreamTicket", resourceType: "stream_ticket", isMutation: true),
            method: "POST",
            path: "/events/stream_ticket.json",
            retryConfig: Metadata.retryConfig(for: "CreateStreamTicket")
        )
    }

    public func pollEvents(options: PollEventsEventFeedOptions? = nil) async throws -> PollEventsResponseContent {
        var queryItems: [URLQueryItem] = []
        if let since = options?.since {
            queryItems.append(URLQueryItem(name: "since", value: since))
        }
        if let position = options?.position {
            queryItems.append(URLQueryItem(name: "position", value: position))
        }
        if let types = options?.types {
            queryItems.append(URLQueryItem(name: "types", value: types))
        }
        if let buckets = options?.buckets {
            queryItems.append(URLQueryItem(name: "buckets", value: buckets))
        }
        if let creators = options?.creators {
            queryItems.append(URLQueryItem(name: "creators", value: creators))
        }
        if let performers = options?.performers {
            queryItems.append(URLQueryItem(name: "performers", value: performers))
        }
        if let excludePerformers = options?.excludePerformers {
            queryItems.append(URLQueryItem(name: "exclude_performers", value: excludePerformers))
        }
        if let actorTypes = options?.actorTypes {
            queryItems.append(URLQueryItem(name: "actor_types", value: actorTypes))
        }
        return try await request(
            OperationInfo(service: "EventFeed", operation: "PollEvents", resourceType: "feed_event", isMutation: false),
            method: "GET",
            path: "/events.json" + queryString(queryItems),
            retryConfig: Metadata.retryConfig(for: "PollEvents")
        )
    }

    public func pollInbox(options: PollInboxEventFeedOptions? = nil) async throws -> PollInboxResponseContent {
        var queryItems: [URLQueryItem] = []
        if let since = options?.since {
            queryItems.append(URLQueryItem(name: "since", value: since))
        }
        if let position = options?.position {
            queryItems.append(URLQueryItem(name: "position", value: position))
        }
        if let reasons = options?.reasons {
            queryItems.append(URLQueryItem(name: "reasons", value: reasons))
        }
        if let types = options?.types {
            queryItems.append(URLQueryItem(name: "types", value: types))
        }
        if let buckets = options?.buckets {
            queryItems.append(URLQueryItem(name: "buckets", value: buckets))
        }
        return try await request(
            OperationInfo(service: "EventFeed", operation: "PollInbox", resourceType: "inbox_item", isMutation: false),
            method: "GET",
            path: "/inbox.json" + queryString(queryItems),
            retryConfig: Metadata.retryConfig(for: "PollInbox")
        )
    }
}
