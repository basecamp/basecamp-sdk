// Hand-written compatibility shim, NOT generated.
//
// The spec renamed the UpdateTodo wire operation to ReplaceTodo (the PUT
// was always full-replace), so the generator now emits ReplaceTodoBody
// and no longer declares UpdateTodoBody. The type is re-declared here in
// its original package so existing caller imports keep compiling; it is
// now the request body of the hand-written merge-safe
// com.basecamp.sdk.services.TodosService.update, where a null field is
// left untouched rather than cleared.
package com.basecamp.sdk.generated.services

/** Request body for the merge-safe Todos update: null fields are untouched. */
data class UpdateTodoBody(
    val content: String? = null,
    val description: String? = null,
    val assigneeIds: List<Long>? = null,
    val completionSubscriberIds: List<Long>? = null,
    val notify: Boolean? = null,
    val dueOn: String? = null,
    val startsOn: String? = null
)
