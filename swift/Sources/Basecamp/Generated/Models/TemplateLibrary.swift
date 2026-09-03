// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct TemplateLibrary: Codable, Sendable {
    public let bucket: RecordingBucket
    public let todolists: [Todolist]
    public let todoset: RecordingParent

    public init(bucket: RecordingBucket, todolists: [Todolist], todoset: RecordingParent) {
        self.bucket = bucket
        self.todolists = todolists
        self.todoset = todoset
    }
}
