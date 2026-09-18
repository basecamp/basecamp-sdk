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
    private const val ADDITIONAL_OPERATIONS = "additionalOperations"

    /**
     * The ordered HTTP methods the SDK generators emit.
     *
     * Read VERBATIM. scripts/check-generated-verbs.rb is the only thing that
     * rejects a malformed declaration, and it is a prerequisite of every
     * *-generate target and a member of `make check`, so nothing gets here
     * without passing it. This loader deliberately performs NO validation: six
     * loaders that each validated disagreed five times in four review rounds,
     * every one of them on invalid input, and each surviving predicate is
     * another chance to disagree. Kotlin's own instance was in the PARSE rather
     * than a check — `JsonPrimitive.content` renders a number as text — which is
     * why "validate less" was not the answer and "do not validate" is.
     */
    fun generatedVerbs(path: String): List<String> {
        val file = File(path)
        require(file.exists()) {
            "Generated-verb declaration not found: ${file.absolutePath}. Run " +
                "'make check-generated-verbs' — it is a prerequisite of every generate target."
        }
        return (Json.parseToJsonElement(file.readText()) as JsonObject)["verbs"]!!
            .jsonArray
            .map { it.jsonPrimitive.content }
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

        // OpenAPI 3.2's `additionalOperations` is a MAP of method to Operation,
        // not an operation. Read as one operation it carries no operationId, so
        // a verb-agnostic caller would drop every operation inside it without
        // saying so. Refuse by name until someone teaches the walk the map shape.
        require(!pathItem.containsKey(ADDITIONAL_OPERATIONS)) {
            "openapi.json path $path declares `$ADDITIONAL_OPERATIONS`, which OpenAPI 3.2 defines " +
                "as a map of method to Operation. This walk reads a path-item field as a single " +
                "operation, so it would drop every operation inside it. Teach the walk the map " +
                "shape, or take the field out of the spec."
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
                // An operation has to be IDENTIFIABLE. OpenAPI lets operationId
                // be omitted, and every walker here used to step over one that
                // was — a silent drop of a real operation, which is
                // basecamp-sdk#925 wearing a different field.
                // `isString` before `content`, for the same reason the declaration
                // loader above checks it: JsonPrimitive renders a number or a
                // boolean as text, so `operationId: 123` would satisfy a bare
                // content check here and name a generated method `123` while the
                // other five walkers reject it.
                val idPrimitive = operation["operationId"] as? JsonPrimitive
                val operationId = idPrimitive?.takeIf { it.isString }?.content
                require(!operationId.isNullOrEmpty()) {
                    "openapi.json declares ${field.uppercase()} $path with no operationId. " +
                        "Everything downstream is keyed by it, and skipping the operation would " +
                        "drop it from the SDK in silence."
                }
                field to operation
            }
    }
}
