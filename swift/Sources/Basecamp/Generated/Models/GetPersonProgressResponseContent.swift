// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct GetPersonProgressResponseContent: Codable, Sendable {
    public var events: [TimelineEvent]?
    public var person: Person?

    public init(events: [TimelineEvent]? = nil, person: Person? = nil) {
        self.events = events
        self.person = person
    }
}
