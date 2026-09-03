// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct TemplateLibraryCopy: Codable, Sendable {
    public let destinationParentId: Int
    public let id: Int
    public let sourceRecordingId: Int
    public let status: String
    public let url: String
    public var destinationTodolist: Todolist?

    public init(
        destinationParentId: Int,
        id: Int,
        sourceRecordingId: Int,
        status: String,
        url: String,
        destinationTodolist: Todolist? = nil
    ) {
        self.destinationParentId = destinationParentId
        self.id = id
        self.sourceRecordingId = sourceRecordingId
        self.status = status
        self.url = url
        self.destinationTodolist = destinationTodolist
    }
}
