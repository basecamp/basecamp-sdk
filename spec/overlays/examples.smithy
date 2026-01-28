$version: "2"
namespace basecamp

use smithy.api#examples

// Polymorphic endpoints
apply GetTodolistOrGroup @examples([
  {
    title: "Get a Todolist"
    documentation: "Returns a Todolist when ID refers to a todolist"
    input: { accountId: "999", projectId: 12345678, id: 987654 }
    output: { result: { todolist: { id: 987654, status: "active", name: "Launch Tasks" } } }
  },
  {
    title: "Get a TodolistGroup"
    documentation: "Returns a TodolistGroup when ID refers to a group"
    input: { accountId: "999", projectId: 12345678, id: 111222 }
    output: { result: { group: { id: 111222, status: "active", name: "Q1 Milestones" } } }
  }
])

apply ListRecordings @examples([
  {
    title: "List Todo recordings"
    documentation: "Use simple type name for basic resources"
    input: { accountId: "999", type: "Todo" }
  },
  {
    title: "List Kanban Card recordings"
    documentation: "Use double-colon notation for nested types"
    input: { accountId: "999", type: "Kanban::Card" }
  },
  {
    title: "List Question Answer recordings"
    documentation: "Another nested type example"
    input: { accountId: "999", type: "Question::Answer" }
  }
])

apply TrashRecording @examples([
  {
    title: "Trash any recording type"
    documentation: "Works on comments, messages, documents, cards - any recording"
    input: { accountId: "999", projectId: 12345678, recordingId: 555666 }
  }
])

apply UpdateProjectAccess @examples([
  {
    title: "Grant access to existing users"
    documentation: "Use grant array with person IDs"
    input: { accountId: "999", projectId: 12345678, grant: [111, 222] }
  },
  {
    title: "Revoke access"
    documentation: "Use revoke array to remove users"
    input: { accountId: "999", projectId: 12345678, revoke: [333] }
  },
  {
    title: "Invite new users"
    documentation: "Use create array for new users without accounts"
    input: { accountId: "999", projectId: 12345678, create: [{ name: "Jane", email_address: "jane@example.com" }] }
  }
])

apply UpdateSubscription @examples([
  {
    title: "Add subscribers"
    input: { accountId: "999", projectId: 12345678, recordingId: 987654, subscriptions: [111, 222] }
  },
  {
    title: "Remove subscribers"
    input: { accountId: "999", projectId: 12345678, recordingId: 987654, unsubscriptions: [333] }
  }
])
