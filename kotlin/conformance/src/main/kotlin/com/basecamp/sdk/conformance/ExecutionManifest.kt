package com.basecamp.sdk.conformance

import java.io.File
import java.io.IOException
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

/**
 * One runner's exclusion set for the cross-runner gate (#602).
 *
 * The case census answers "did THIS runner account for every case". A case
 * every runner excludes leaves all six censuses green, because each one counted
 * its own skip — only `scripts/check-fixture-execution.rb`, comparing these
 * manifests, can see it.
 *
 * [executed] is recorded alongside the exclusions and asserted against the
 * census total. Without it, a case a runner silently dropped is simply absent
 * from the exclusion set, and "absent" reads identically to "ran fine" — the
 * collecting gate would conclude the case is covered by this runner precisely
 * when it was not.
 *
 * Kotlin excludes a case through THREE paths — the `link-header` tag branch,
 * the [KOTLIN_SKIPS] roster, and a runtime `TestResult.skipped` — and all three
 * record here, from the same branches that increment `skipped`.
 */
object ExecutionManifest {
    class Error(message: String) : Exception(message)

    @kotlinx.serialization.Serializable
    data class Exclusion(val name: String, val reason: String)

    @kotlinx.serialization.Serializable
    data class Body(
        val runner: String,
        val total_non_live: Int,
        val executed: Int,
        val excluded: List<Exclusion>,
    )

    private val json = Json { prettyPrint = true }

    /** Sorted, so a re-run is byte-identical. */
    fun write(runner: String, total: Int, executed: Int, excluded: List<Exclusion>) {
        if (executed + excluded.size != total) {
            throw Error(
                "manifest for $runner is internally inconsistent: $executed executed + " +
                    "${excluded.size} excluded != $total non-live cases; the run dropped a case " +
                    "without recording it as either"
            )
        }

        val dir = File("../conformance/manifests")
        try {
            dir.mkdirs()
            val body = Body(runner, total, executed, excluded.sortedBy { it.name })
            File(dir, "$runner.json").writeText(json.encodeToString(body) + "\n")
        } catch (e: IOException) {
            throw Error("could not write $runner manifest: ${e.message}")
        }
    }
}
