package com.basecamp.sdk.generator

import kotlinx.serialization.json.*
import java.io.File

/**
 * Walking an OpenAPI Path Item Object (basecamp-sdk#925).
 *
 * A path item is read by EXCLUSION. Its non-operation fields are a closed,
 * spec-defined set and its extensions are `x-` prefixed, so every OTHER field
 * is an operation. Enumerating the verbs instead is the defect this replaces:
 * Smithy's `@http` trait takes the method as a free-form string it "will use
 * literally and will perform no validation on", so a model author writing
 * `method: "HEAD"` produced a valid model, a valid openapi.json, and no method
 * on any client — the verb was not in the list, so the operation was stepped
 * over in silence.
 */
object PathItems {
    private val NON_OPERATION_FIELDS = setOf("summary", "description", "servers", "parameters")

    /**
     * The ordered HTTP methods the SDK generators emit — one declaration for
     * all six SDKs. See spec/generated-verbs.json for why it is a policy
     * rather than six per-language capabilities.
     */
    fun generatedVerbs(path: String): List<String> {
        val file = File(path)
        require(file.exists()) { "Generated-verb declaration not found: ${file.absolutePath}" }
        val verbs = (Json.parseToJsonElement(file.readText()) as JsonObject)["verbs"]
            ?.jsonArray
            ?.map { it.jsonPrimitive.content }
            ?: error("$path must declare a `verbs` array")
        require(verbs.isNotEmpty() && verbs.none { it.isEmpty() }) {
            "$path must declare a non-empty `verbs` array of strings"
        }
        return verbs
    }

    /**
     * Every operation in one path item, as (verb, operation) pairs.
     *
     * [emittable] bounds what the caller can RENDER, and is checked after
     * discovery: an operation on any other verb stops the run by name rather
     * than being dropped. Pass null from a caller that is verb-agnostic.
     *
     * Visit order follows [order] so emitted output stays byte-stable; a verb
     * outside it sorts deterministically to the end by name, which is
     * ordering, not membership.
     */
    fun operationsOf(
        path: String,
        pathItem: JsonObject,
        order: List<String>,
        emittable: List<String>?,
    ): List<Pair<String, JsonObject>> {
        // A `$ref` path item points at operations this walk cannot see without
        // resolving the reference. Skipping it is the same silent under-count
        // the exclusion walk exists to prevent, so refuse instead.
        require(!pathItem.containsKey("\$ref")) {
            "openapi.json path $path is a \$ref; resolving a path-item reference is not " +
                "implemented, and skipping it would hide every operation behind it from the SDK."
        }

        return pathItem.keys
            .filter { it !in NON_OPERATION_FIELDS && !it.startsWith("x-") }
            .sortedWith(compareBy({ order.indexOf(it).takeIf { at -> at >= 0 } ?: order.size }, { it }))
            .map { field ->
                val operation = pathItem[field] as? JsonObject
                    ?: error(
                        "openapi.json path $path field \"$field\" is neither a known non-operation " +
                            "field nor an operation object. If a later OpenAPI version added it, " +
                            "add it to NON_OPERATION_FIELDS with a reason."
                    )
                if (emittable != null && field !in emittable) {
                    val opId = operation["operationId"]?.jsonPrimitive?.content ?: "(no operationId)"
                    error(
                        "openapi.json declares ${field.uppercase()} $path ($opId), and this " +
                            "generator emits only ${emittable.joinToString("/") { it.uppercase() }}. " +
                            "Generating the rest of the SDK without it would drop the operation " +
                            "from every client in silence, which is the failure basecamp-sdk#925 " +
                            "closed. Give the runtime a $field helper and add \"$field\" to " +
                            "spec/generated-verbs.json (read that file first — the other five SDKs " +
                            "need the same helper), or take the operation out of the Smithy model."
                    )
                }
                field to operation
            }
    }
}
