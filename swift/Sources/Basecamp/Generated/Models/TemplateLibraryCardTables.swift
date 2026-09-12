// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct TemplateLibraryCardTables: Codable, Sendable {
    public let bucket: RecordingBucket
    public let cardTables: [Recording]
    public let kanbanBoardset: RecordingParent?

    public init(bucket: RecordingBucket, cardTables: [Recording], kanbanBoardset: RecordingParent?) {
        self.bucket = bucket
        self.cardTables = cardTables
        self.kanbanBoardset = kanbanBoardset
    }

    enum CodingKeys: String, CodingKey {
        case bucket
        case cardTables
        case kanbanBoardset
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.bucket = try container.decode(RecordingBucket.self, forKey: .bucket)
        self.cardTables = try container.decode([Recording].self, forKey: .cardTables)
        self.kanbanBoardset = try container.decode(RecordingParent?.self, forKey: .kanbanBoardset)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.bucket, forKey: .bucket)
        try container.encode(self.cardTables, forKey: .cardTables)
        try container.encode(self.kanbanBoardset, forKey: .kanbanBoardset)
    }
}
