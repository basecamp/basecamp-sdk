// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct FeedEventDetails: Codable, Sendable {
    public var boostId: Int?
    public var boostedEventId: Int?
    public var boostedEventType: String?
    public var columnId: Int?
    public var previousColumnId: Int?

    public init(
        boostId: Int? = nil,
        boostedEventId: Int? = nil,
        boostedEventType: String? = nil,
        columnId: Int? = nil,
        previousColumnId: Int? = nil
    ) {
        self.boostId = boostId
        self.boostedEventId = boostedEventId
        self.boostedEventType = boostedEventType
        self.columnId = columnId
        self.previousColumnId = previousColumnId
    }
}
