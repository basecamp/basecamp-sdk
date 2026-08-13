package com.basecamp.sdk.conformance

import java.io.File
import kotlin.io.path.createTempDirectory
import kotlin.test.Test
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.serialization.json.Json

/**
 * Case-census contract (#602).
 *
 * The check is green on the real fixture tree by construction, so a live run
 * only ever proves it can say yes. These cases run it against a SYNTHETIC
 * fixture set and prove it can say no — the `mode: "moc"` case in particular,
 * which every runner's "mock unless told otherwise" filter drops with nothing
 * printed. That divergence is asserted end-to-end here: the census and the run
 * loop's own predicate ([CaseCensus.isMockMode], shared with the load path)
 * disagree by one, and [CaseCensus.countFailure] reports it.
 */
class CaseCensusTest {
    /**
     * One case of each kind: a plain mock case (no `mode` at all, the common
     * spelling), a live case the runners are meant to drop, and a typo'd mode
     * that nothing recognizes.
     */
    private val fixture = """
        [
          {"name": "plain", "operation": "GetProject"},
          {"name": "live one", "operation": "GetProject", "mode": "live"},
          {"name": "typo", "operation": "GetProject", "mode": "moc"}
        ]
    """.trimIndent()

    private fun withFixtureTree(files: Map<String, String>, body: (File) -> Unit) {
        val dir = createTempDirectory("case-census").toFile()
        try {
            for ((relative, content) in files) {
                val file = File(dir, relative)
                file.parentFile.mkdirs()
                file.writeText(content)
            }
            body(dir)
        } finally {
            dir.deleteRecursively()
        }
    }

    @Test
    fun `census counts every case that is not explicitly live`() {
        withFixtureTree(mapOf("cases.json" to fixture)) { dir ->
            assertEquals(2, CaseCensus.nonLiveCaseCount(dir))
        }
    }

    @Test
    fun `a typoed mode makes the count check fail`() {
        // The regression this whole check exists for. The runner's own filter
        // keeps one case; the census counts two; the difference is the case
        // executed by nothing.
        withFixtureTree(mapOf("cases.json" to fixture)) { dir ->
            val json = Json { ignoreUnknownKeys = true }
            val ran = json.decodeFromString<List<TestCase>>(fixture)
                .count { CaseCensus.isMockMode(it.mode) }
            assertEquals(1, ran, "the run loop should keep only the plain case")

            val failure = CaseCensus.countFailure(ran, CaseCensus.nonLiveCaseCount(dir))

            assertNotNull(failure, "a case no runner recognizes must fail the count check")
            assertContains(failure, "1 executed by nothing")
        }
    }

    @Test
    fun `census finds fixtures nested below the tests directory`() {
        // No runner globs recursively, so a case parked one directory down is
        // run by nothing. The census walks, which is what makes that visible.
        withFixtureTree(mapOf("nested/cases.json" to fixture)) { dir ->
            assertEquals(2, CaseCensus.nonLiveCaseCount(dir))
        }
    }

    @Test
    fun `census rejects a fixture that does not parse`() {
        withFixtureTree(mapOf("broken.json" to """[{"name": "truncated"""")) { dir ->
            assertFailsWith<CaseCensus.CensusException> { CaseCensus.nonLiveCaseCount(dir) }
        }
    }

    @Test
    fun `census rejects a fixture that is not an array`() {
        withFixtureTree(mapOf("object.json" to """{"name": "not a list"}""")) { dir ->
            assertFailsWith<CaseCensus.CensusException> { CaseCensus.nonLiveCaseCount(dir) }
        }
    }

    @Test
    fun `census rejects an empty tree`() {
        // A census that counted nothing certifies nothing: zero on both sides
        // is the shape a broken walk takes.
        withFixtureTree(emptyMap()) { dir ->
            assertFailsWith<CaseCensus.CensusException> { CaseCensus.nonLiveCaseCount(dir) }
        }
    }

    @Test
    fun `census rejects an emptied fixture`() {
        // The one truncation both sides read identically: the runner registers
        // nothing from the file and the census would expect nothing, so the
        // totals fall together and no mismatch appears. Counting it as zero is
        // what would make the whole-file guarantee a lie, so the census refuses
        // it instead.
        withFixtureTree(mapOf("cases.json" to fixture, "emptied.json" to "[]")) { dir ->
            assertFailsWith<CaseCensus.CensusException> { CaseCensus.nonLiveCaseCount(dir) }
        }
    }

    @Test
    fun `count failure accepts agreement`() {
        assertNull(CaseCensus.countFailure(42, 42))
    }

    @Test
    fun `count failure names an over count`() {
        val failure = CaseCensus.countFailure(43, 42)

        assertNotNull(failure)
        assertContains(failure, "1 more than the fixtures declare")
    }

    @Test
    fun `isMockMode treats absence as mock`() {
        assertTrue(CaseCensus.isMockMode(null))
        assertTrue(CaseCensus.isMockMode("mock"))
        assertFalse(CaseCensus.isMockMode("live"))
        // The census is what catches this one; the filter must not run it.
        assertFalse(CaseCensus.isMockMode("moc"))
        // Null-coalescing, not falsiness: an explicit empty mode is not an
        // absent one. Python defaulted on falsiness and ran it.
        assertFalse(CaseCensus.isMockMode(""))
    }
}
