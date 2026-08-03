// Hand-written companion type, NOT generated.
//
// The spec renamed the UpdateDocument wire operation to ReplaceDocument (the
// PUT was always full-replace), so the generator now emits ReplaceDocumentBody
// and no longer declares UpdateDocumentBody. The name is re-declared here in
// the generated package — alongside the UpdateTodoBody and UpdateTodolistBody
// shims that arrived the same way — as the request body of the hand-written
// merge-safe com.basecamp.sdk.services.DocumentsService.update, where a null
// field is left untouched rather than cleared.
//
// This is NOT a deprecated alias for the old operation: ReplaceDocument ships
// without one (the ReplaceTodo precedent, #375), and the two types differ in
// meaning — null here preserves, where the generated body's null omits and the
// server clears.
package com.basecamp.sdk.generated.services

/** Request body for the merge-safe Documents update: null fields are untouched. */
data class UpdateDocumentBody(
    val title: String? = null,
    val content: String? = null
)
