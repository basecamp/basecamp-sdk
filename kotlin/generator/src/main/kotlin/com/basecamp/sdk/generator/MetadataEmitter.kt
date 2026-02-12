package com.basecamp.sdk.generator

import kotlinx.serialization.json.*

/**
 * Generates Metadata.kt with per-operation retry configuration from behavior-model.json.
 */
class MetadataEmitter {

    data class RetryConfig(
        val max: Int,
        val baseDelayMs: Long,
        val backoff: String,
        val retryOn: List<Int>,
    )

    data class OperationConfig(
        val operationId: String,
        val idempotent: Boolean,
        val retry: RetryConfig?,
    )

    fun parse(behaviorModel: JsonObject): List<OperationConfig> {
        val operations = behaviorModel["operations"]?.jsonObject ?: return emptyList()

        return operations.entries.map { (opId, config) ->
            val obj = config.jsonObject
            val idempotent = obj["idempotent"]?.jsonPrimitive?.boolean ?: false
            val retry = obj["retry"]?.jsonObject?.let { r ->
                RetryConfig(
                    max = r["max"]!!.jsonPrimitive.int,
                    baseDelayMs = r["base_delay_ms"]!!.jsonPrimitive.long,
                    backoff = r["backoff"]!!.jsonPrimitive.content,
                    retryOn = r["retry_on"]!!.jsonArray.map { it.jsonPrimitive.int },
                )
            }
            OperationConfig(opId, idempotent, retry)
        }.sortedBy { it.operationId }
    }

    fun generate(configs: List<OperationConfig>): String {
        val sb = StringBuilder()

        sb.appendLine("package com.basecamp.sdk.generated")
        sb.appendLine()
        sb.appendLine("/**")
        sb.appendLine(" * Per-operation metadata from behavior-model.json.")
        sb.appendLine(" *")
        sb.appendLine(" * @generated from behavior-model.json — do not edit directly")
        sb.appendLine(" */")
        sb.appendLine("object Metadata {")
        sb.appendLine()
        sb.appendLine("    data class RetryConfig(")
        sb.appendLine("        val maxRetries: Int,")
        sb.appendLine("        val baseDelayMs: Long,")
        sb.appendLine("        val backoff: String,")
        sb.appendLine("        val retryOn: Set<Int>,")
        sb.appendLine("    )")
        sb.appendLine()
        sb.appendLine("    data class OperationConfig(")
        sb.appendLine("        val idempotent: Boolean,")
        sb.appendLine("        val retry: RetryConfig?,")
        sb.appendLine("    )")
        sb.appendLine()
        sb.appendLine("    val operations: Map<String, OperationConfig> = mapOf(")

        for (config in configs) {
            val retryStr = if (config.retry != null) {
                val r = config.retry
                "RetryConfig(${r.max}, ${r.baseDelayMs}L, \"${r.backoff}\", setOf(${r.retryOn.joinToString(", ")}))"
            } else {
                "null"
            }
            sb.appendLine("        \"${config.operationId}\" to OperationConfig(${config.idempotent}, $retryStr),")
        }

        sb.appendLine("    )")
        sb.appendLine("}")

        return sb.toString()
    }
}
