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
    private val VERB_PATTERN = Regex("[a-z]+")

    /**
     * The ordered HTTP methods the SDK generators emit — one declaration for
     * all six SDKs. See spec/generated-verbs.json for why it is a policy
     * rather than six per-language capabilities.
     */
    fun generatedVerbs(path: String): List<String> {
        val file = File(path)
        require(file.exists()) { "Generated-verb declaration not found: ${file.absolutePath}" }
        val declared = (Json.parseToJsonElement(file.readText()) as JsonObject)["verbs"]
            ?.jsonArray
            ?: error("$path must declare a `verbs` array")
        // Every entry must be a JSON STRING matching VERB_PATTERN. The type check
        // is not redundant: `jsonPrimitive.content` renders a number or a boolean
        // as text, so `["get", 1]` would otherwise read as `get`/`1` here while
        // the other loaders reject it.
        //
        // The shape rule is a positive character class rather than a blankness
        // predicate, and deliberately so. Two review rounds chased "blank" across
        // languages — a space, then a non-breaking space — and the next
        // disagreement was guaranteed, because every language defines whitespace
        // differently (Java's Character.isWhitespace, which backs Kotlin's
        // isBlank, excludes U+00A0; Rust's char::is_whitespace includes it;
        // Ruby's String#strip is ASCII-only). An HTTP method is a token, so
        // `[a-z]+` says what a verb IS, is written the same way in all six
        // loaders, and leaves no seam for case, normalisation or a leading BOM
        // to open later.
        val verbs = declared.map { element ->
            val primitive = element as? JsonPrimitive
            require(primitive != null && primitive.isString && VERB_PATTERN.matches(primitive.content)) {
                "$path must declare a non-empty `verbs` array of lowercase ASCII method names " +
                    "(/[a-z]+/); got $element"
            }
            primitive.content
        }
        require(verbs.isNotEmpty()) { "$path must declare a non-empty `verbs` array" }
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
        element: JsonElement,
        order: List<String>,
        emittable: List<String>?,
    ): List<Pair<String, JsonObject>> {
        // The cast happens HERE, not at the call site. Calling `.jsonObject` on
        // the way in threw a bare kotlinx type exception that named neither the
        // path nor the problem, so a malformed path item failed with a generic
        // message where the other five generators name it — and a test asserting
        // only on exit status could not tell the two apart.
        val pathItem = element as? JsonObject
            ?: error(
                "openapi.json path $path is a ${element::class.simpleName}, not a path item object."
            )
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
