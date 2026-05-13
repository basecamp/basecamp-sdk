package com.basecamp.sdk.conformance

import com.basecamp.sdk.generated.models.Person
import com.basecamp.sdk.generated.models.Project
import com.basecamp.sdk.generated.models.Todo
import com.basecamp.sdk.generated.models.Todolist
import com.basecamp.sdk.generated.models.Todoset
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import java.io.File
import java.io.IOException
import java.nio.file.Path
import java.nio.file.Paths
import kotlin.system.exitProcess

/**
 * Wire-replay runner for the Kotlin SDK conformance suite.
 *
 * Reads canonical wire snapshots written by the TS live runner (see
 * `conformance/runner/typescript/live-runner.test.ts`), decodes each page
 * through the Kotlin SDK's deserialization boundary, walks the raw JSON for
 * required-field and extras detection, and persists per-test decode-result
 * snapshots at `<WIRE_REPLAY_DIR>/<BACKEND>/decode/kotlin/<safe>.json`.
 *
 * Mode-gate: invoked only when `WIRE_REPLAY_DIR` is set (Makefile target
 * `conformance-kotlin-replay`). The existing mock runner (`Main.kt`) handles
 * the unset case and is unaffected by anything in this file.
 *
 * Decode boundary
 * ---------------
 * Where the Kotlin SDK has a typed model for the response payload (`Project`,
 * `Person`, `Todo`, `Todolist`, `Todoset`), the decoder routes through that
 * type — exactly what the generated service method does at runtime. The four
 * `My*` operations (assignments, completed assignments, due assignments,
 * notifications) currently return `JsonElement` from the SDK because
 * `MyAssignment` and `Notification` lack typed Kotlin models; for those we
 * decode as `JsonElement`, which mirrors the SDK's actual surface.
 *
 * The `Json` instance below intentionally mirrors mock-mode (`ignoreUnknownKeys
 * = true`, `coerceInputValues = false`) — additive BC5 fields must NOT decode
 * as errors here. This is a forward-compat canary, not a strictness audit.
 * Type mismatches and missing required (non-nullable) fields still throw and
 * surface as `decode_error`. Extras detection comes from the schema walker on
 * the parsed `JsonElement`, not from the decoder.
 */
private val replayJson = Json {
    ignoreUnknownKeys = true
    coerceInputValues = false
    isLenient = false
}

/** Walker uses an unconfigured Json — only parses the body for tree access. */
private val parserJson = Json

const val REPLAY_SCHEMA_VERSION = 1

/**
 * Decoders per operation. Each takes the raw bodyText for one page and
 * either completes normally (decode succeeded) or throws (decode failed).
 *
 * For `JsonElement`-returning SDK methods, we decode as `JsonElement` —
 * that's the SDK's actual decode boundary today. When the SDK adds typed
 * models for those operations, switch the decoder to the typed model.
 */
private val decoders: Map<String, (String) -> Unit> = mapOf(
    "ListProjects" to { bt -> replayJson.decodeFromString(ListSerializer(Project.serializer()), bt) },
    "GetProject" to { bt -> replayJson.decodeFromString(Project.serializer(), bt) },
    "GetMyAssignments" to { bt -> replayJson.decodeFromString(JsonElement.serializer(), bt) },
    "GetMyCompletedAssignments" to { bt -> replayJson.decodeFromString(JsonElement.serializer(), bt) },
    "GetMyDueAssignments" to { bt -> replayJson.decodeFromString(JsonElement.serializer(), bt) },
    "GetMyNotifications" to { bt -> replayJson.decodeFromString(JsonElement.serializer(), bt) },
    "GetMyProfile" to { bt -> replayJson.decodeFromString(Person.serializer(), bt) },
    "GetTodoset" to { bt -> replayJson.decodeFromString(Todoset.serializer(), bt) },
    "ListTodolists" to { bt -> replayJson.decodeFromString(ListSerializer(Todolist.serializer()), bt) },
    "ListTodos" to { bt -> replayJson.decodeFromString(ListSerializer(Todo.serializer()), bt) },
)

