$version: "2"
namespace basecamp

use smithy.api#examples

// Polymorphic endpoint: one flat shape, discriminated by groups_url XOR
// group_position_url. Both variants report type "Todolist" — BC3's recording
// partial emits recordable_type, and a group is a Todolist whose parent is a
// Todolist. Never branch on the type string.
apply GetTodolistOrGroup @examples([
  {
    title: "Get a to-do list"
    documentation: "A to-do list: parent is a Todoset, and groups_url is present"
    input: { accountId: "999", id: 987654 }
    output: { result: {
      id: 987654, status: "active", name: "Launch Tasks",
      visible_to_clients: false, created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z",
      title: "Launch Tasks", inherits_status: true, type: "Todolist",
      description: "", description_attachments: [],
      url: "https://3.basecampapi.com/999/buckets/12345678/todolists/987654.json",
      app_url: "https://3.basecamp.com/999/buckets/12345678/todolists/987654",
      bubble_up_url: "https://3.basecampapi.com/999/buckets/12345678/recordings/987654/bubble_up.json",
      creator: { id: 1, name: "Someone", created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" },
      bucket: { id: 12345678, name: "My Project", type: "Project" },
      parent: { id: 99999, title: "To-dos", type: "Todoset", url: "https://3.basecampapi.com/999/buckets/12345678/todosets/99999.json", app_url: "https://3.basecamp.com/999/buckets/12345678/todosets/99999" },
      comments_app_url: "https://3.basecamp.com/999/buckets/12345678/recordings/987654/comments",
      groups_url: "https://3.basecampapi.com/999/buckets/12345678/todolists/987654/groups.json"
    } }
  },
  {
    title: "Get a group"
    documentation: "A group: parent is a Todolist, and group_position_url replaces groups_url"
    input: { accountId: "999", id: 111222 }
    output: { result: {
      id: 111222, status: "active", name: "Q1 Milestones",
      visible_to_clients: false, created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z",
      title: "Q1 Milestones", inherits_status: true, type: "Todolist",
      description: "", description_attachments: [],
      url: "https://3.basecampapi.com/999/buckets/12345678/todolists/111222.json",
      app_url: "https://3.basecamp.com/999/buckets/12345678/todolists/111222",
      bubble_up_url: "https://3.basecampapi.com/999/buckets/12345678/recordings/111222/bubble_up.json",
      creator: { id: 1, name: "Someone", created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" },
      bucket: { id: 12345678, name: "My Project", type: "Project" },
      parent: { id: 987654, title: "Launch Tasks", type: "Todolist", url: "https://3.basecampapi.com/999/buckets/12345678/todolists/987654.json", app_url: "https://3.basecamp.com/999/buckets/12345678/todolists/987654" },
      comments_app_url: "https://3.basecamp.com/999/buckets/12345678/recordings/111222/comments",
      group_position_url: "https://3.basecampapi.com/999/buckets/12345678/todolists/groups/111222/position.json"
    } }
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
    input: { accountId: "999", recordingId: 555666 }
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
    input: { accountId: "999", recordingId: 987654, subscriptions: [111, 222] }
  },
  {
    title: "Remove subscribers"
    input: { accountId: "999", recordingId: 987654, unsubscriptions: [333] }
  }
])
