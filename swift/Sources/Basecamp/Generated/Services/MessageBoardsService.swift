// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class MessageBoardsService: BaseService, @unchecked Sendable {
    public func get(projectId: Int, boardId: Int) async throws -> MessageBoard {
        return try await request(
            OperationInfo(service: "MessageBoards", operation: "GetMessageBoard", resourceType: "message_board", isMutation: false, projectId: projectId, resourceId: boardId),
            method: "GET",
            path: "/buckets/\(projectId)/message_boards/\(boardId)",
            retryConfig: Metadata.retryConfig(for: "GetMessageBoard")
        )
    }
}
