package com.basecamp.sdk.serialization

import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Reads a required member out of a wrapped response envelope, refusing an
 * absent one as a decode failure.
 *
 * A wrapped-pagination response (`GetPersonProgress`) is not a shape any
 * generated model covers: the items array is paginated across pages and the
 * remaining members come off the first page, so the generator reaches into the
 * envelope by name instead of decoding it whole. That leaves the "member is
 * absent" case to be raised by hand, and the type it is raised as is the whole
 * point of this function. [SerializationException] is the one exception type
 * `BaseService.decodeOrApiError` maps to SPEC §6's statusless `api_error`; the
 * `!!` this replaces raised [NullPointerException], which that mapping refuses
 * to catch and should refuse to catch, because it cannot tell a wrong-shaped
 * response from a genuine programming error (#728).
 *
 * Absence is a malformed body rather than an empty result because BC3 writes
 * every member of these envelopes unconditionally — `person` and `events` are
 * two bare `json.` lines in `app/views/api/users/timelines/show.json.jbuilder`,
 * with no `if` between them.
 *
 * The message names the member only. The operation and the "does not decode"
 * framing are added by the mapping, which is the one place that knows them.
 */
internal fun JsonObject.requiredMember(name: String): JsonElement =
    this[name] ?: throw SerializationException("required member '$name' is absent from the response wrapper")
