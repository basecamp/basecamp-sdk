// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct TodolistOrGroup: Codable, Sendable {
    public var todolist: Todolist?
    public var group: TodolistGroup?

    public enum CodingKeys: String, CodingKey {
        case todolist = "todolist"
        case group = "group"
    }

    public init(todolist: Todolist) {
        self.todolist = todolist
        self.group = nil
    }

    public init(group: TodolistGroup) {
        self.todolist = nil
        self.group = group
    }

    public init(from decoder: any Decoder) throws {
        if let envelope = try? decoder.container(keyedBy: CodingKeys.self), !envelope.allKeys.isEmpty {
            self.todolist = try? envelope.decodeIfPresent(Todolist.self, forKey: .todolist)
            self.group = try? envelope.decodeIfPresent(TodolistGroup.self, forKey: .group)
            if self.todolist != nil || self.group != nil { return }
        }

        let bodyKeys: Set<String> = (try? decoder.container(keyedBy: PermissiveKey.self))
            .map { Set($0.allKeys.map(\.stringValue)) } ?? []
        var firstArmError: Error?

        let flat = try decoder.singleValueContainer()
        do {
            self.todolist = try flat.decode(Todolist.self)
        } catch {
            firstArmError = error
            self.todolist = nil
        }
        // Keys only an earlier arm defines, in both wire and camelCase
        // spelling. Their presence means the body really is that earlier
        // shape, so its failure must not be masked here.
        let earlierOnlyKeys1: Set<String> = ["boostsCount", "boostsUrl", "boosts_count", "boosts_url", "description", "descriptionAttachments", "description_attachments", "groupsUrl", "groups_url"]
        self.group = (self.todolist == nil && bodyKeys.isDisjoint(with: earlierOnlyKeys1)) ? try? flat.decode(TodolistGroup.self) : nil
        guard self.todolist != nil || self.group != nil else {
            if let firstArmError { throw firstArmError }
            throw DecodingError.dataCorrupted(
                DecodingError.Context(
                    codingPath: decoder.codingPath,
                    debugDescription: "TodolistOrGroup: body matched no variant"
                )
            )
        }
    }

    /// Reads the raw wire keys so the decoder can tell which arm a flat body
    /// really is, independent of any arm's own CodingKeys.
    private struct PermissiveKey: CodingKey {
        let stringValue: String
        var intValue: Int? { nil }
        init?(stringValue: String) { self.stringValue = stringValue }
        init?(intValue: Int) { nil }
    }

    public func encode(to encoder: any Encoder) throws {
        var envelope = encoder.container(keyedBy: CodingKeys.self)
        try envelope.encodeIfPresent(todolist, forKey: .todolist)
        try envelope.encodeIfPresent(group, forKey: .group)
    }
}
