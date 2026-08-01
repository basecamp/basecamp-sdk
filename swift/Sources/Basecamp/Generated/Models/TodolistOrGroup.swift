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

        let flat = try decoder.singleValueContainer()
        self.todolist = try? flat.decode(Todolist.self)
        self.group = (self.todolist == nil) ? try? flat.decode(TodolistGroup.self) : nil
        guard self.todolist != nil || self.group != nil else {
            throw DecodingError.dataCorrupted(
                DecodingError.Context(
                    codingPath: decoder.codingPath,
                    debugDescription: "TodolistOrGroup: body matched no variant"
                )
            )
        }
    }

    public func encode(to encoder: any Encoder) throws {
        var envelope = encoder.container(keyedBy: CodingKeys.self)
        try envelope.encodeIfPresent(todolist, forKey: .todolist)
        try envelope.encodeIfPresent(group, forKey: .group)
    }
}
