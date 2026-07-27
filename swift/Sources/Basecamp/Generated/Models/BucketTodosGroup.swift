// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct BucketTodosGroup: Codable, Sendable {
    public let bucket: RecordingBucket
    public let todos: [Todo]

    public init(bucket: RecordingBucket, todos: [Todo]) {
        self.bucket = bucket
        self.todos = todos
    }
}