private val safeNameRegex = Regex("[^a-z0-9_-]+", RegexOption.IGNORE_CASE)
private fun safeName(s: String): String = safeNameRegex.replace(s, "_")

@Serializable
data class ReplayPageResult(
    val decoded: Boolean,
    val decode_error: String?,
    val missing_required: List<String>,
    val extras_seen: List<String>,
)

@Serializable
data class ReplayResult(
    val schema_version: Int,
    val operation: String,
    val pages: List<ReplayPageResult>,
)

class ReplayRunner(
    private val replayDir: Path,
    private val backend: String,
    fixturePath: Path,
    openapiPath: Path,
) {
    private val walker = SchemaWalker(openapiPath.toString())
    private val fixture: List<JsonObject> = run {
        val all = parserJson.parseToJsonElement(File(fixturePath.toString()).readText()).jsonArray
        all.mapNotNull { el ->
            val obj = el as? JsonObject ?: return@mapNotNull null
            obj.takeIf { it["mode"]?.jsonPrimitive?.contentOrNull == "live" }
        }
    }

    fun coverageGate(): List<String> {
        val msgs = mutableListOf<String>()
        val fixtureOps = fixture.mapNotNull { it["operation"]?.jsonPrimitive?.contentOrNull }
            .distinct().sorted()

        // 1. Decoder coverage — every fixture operation must have a decoder.
        val missing = fixtureOps.filterNot { decoders.containsKey(it) }
        if (missing.isNotEmpty()) {
            msgs += "Kotlin replay runner missing decoders for: ${missing.joinToString(", ")}. " +
                "Add to decoders in ReplayRunner.kt."
        }

        // 2. Snapshot completeness — every fixture op needs a wire file.
        val wireDir = replayDir.resolve(backend).resolve("wire").toFile()
        for (t in fixture) {
            val name = t["name"]?.jsonPrimitive?.contentOrNull ?: continue
            val op = t["operation"]?.jsonPrimitive?.contentOrNull ?: continue
            val f = File(wireDir, "${safeName(name)}.json")
            if (!f.exists()) {
                msgs += "Snapshot missing for operation $op (test \"$name\"); expected at " +
                    "${f.path}. Re-run TS live capture or check skip status."
            }
        }

        // 3. Snapshot recognition — every captured snapshot's operation must
        //    be in the shared fixture (catches TS-side dispatch drift).
        if (wireDir.exists()) {
            wireDir.listFiles { f -> f.extension == "json" }?.forEach { f ->
                val text = try {
                    f.readText()
                } catch (e: IOException) {
                    msgs += "Snapshot ${f.name} could not be read: ${e::class.simpleName}: ${e.message}."
                    return@forEach
                }
                val parsed = try {
                    parserJson.parseToJsonElement(text)
                } catch (e: SerializationException) {
                    msgs += "Snapshot ${f.name} is not valid JSON: ${e.message}."
                    return@forEach
                }
                val snap = parsed as? JsonObject
                if (snap == null) {
                    msgs += "Snapshot ${f.name} top-level JSON is not an object; expected the wire-snapshot envelope."
                    return@forEach
                }
                // Defensive: `.jsonPrimitive` on JsonNull/JsonObject/JsonArray
                // throws, which would crash the gate. Cast first so a malformed
                // `operation` value emits the gate message instead.
                val op = (snap["operation"] as? JsonPrimitive)?.contentOrNull
                if (op == null) {
                    msgs += "Snapshot ${f.name} is missing the top-level `operation` field. " +
                        "Re-run the TS live canary; pre-PR3 snapshots are no longer supported."
                    return@forEach
                }
                if (op !in fixtureOps) {
                    msgs += "Unknown operation \"$op\" in snapshot ${f.name}; TS dispatch " +
                        "table appears to have drifted from live-my-surface.json."
                }
            }
        }
        return msgs
    }

    fun run(): Int {
        val msgs = coverageGate()
        if (msgs.isNotEmpty()) {
            msgs.forEach { System.err.println(it) }
            return 1
        }

        val outDir = replayDir.resolve(backend).resolve("decode").resolve("kotlin").toFile()
        outDir.mkdirs()

        val pretty = Json { prettyPrint = true }
        var failures = 0
        for (t in fixture) {
            val name = t["name"]!!.jsonPrimitive.content
            val snap = readSnapshot(name)
            val result = decodeSnapshot(snap)
            File(outDir, "${safeName(name)}.json").writeText(
                pretty.encodeToString(ReplayResult.serializer(), result)
            )
            if (result.pages.any { !it.decoded || it.missing_required.isNotEmpty() }) failures++
        }
        return if (failures == 0) 0 else 1
    }

    private fun readSnapshot(testName: String): JsonObject {
        val path = replayDir.resolve(backend).resolve("wire")
            .resolve("${safeName(testName)}.json").toFile()
        return parserJson.parseToJsonElement(path.readText()).jsonObject
    }

    private fun decodeSnapshot(snap: JsonObject): ReplayResult {
        val operation = snap["operation"]!!.jsonPrimitive.content
        val decoder = decoders[operation]!!
        val schema = walker.findResponseSchema(operation)

        val pages = snap["pages"]!!.jsonArray.map { p ->
            val page = p.jsonObject
            // Prefer bodyText (raw); fall back to serializing `body` if absent.
            val bodyText = page["bodyText"]?.jsonPrimitive?.contentOrNull
                ?: page["body"]?.let { parserJson.encodeToString(JsonElement.serializer(), it) }
                ?: ""

            var decoded = false
            var decodeError: String? = null
            try {
                decoder(bodyText)
                decoded = true
            } catch (e: Exception) {
                // Catch Exception, not Throwable: fatal JVM errors (OOM,
                // StackOverflow, LinkageError) shouldn't be silently demoted
                // to a decode-failure record.
                decodeError = "${e::class.simpleName}: ${e.message}"
            }

            var missing: List<String> = emptyList()
            var extras: List<String> = emptyList()
            if (schema != null && bodyText.isNotEmpty()) {
                try {
                    val body = parserJson.parseToJsonElement(bodyText)
                    missing = walker.missingRequired(body, schema)
                    extras = walker.extrasSeen(body, schema)
                } catch (_: Exception) {
                    // Body is not parseable JSON; decode_error above already
                    // records the failure. Leave walker output empty.
                }
            }

            ReplayPageResult(decoded, decodeError, missing, extras)
        }
        return ReplayResult(REPLAY_SCHEMA_VERSION, operation, pages)
    }
}

