// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListLinesCampfireOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct ListUploadsCampfireOptions: Sendable {
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
    public func createLine(campfireId: Int, req: CreateCampfireLineRequest) async throws -> CampfireLine {
        return try await request(
            OperationInfo(service: "Campfires", operation: "CreateCampfireLine", resourceType: "campfire_line", isMutation: true, resourceId: campfireId),
            method: "POST",
            path: "/chats/\(campfireId)/lines.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateCampfireLine")
        )
    }

    public func createUpload(campfireId: Int, data: Data, contentType: String, name: String) async throws -> CampfireLine {
        var queryItems: [URLQueryItem] = []
        queryItems.append(URLQueryItem(name: "name", value: name))
        return try await request(
            OperationInfo(service: "Campfires", operation: "CreateCampfireUpload", resourceType: "campfire_upload", isMutation: true, resourceId: campfireId),
            method: "POST",
            path: "/chats/\(campfireId)/uploads.json" + queryString(queryItems),
            body: data,
            contentType: contentType,
            retryConfig: Metadata.retryConfig(for: "CreateCampfireUpload")
        )
    }

    public func createChatbot(campfireId: Int, req: CreateChatbotRequest) async throws -> Chatbot {
        return try await request(
            OperationInfo(service: "Campfires", operation: "CreateChatbot", resourceType: "chatbot", isMutation: true, resourceId: campfireId),
            method: "POST",
            path: "/chats/\(campfireId)/integrations.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateChatbot")
        )
    }

    public func deleteLine(campfireId: Int, lineId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Campfires", operation: "DeleteCampfireLine", resourceType: "campfire_line", isMutation: true, resourceId: lineId),
            method: "DELETE",
            path: "/chats/\(campfireId)/lines/\(lineId)",
            retryConfig: Metadata.retryConfig(for: "DeleteCampfireLine")
        )
    }

    public func deleteChatbot(campfireId: Int, chatbotId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Campfires", operation: "DeleteChatbot", resourceType: "chatbot", isMutation: true, resourceId: chatbotId),
            method: "DELETE",
            path: "/chats/\(campfireId)/integrations/\(chatbotId)",
            retryConfig: Metadata.retryConfig(for: "DeleteChatbot")
        )
    }

    public func get(campfireId: Int) async throws -> Campfire {
        return try await request(
            OperationInfo(service: "Campfires", operation: "GetCampfire", resourceType: "campfire", isMutation: false, resourceId: campfireId),
            method: "GET",
            path: "/chats/\(campfireId)",
            retryConfig: Metadata.retryConfig(for: "GetCampfire")
        )
    }

    public func getLine(campfireId: Int, lineId: Int) async throws -> CampfireLine {
        return try await request(
            OperationInfo(service: "Campfires", operation: "GetCampfireLine", resourceType: "campfire_line", isMutation: false, resourceId: lineId),
            method: "GET",
            path: "/chats/\(campfireId)/lines/\(lineId)",
            retryConfig: Metadata.retryConfig(for: "GetCampfireLine")
        )
    }

    public func getChatbot(campfireId: Int, chatbotId: Int) async throws -> Chatbot {
        return try await request(
            OperationInfo(service: "Campfires", operation: "GetChatbot", resourceType: "chatbot", isMutation: false, resourceId: chatbotId),
            method: "GET",
            path: "/chats/\(campfireId)/integrations/\(chatbotId)",
            retryConfig: Metadata.retryConfig(for: "GetChatbot")
        )
    }

    public func listLines(campfireId: Int, options: ListLinesCampfireOptions? = nil) async throws -> ListResult<CampfireLine> {
        return try await requestPaginated(
            OperationInfo(service: "Campfires", operation: "ListCampfireLines", resourceType: "campfire_line", isMutation: false, resourceId: campfireId),
            path: "/chats/\(campfireId)/lines.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListCampfireLines")
        )
    }

    public func listUploads(campfireId: Int, options: ListUploadsCampfireOptions? = nil) async throws -> ListResult<CampfireLine> {
        return try await requestPaginated(
            OperationInfo(service: "Campfires", operation: "ListCampfireUploads", resourceType: "campfire_upload", isMutation: false, resourceId: campfireId),
            path: "/chats/\(campfireId)/uploads.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListCampfireUploads")
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

    public func listChatbots(campfireId: Int, options: ListChatbotsCampfireOptions? = nil) async throws -> ListResult<Chatbot> {
        return try await requestPaginated(
            OperationInfo(service: "Campfires", operation: "ListChatbots", resourceType: "chatbot", isMutation: false, resourceId: campfireId),
            path: "/chats/\(campfireId)/integrations.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListChatbots")
        )
    }

    public func updateChatbot(campfireId: Int, chatbotId: Int, req: UpdateChatbotRequest) async throws -> Chatbot {
        return try await request(
            OperationInfo(service: "Campfires", operation: "UpdateChatbot", resourceType: "chatbot", isMutation: true, resourceId: chatbotId),
            method: "PUT",
            path: "/chats/\(campfireId)/integrations/\(chatbotId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateChatbot")
        )
    }
}
