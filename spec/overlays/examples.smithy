$version: "2"
namespace basecamp

use smithy.api#examples

// Polymorphic endpoint: one flat shape, discriminated by groups_url XOR
// group_position_url. Both variants report type "Todolist" — BC3's recording
// partial emits recordable_type, and a group is a Todolist whose parent is a
// Todolist. Never branch on the type string.
//
// `color` is DELIBERATELY absent from both examples here and injected into the
// OpenAPI projection by `jsonAdd` in spec/smithy-build.json — "blue" for the
// list, an explicit `null` for the uncolored group. It is required-and-nullable
// (see Todolist.color), and Smithy cannot put a `null` into an example for a
// String shape, so writing it here would force the group to advertise a color
// that uncolored groups do not have. Requiredness lives in the projection for
// the same reason, so the example that satisfies it does too. Both halves must
// move together: adding `color` to a Smithy example without removing its
// `jsonAdd` pointer would leave the pointer overwriting the value.
//
// Those two pointers end `/value/color`, NOT `/value/result/color`, and that is
// a consequence of the fix below: BareResponseExampleMapper strips the Smithy
// output wrapper off the projected example so it matches the unwrapped schema,
// and jsonAdd is applied after the mappers run. A pointer still naming `result`
// would rebuild the wrapper it just removed — as a sibling object holding only
// `color`, with no color on the body — so the two move together too.
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

// Every @examples entry below declares an `output`, and that is load-bearing
// rather than decorative. The OpenAPI converter emits a response example for
// each entry whether or not it has one, and fills an entry that has none with
// the empty object — which is how eight published 200 examples came to be `{}`,
// one of them against an array schema (#644). BareResponseExampleMapper now
// drops an outputless response example instead of publishing `{}`, and unwraps
// the ones that do carry an output, so what lands in openapi.json validates
// against the schema beside it.
//
// An output here is the Smithy WRAPPER structure (`{ recordings: [...] }`); the
// mapper unwraps it to the bare body BC3 sends, matching the schema the
// bare-response mappers rewrite. Write it wrapped.

apply ListRecordings @examples([
  {
    title: "List Todo recordings"
    documentation: "Use simple type name for basic resources"
    input: { accountId: "999", type: "Todo" }
    output: { recordings: [{
      id: 1069479523, status: "active", visible_to_clients: false,
      created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-02T00:00:00Z",
      title: "Ship the hardware", inherits_status: true, type: "Todo",
      url: "https://3.basecampapi.com/999/buckets/12345678/todos/1069479523.json",
      app_url: "https://3.basecamp.com/999/buckets/12345678/todos/1069479523",
      creator: { id: 1, name: "Someone", created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" },
      bucket: { id: 12345678, name: "My Project", type: "Project" },
      parent: { id: 987654, title: "Launch Tasks", type: "Todolist", url: "https://3.basecampapi.com/999/buckets/12345678/todolists/987654.json", app_url: "https://3.basecamp.com/999/buckets/12345678/todolists/987654" }
    }] }
  },
  {
    title: "List Kanban Card recordings"
    documentation: "Use double-colon notation for nested types"
    input: { accountId: "999", type: "Kanban::Card" }
    output: { recordings: [{
      id: 1069479524, status: "active", visible_to_clients: false,
      created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-02T00:00:00Z",
      title: "Design the enclosure", inherits_status: true, type: "Kanban::Card",
      url: "https://3.basecampapi.com/999/buckets/12345678/card_tables/cards/1069479524.json",
      app_url: "https://3.basecamp.com/999/buckets/12345678/card_tables/cards/1069479524",
      creator: { id: 1, name: "Someone", created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" },
      bucket: { id: 12345678, name: "My Project", type: "Project" },
      parent: { id: 1069479519, title: "In Progress", type: "Kanban::Column", url: "https://3.basecampapi.com/999/buckets/12345678/card_tables/lists/1069479519.json", app_url: "https://3.basecamp.com/999/buckets/12345678/card_tables/columns/1069479519" }
    }] }
  },
  {
    title: "List Question Answer recordings"
    documentation: "Another nested type example"
    input: { accountId: "999", type: "Question::Answer" }
    output: { recordings: [{
      id: 1069479525, status: "active", visible_to_clients: false,
      created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-02T00:00:00Z",
      title: "How did the week go?", inherits_status: true, type: "Question::Answer",
      url: "https://3.basecampapi.com/999/buckets/12345678/question_answers/1069479525.json",
      app_url: "https://3.basecamp.com/999/buckets/12345678/question_answers/1069479525",
      creator: { id: 1, name: "Someone", created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" },
      bucket: { id: 12345678, name: "My Project", type: "Project" },
      parent: { id: 1069479518, title: "How did the week go?", type: "Question", url: "https://3.basecampapi.com/999/buckets/12345678/questions/1069479518.json", app_url: "https://3.basecamp.com/999/buckets/12345678/questions/1069479518" }
    }] }
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
    output: { result: {
      granted: [
        { id: 111, name: "Jane Doe" },
        { id: 222, name: "John Roe" }
      ],
      revoked: []
    } }
  },
  {
    title: "Revoke access"
    documentation: "Use revoke array to remove users"
    input: { accountId: "999", projectId: 12345678, revoke: [333] }
    output: { result: {
      granted: [],
      revoked: [{ id: 333, name: "Sam Ray" }]
    } }
  },
  {
    title: "Invite new users"
    documentation: "Use create array for new users without accounts"
    input: { accountId: "999", projectId: 12345678, create: [{ name: "Jane", email_address: "jane@example.com" }] }
    output: { result: {
      granted: [{ id: 444, name: "Jane", email_address: "jane@example.com" }],
      revoked: []
    } }
  }
])

apply UpdateSubscription @examples([
  {
    title: "Add subscribers"
    input: { accountId: "999", recordingId: 987654, subscriptions: [111, 222] }
    output: { subscription: {
      subscribed: true, count: 2,
      url: "https://3.basecampapi.com/999/buckets/12345678/recordings/987654/subscription.json",
      subscribers: [
        { id: 111, name: "Jane Doe" },
        { id: 222, name: "John Roe" }
      ]
    } }
  },
  {
    title: "Remove subscribers"
    input: { accountId: "999", recordingId: 987654, unsubscriptions: [333] }
    output: { subscription: {
      subscribed: false, count: 0,
      url: "https://3.basecampapi.com/999/buckets/12345678/recordings/987654/subscription.json",
      subscribers: []
    } }
  }
])
