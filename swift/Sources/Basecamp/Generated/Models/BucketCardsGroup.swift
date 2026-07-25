// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct BucketCardsGroup: Codable, Sendable {
    public let bucket: RecordingBucket
    public let cards: [Card]

    public init(bucket: RecordingBucket, cards: [Card]) {
        self.bucket = bucket
        self.cards = cards
    }
}
