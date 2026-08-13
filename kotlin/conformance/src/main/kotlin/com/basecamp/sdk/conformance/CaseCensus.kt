package com.basecamp.sdk.conformance

import java.io.File
import java.io.IOException
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull

/**
 * Case census (#602): every non-live fixture case must be accounted for by the
 * run.
 *
 * ```
 * passed + failed + skipped  ==  every JSON case under conformance/tests,
 *                                recursively, whose mode != "live"
 * ```
 *
 * (Spelled out rather than written as a glob: Kotlin nests block comments, so a
 * `tests/` followed by two stars opens one inside this KDoc and swallows the
 * rest of the file.)
 *
 * The left side is what the runner actually did. The right side is counted by
 * [nonLiveCaseCount] below — a SEPARATE walk and parse, deliberately not the
 * runner's own load path. That independence is the entire point: a check fed by
 * the load path can only confirm the load path agrees with itself.
 *
 * Why `mode != "live"` rather than `mode == "mock"`: all six runners select with
 * "mock unless told otherwise" ([isMockMode] here, and its five equivalents), so
 * a typo'd `mode: "moc"` is dropped by every runner at once with nothing printed
 * anywhere. Counting the expected side as "not explicitly live" turns that
 * silent divergence into arithmetic.
 *
 * Catches: an unrecognized `mode`; a fixture that failed to parse or was never
 * globbed (including one nested below `conformance/tests/`, which no runner
 * discovers — hence the recursive walk); a case dropped between load and
 * dispatch; a fixture emptied to `[]` (which the census REFUSES rather than
 * counts — see [nonLiveCaseCount], and note that counting it would make this
 * bullet a lie); and any future skip channel that bypasses the counters,
 * because the counters are what it reads.
 *
 * The typo is not this check's alone to catch, and saying so is what keeps the
 * rest of the list honest: `make conformance-fixtures-check` validates the
 * top-level fixtures against `conformance/schema.json`, whose `mode` is
 * `enum: ["mock", "live"]`, so a typo in a TOP-LEVEL fixture fails there first
 * and this census is defense in depth for that one case. What that gate
 * structurally cannot see is everything else above — its glob is not recursive,
 * so a fixture nested below `conformance/tests` is validated by nothing AND run
 * by nothing (verified: such a file passes the schema gate and fails this
 * census); a fixture truncated to `[]` is a valid array of zero cases; and a
 * case dropped between load and dispatch is not a fixture-format question at
 * all. Nor does that gate run when `make conformance-<lang>` is invoked alone.
 *
 * Does NOT catch the all-six case #602 names — one case every runner excludes
 * for its own reason, which leaves each runner's own census green. That needs
 * the six exclusion sets in one place, hence artifact plumbing across six CI
 * jobs; #602 stays open for it.
 *
 * Kept apart from the run loop so it is unit-testable (`CaseCensusTest`).
 */
object CaseCensus {
    /**
     * Raised for every fail-closed condition below, so a caller cannot catch
     * the parse failure and miss the empty-tree one.
     */
    class CensusException(message: String) : Exception(message)

    private val json = Json { ignoreUnknownKeys = true }

    /**
     * Whether a fixture case's `mode` selects this runner.
     *
     * Kotlin's [TestCase.mode] already defaults to `"mock"` when the key is
     * absent, so this is passed a non-null value from the run loop — but the
     * census reads raw JSON where the key really can be missing, and both must
     * apply the same rule. Live cases are TS-only (the canonical wire-capturer)
     * and every other value is nobody's.
     */
    fun isMockMode(mode: String?): Boolean = (mode ?: "mock") == "mock"

    /**
     * Counts fixture cases whose mode is not `"live"`, recursively.
     *
     * Fail-closed in three places, each a way the count could certify nothing
     * while looking green: an unreadable tree, a fixture that does not parse,
     * and a walk that found no fixture files at all.
     */
    fun nonLiveCaseCount(testsDir: File): Int {
        // onFail is not optional decoration. WITHOUT it, FileTreeWalk silently
        // skips a directory whose listFiles() fails and keeps going: the
        // subtree vanishes from the census, the runner never listed it either
        // (it lists only the top level), and the two sides agree on a count
        // that omits it — a fail-closed check quietly failing open.
        val files = testsDir.walkTopDown()
            .onFail { file, e -> throw CensusException("could not walk ${file.path}: ${e.message}") }
            .filter { it.isFile && it.extension == "json" }
            .sortedBy { it.path }
            .toList()
        if (files.isEmpty()) {
            throw CensusException("no *.json fixture files found under ${testsDir.absolutePath}")
        }

        return files.sumOf { file ->
            val parsed = try {
                json.parseToJsonElement(file.readText()) as? JsonArray
                    ?: throw CensusException("${file.path}: fixture is not a JSON array")
            } catch (e: SerializationException) {
                throw CensusException("${file.path}: ${e.message}")
            } catch (e: IOException) {
                throw CensusException("${file.path}: ${e.message}")
            }
            // An emptied fixture is REFUSED, not counted as zero, and this is
            // the one rejection that carries the whole-file guarantee. It is
            // the single truncation both sides of the census read identically:
            // the runner registers nothing from the file and the census expects
            // nothing, so the two totals fall together and no mismatch ever
            // appears. Counting it would make "a fixture truncated to []" a
            // claim this check cannot keep. A file declaring no cases tests
            // nothing, so refusing it costs nothing — and it closes the same
            // hole in conformance-fixtures-check, where an empty array is a
            // schema-valid list of zero items.
            if (parsed.isEmpty()) {
                throw CensusException(
                    "${file.path}: fixture declares no cases; delete the file or restore its cases"
                )
            }
            // Only `mode` is read: the census must survive a fixture whose
            // other fields this runner cannot model, or it would report a
            // failure for a case the run itself handled fine.
            parsed.count { element ->
                // `as?` throughout: a fixture element that is not an object, or
                // a `mode` that is not a string, must not crash the census —
                // neither is a live case, and both are the schema gate's to
                // reject.
                ((element as? JsonObject)?.get("mode") as? JsonPrimitive)?.contentOrNull != "live"
            }
        }
    }

    /**
     * Compares what the run accounted for against the census, returning null
     * when they agree and a message naming the short side otherwise.
     */
    fun countFailure(ran: Int, expected: Int): String? = when {
        ran == expected -> null
        ran < expected ->
            "case census: the run accounted for $ran case(s) (passed+failed+skipped) " +
                "but conformance/tests holds $expected non-live case(s) — " +
                "${expected - ran} executed by nothing. An unrecognized `mode`, a fixture " +
                "that failed to parse or was never globbed, or a case dropped between load " +
                "and dispatch will do this."
        else ->
            "case census: the run accounted for $ran case(s) (passed+failed+skipped) " +
                "but conformance/tests holds only $expected non-live case(s) — " +
                "${ran - expected} more than the fixtures declare."
    }
}
