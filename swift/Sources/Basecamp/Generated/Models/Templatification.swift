// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Templatification: Codable, Sendable {
    public let id: Int
    public let sourceRecordingId: Int
    public let status: String
    public let url: String
    public var destinationCardTable: CardTable?
    public var destinationTodolist: Todolist?

    public init(
        id: Int,
        sourceRecordingId: Int,
        status: String,
        url: String,
        destinationCardTable: CardTable? = nil,
        destinationTodolist: Todolist? = nil
    ) {
        self.id = id
        self.sourceRecordingId = sourceRecordingId
        self.status = status
        self.url = url
        self.destinationCardTable = destinationCardTable
        self.destinationTodolist = destinationTodolist
    }
}
