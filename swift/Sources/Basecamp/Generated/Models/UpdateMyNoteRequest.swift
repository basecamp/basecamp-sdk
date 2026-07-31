// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateMyNoteRequest: Codable, Sendable {
    public let note: MyNoteAttributes

    public init(note: MyNoteAttributes) {
        self.note = note
    }
}
