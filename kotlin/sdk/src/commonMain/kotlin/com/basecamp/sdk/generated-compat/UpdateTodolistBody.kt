// Hand-written compatibility shim, NOT generated.
//
// The Todolists PUT is full-replace (BC3 rebuilds the recordable from only the
// permitted params, so omission clears), so the generator emits exactly one
// request body for it — UpdateTodolistOrGroupBody, whose `name` is required
// and non-null, the shape the wire operation demands. The merge-safe
// com.basecamp.sdk.services.TodolistsService.update needs the other shape: a
// per-field nullable body where null means "leave this alone". That type is
// declared here, in the generated request-body package so callers import it
// from the same place as every other body, alongside its Todos sibling.
package com.basecamp.sdk.generated.services

/** Request body for the merge-safe Todolists update: null fields are untouched. */
data class UpdateTodolistBody(
    val name: String? = null,
    val description: String? = null
)
