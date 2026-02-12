// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListLinesCampfireOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct ListCampfireOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct ListChatbotsCampfireOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class CampfiresService: BaseService, @unchecked Sendable {
    public func createLine(projectId: Int, campfireId: Int, req: CreateCampfireLineRequest) async throws -> CampfireLine {
        return try await request(
            OperationInfo(service: "Campfires", operation: "CreateCampfireLine", resourceType: "campfire_line", isMutation: true, projectId: projectId, resourceId: campfireId),
            method: "POST",
            path: "/buckets/\(projectId)/chats/\(campfireId)/lines.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateCampfireLine")
        )
    }

    public func createChatbot(projectId: Int, campfireId: Int, req: CreateChatbotRequest) async throws -> Chatbot {
        return try await request(
            OperationInfo(service: "Campfires", operation: "CreateChatbot", resourceType: "chatbot", isMutation: true, projectId: projectId, resourceId: campfireId),
            method: "POST",
            path: "/buckets/\(projectId)/chats/\(campfireId)/integrations.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateChatbot")
        )
    }

    public func deleteLine(projectId: Int, campfireId: Int, lineId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Campfires", operation: "DeleteCampfireLine", resourceType: "campfire_line", isMutation: true, projectId: projectId, resourceId: campfireId),
            method: "DELETE",
            path: "/buckets/\(projectId)/chats/\(campfireId)/lines/\(lineId)",
            retryConfig: Metadata.retryConfig(for: "DeleteCampfireLine")
        )
    }

    public func deleteChatbot(projectId: Int, campfireId: Int, chatbotId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Campfires", operation: "DeleteChatbot", resourceType: "chatbot", isMutation: true, projectId: projectId, resourceId: campfireId),
            method: "DELETE",
            path: "/buckets/\(projectId)/chats/\(campfireId)/integrations/\(chatbotId)",
            retryConfig: Metadata.retryConfig(for: "DeleteChatbot")
        )
    }

    public func get(projectId: Int, campfireId: Int) async throws -> Campfire {
        return try await request(
            OperationInfo(service: "Campfires", operation: "GetCampfire", resourceType: "campfire", isMutation: false, projectId: projectId, resourceId: campfireId),
            method: "GET",
            path: "/buckets/\(projectId)/chats/\(campfireId)",
            retryConfig: Metadata.retryConfig(for: "GetCampfire")
        )
    }

    public func getLine(projectId: Int, campfireId: Int, lineId: Int) async throws -> CampfireLine {
        return try await request(
            OperationInfo(service: "Campfires", operation: "GetCampfireLine", resourceType: "campfire_line", isMutation: false, projectId: projectId, resourceId: campfireId),
            method: "GET",
            path: "/buckets/\(projectId)/chats/\(campfireId)/lines/\(lineId)",
            retryConfig: Metadata.retryConfig(for: "GetCampfireLine")
        )
    }

    public func getChatbot(projectId: Int, campfireId: Int, chatbotId: Int) async throws -> Chatbot {
        return try await request(
            OperationInfo(service: "Campfires", operation: "GetChatbot", resourceType: "chatbot", isMutation: false, projectId: projectId, resourceId: campfireId),
            method: "GET",
            path: "/buckets/\(projectId)/chats/\(campfireId)/integrations/\(chatbotId)",
            retryConfig: Metadata.retryConfig(for: "GetChatbot")
        )
    }

    public func listLines(projectId: Int, campfireId: Int, options: ListLinesCampfireOptions? = nil) async throws -> ListResult<CampfireLine> {
        return try await requestPaginated(
            OperationInfo(service: "Campfires", operation: "ListCampfireLines", resourceType: "campfire_line", isMutation: false, projectId: projectId, resourceId: campfireId),
            path: "/buckets/\(projectId)/chats/\(campfireId)/lines.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListCampfireLines")
        )
    }

    public func list(options: ListCampfireOptions? = nil) async throws -> ListResult<Campfire> {
        return try await requestPaginated(
            OperationInfo(service: "Campfires", operation: "ListCampfires", resourceType: "campfire", isMutation: false),
            path: "/chats.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListCampfires")
        )
    }

    public func listChatbots(projectId: Int, campfireId: Int, options: ListChatbotsCampfireOptions? = nil) async throws -> ListResult<Chatbot> {
        return try await requestPaginated(
            OperationInfo(service: "Campfires", operation: "ListChatbots", resourceType: "chatbot", isMutation: false, projectId: projectId, resourceId: campfireId),
            path: "/buckets/\(projectId)/chats/\(campfireId)/integrations.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListChatbots")
        )
    }

    public func updateChatbot(projectId: Int, campfireId: Int, chatbotId: Int, req: UpdateChatbotRequest) async throws -> Chatbot {
        return try await request(
            OperationInfo(service: "Campfires", operation: "UpdateChatbot", resourceType: "chatbot", isMutation: true, projectId: projectId, resourceId: campfireId),
            method: "PUT",
            path: "/buckets/\(projectId)/chats/\(campfireId)/integrations/\(chatbotId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateChatbot")
        )
    }
}
