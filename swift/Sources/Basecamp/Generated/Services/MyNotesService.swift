// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class MyNotesService: BaseService, @unchecked Sendable {
    public func getMyNote() async throws -> MyNote {
        return try await request(
            OperationInfo(service: "MyNotes", operation: "GetMyNote", resourceType: "my_note", isMutation: false),
            method: "GET",
            path: "/my/notes.json",
            retryConfig: Metadata.retryConfig(for: "GetMyNote")
        )
    }

    public func updateMyNote(req: UpdateMyNoteRequest) async throws -> MyNote {
        return try await request(
            OperationInfo(service: "MyNotes", operation: "UpdateMyNote", resourceType: "my_note", isMutation: true),
            method: "PUT",
            path: "/my/notes.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateMyNote")
        )
    }
}
