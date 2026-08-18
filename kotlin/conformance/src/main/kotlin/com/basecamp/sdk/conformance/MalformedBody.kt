package com.basecamp.sdk.conformance

import com.basecamp.sdk.BasecampException
import kotlinx.serialization.SerializationException

/**
 * The response decoder's own refusal carried by [e], or null when [e] is any
 * other failure.
 *
 * Since #604 a body the model refuses arrives here as a `BasecampException.Api`
 * rather than a raw `SerializationException`, so the runner has to ask which
 * shape it is holding: a decoder rejection means "repair the fixture body"
 * (#555), and every other `api_error` is a result the assertions get to see.
 *
 * **Ask the SDK, do not re-derive.** The test this replaces was
 * `(e as? BasecampException.Api)?.cause as? SerializationException`, which is
 * the #730 bug: `BasecampHttpClient` propagates an already-classified
 * `BasecampException` from the auth strategy untouched, so an `AuthStrategy`
 * that classifies its own JSON failure as `Api(cause = SerializationException(…))`
 * — through the public constructor, which leaves `decodeFailure` null — matched
 * it and was reported as a malformed mock body for a request that was never
 * sent. #730 fixed the three §18 composites by giving the SDK a structural slot
 * and left this module reading the old signal; the two agreed only because the
 * decoder's own refusal fills both, and diverged on exactly the case the slot
 * exists for. `decodeFailure` is public for this caller: `:conformance` is a
 * separate Gradle module and cannot see `internal`.
 *
 * Split out of the runner so both directions are unit-testable
 * (MalformedBodyTest). Only the positive one is reachable from
 * `conformance/tests/`: no fixture can supply a custom `AuthStrategy`, so
 * nothing committed would have failed while the misread was live.
 */
fun malformedBodyFailure(e: BasecampException): SerializationException? =
    (e as? BasecampException.Api)?.decodeFailure
