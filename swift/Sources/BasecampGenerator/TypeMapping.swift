import Foundation

// MARK: - Type Aliases

/// Maps OpenAPI schema names to Swift type names.
/// Format: SchemaName → (typeName, kind).
let typeAliases: [String: (name: String, kind: String)] = [
    "Todo": ("Todo", "entity"),
    "Person": ("Person", "entity"),
    "Project": ("Project", "entity"),
    "Message": ("Message", "entity"),
    "Comment": ("Comment", "entity"),
    "Card": ("Card", "entity"),
    "CardTable": ("CardTable", "entity"),
    "CardColumn": ("CardColumn", "entity"),
    "CardStep": ("CardStep", "entity"),
    "Campfire": ("Campfire", "entity"),
    "CampfireLine": ("CampfireLine", "entity"),
    "Chatbot": ("Chatbot", "entity"),
    "Webhook": ("Webhook", "entity"),
    "Vault": ("Vault", "entity"),
    "Document": ("Document", "entity"),
    "Upload": ("Upload", "entity"),
    "Schedule": ("Schedule", "entity"),
    "ScheduleEntry": ("ScheduleEntry", "entity"),
    "Recording": ("Recording", "entity"),
    "Template": ("Template", "entity"),
    "Todolist": ("Todolist", "entity"),
    "Todoset": ("Todoset", "entity"),
    "TodolistGroup": ("TodolistGroup", "entity"),
    "Questionnaire": ("Questionnaire", "entity"),
    "Question": ("Question", "entity"),
    "QuestionAnswer": ("Answer", "entity"),
    "Subscription": ("Subscription", "entity"),
    "Forward": ("Forward", "entity"),
    "ForwardReply": ("ForwardReply", "entity"),
    "Inbox": ("Inbox", "entity"),
    "MessageBoard": ("MessageBoard", "entity"),
    "MessageType": ("MessageType", "entity"),
    "Event": ("Event", "entity"),
    "Tool": ("Tool", "entity"),
    "LineupMarker": ("LineupMarker", "entity"),
    "ClientApproval": ("ClientApproval", "entity"),
    "ClientCorrespondence": ("ClientCorrespondence", "entity"),
    "ClientReply": ("ClientReply", "entity"),
    "Boost": ("Boost", "entity"),
]

// MARK: - Property Hints

/// Human-friendly descriptions for common body/query property names.
let propertyHints: [String: String] = [
    "content": "Text content",
    "description": "Rich text description (HTML)",
    "name": "Display name",
    "title": "Title",
    "subject": "Subject line",
    "summary": "Summary text",
    "notify": "Whether to send notifications to relevant people",
    "position": "Position for ordering (1-based)",
    "status": "Status filter",
    "assignee_ids": "Person IDs to assign to",
    "completion_subscriber_ids": "Person IDs to notify on completion",
    "subscriber_ids": "Person IDs to subscribe",
    "due_on": "Due date",
    "starts_on": "Start date",
    "start_date": "Start date",
    "end_date": "End date",
    "color": "Color value",
    "icon": "Icon identifier",
    "enabled": "Whether this is enabled",
    "parent_id": "Parent resource ID to move under",
    "admissions": "Access policy for the project",
    "schedule_attributes": "Schedule date range settings",
]

// MARK: - Schema → Swift Type

/// Maps an OpenAPI schema to a Swift type string.
func schemaToSwiftType(_ schema: [String: Any]) -> String {
    if let ref = schema["$ref"] as? String {
        return resolveRef(ref)
    }
    let type = schema["type"] as? String ?? "String"
    switch type {
    case "integer":
        let format = schema["format"] as? String ?? ""
        return format == "int32" ? "Int32" : "Int"
    case "boolean":
        return "Bool"
    case "number":
        return "Double"
    case "array":
        if let items = schema["items"] as? [String: Any] {
            let itemType = schemaToSwiftType(items)
            return "[\(itemType)]"
        }
        return "[Any]"
    case "object":
        if let additionalProperties = schema["additionalProperties"] as? [String: Any] {
            let valueType = schemaToSwiftType(additionalProperties)
            return "[String: \(valueType)]"
        }
        return "[String: Any]"
    default:
        return "String"
    }
}

/// Maps an OpenAPI schema to the Swift type for a Codable struct property.
/// Uses optional wrapper types for nested refs.
func schemaToSwiftPropertyType(_ schema: [String: Any]) -> String {
    schemaToSwiftType(schema)
}

/// Gets the entity type name for a response schema ref.
func getEntityTypeName(_ schemaRef: String, schemas: [String: Any]) -> String? {
    // Direct entity reference
    if typeAliases[schemaRef] != nil {
        return typeAliases[schemaRef]!.name
    }

    // ResponseContent types — resolve to underlying entity
    if let entitySchema = findUnderlyingEntitySchema(schemaRef, schemas: schemas) {
        return typeAliases[entitySchema]?.name
    }

    return nil
}

/// Resolves ResponseContent wrapper schemas to their underlying entity schema name.
func findUnderlyingEntitySchema(_ responseSchemaRef: String, schemas: [String: Any]) -> String? {
    guard let schema = schemas[responseSchemaRef] as? [String: Any] else { return nil }

    // Direct $ref to known entity
    if let ref = schema["$ref"] as? String {
        let refName = resolveRef(ref)
        if typeAliases[refName] != nil { return refName }
    }

    // Array with items.$ref to known entity
    if (schema["type"] as? String) == "array",
       let items = schema["items"] as? [String: Any],
       let ref = items["$ref"] as? String {
        let refName = resolveRef(ref)
        if typeAliases[refName] != nil { return refName }
    }

    return nil
}