fun main() {
    // Treat empty/blank as missing — Gradle and shells can pass empty strings
    // through, and those would otherwise slip past a `?: null` guard and let
    // the runner read/write relative to CWD or `<dir>/<empty>/...`.
    val replayDir = System.getenv("WIRE_REPLAY_DIR")?.takeUnless { it.isBlank() }
        ?: run { System.err.println("WIRE_REPLAY_DIR is required"); exitProcess(1) }
    val backend = System.getenv("BASECAMP_BACKEND")?.takeUnless { it.isBlank() }
        ?: run { System.err.println("BASECAMP_BACKEND is required"); exitProcess(1) }

    // Resolve repo paths relative to the kotlin/ root. The Gradle `runReplay`
    // task sets workingDir = rootProject.projectDir (i.e. the repo's `kotlin/`
    // directory), so repoRoot is the parent.
    val kotlinRoot = Paths.get("").toAbsolutePath()
    val repoRoot = kotlinRoot.parent
        ?: error("Cannot resolve repo root from CWD=$kotlinRoot")
    val fixturePath = repoRoot.resolve("conformance/tests/live-my-surface.json")
    val openapiPath = repoRoot.resolve("openapi.json")

    val runner = ReplayRunner(Paths.get(replayDir), backend, fixturePath, openapiPath)
    exitProcess(runner.run())
}
