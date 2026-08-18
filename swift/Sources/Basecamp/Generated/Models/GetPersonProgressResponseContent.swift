// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct GetPersonProgressResponseContent: Codable, Sendable {
    public let events: [TimelineEvent]
    public let person: Person

    public init(events: [TimelineEvent], person: Person) {
        self.events = events
        self.person = person
    }
}
