package com.basecamp.sdk.generator

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonPrimitive
import java.io.File

/**
 * Kotlin SDK code generator.
 *
 * Reads openapi.json + behavior-model.json and generates:
 * - Model data classes (generated/models/)
 * - Service classes (generated/services/)
 * - Service types (body/options classes)
 * - Metadata.kt (per-operation retry config)
 * - ServiceAccessors.kt (AccountClient extension properties)
 *
 * Usage:
 *   ./gradlew :generator:run --args="--openapi ../openapi.json --behavior ../behavior-model.json --output sdk/src/commonMain/kotlin/com/basecamp/sdk/generated"
 */
const val OPTIONS_PARAM_ORDER_FILENAME = "options-param-order.json"

private const val COMMITTED_GENERATED_DIR = "sdk/src/commonMain/kotlin/com/basecamp/sdk/generated"

fun main(args: Array<String>) {
    var openapiPath = "../openapi.json"
    var behaviorPath = "../behavior-model.json"
    var outputDir = COMMITTED_GENERATED_DIR
    // Read the constructor-order pin from the COMMITTED tree by default, even
    // when --output points elsewhere: it records the order already shipped, so
    // regenerating into a scratch directory must not reset it.
    var optionsOrderPath = "$COMMITTED_GENERATED_DIR/$OPTIONS_PARAM_ORDER_FILENAME"

    var i = 0
    while (i < args.size) {
        when (args[i]) {
            "--openapi" -> openapiPath = args[++i]
            "--behavior" -> behaviorPath = args[++i]
            "--output" -> outputDir = args[++i]
            "--options-order" -> optionsOrderPath = args[++i]
        }
        i++
    }

    val openapiFile = File(openapiPath)
    require(openapiFile.exists()) { "OpenAPI file not found: ${openapiFile.absolutePath}" }

    val behaviorFile = File(behaviorPath)
    require(behaviorFile.exists()) { "Behavior model file not found: ${behaviorFile.absolutePath}" }

    val json = Json { ignoreUnknownKeys = true }
    val spec = json.parseToJsonElement(openapiFile.readText()) as JsonObject
    val behaviorModel = json.parseToJsonElement(behaviorFile.readText()) as JsonObject

    val outputBase = File(outputDir)
    val modelsDir = File(outputBase, "models")
    val servicesDir = File(outputBase, "services")

    modelsDir.mkdirs()
    servicesDir.mkdirs()

    // Clean generated directories before writing to remove stale files
    modelsDir.listFiles { f -> f.extension == "kt" }?.forEach { it.delete() }
    servicesDir.listFiles { f -> f.extension == "kt" }?.forEach { it.delete() }
    File(outputBase, "Metadata.kt").delete()
    File(outputBase, "ServiceAccessors.kt").delete()

    // Parse
    val api = OpenApiParser(spec)
    val parser = OperationParser(api)
    val services = parser.groupOperations()

    // 1. Generate entity models
    val modelEmitter = ModelEmitter(api)
    var modelCount = 0
    for ((schemaName, typeName) in TYPE_ALIASES) {
        val code = modelEmitter.generateModel(schemaName, typeName)
        if (code != null) {
            File(modelsDir, "$typeName.kt").writeText(code)
            modelCount++
            println("  model: $typeName.kt")
        }
    }

    // Also generate supporting model types (nested references not in TYPE_ALIASES)
    val supportingModels = findSupportingModels(api)
    for ((schemaName, typeName) in supportingModels) {
        if (typeName in TYPE_ALIASES.values) continue
        val code = modelEmitter.generateModel(schemaName, typeName)
        if (code != null) {
            File(modelsDir, "$typeName.kt").writeText(code)
            modelCount++
            println("  model: $typeName.kt (supporting)")
        }
    }

    println("Generated $modelCount models")

    // 2. Generate service classes
    val serviceEmitter = ServiceEmitter(api)
    var serviceCount = 0
    var opCount = 0
    for ((_, service) in services) {
        val code = serviceEmitter.generateService(service)
        val fileName = "${service.name.toKebabCase()}.kt"
        File(servicesDir, fileName).writeText(code)
        serviceCount++
        opCount += service.operations.size
        println("  service: $fileName (${service.operations.size} operations)")
    }
    println("Generated $serviceCount services with $opCount operations")

    // 3. Generate body/options types
    val typeEmitter = TypeEmitter(readOptionsParamOrder(File(optionsOrderPath)))
    val typesCode = typeEmitter.generateTypes(services)
    File(servicesDir, "Types.kt").writeText(typesCode)
    println("  types: Types.kt")

    // 3b. Re-pin the options-class constructor order. Written into the output
    // tree so a regenerate-and-diff drift gate compares it like any other
    // generated artifact; read (above) from the committed copy, which is the
    // shipped API's compatibility baseline.
    val orderFile = File(outputBase, OPTIONS_PARAM_ORDER_FILENAME)
    orderFile.writeText(renderOptionsParamOrder(typeEmitter.emittedParamOrder()))
    println("  order: $OPTIONS_PARAM_ORDER_FILENAME (${typeEmitter.emittedParamOrder().size} options classes)")

    // 4. Generate Metadata.kt
    val metadataEmitter = MetadataEmitter()
    val configs = metadataEmitter.parse(behaviorModel)
    val metadataCode = metadataEmitter.generate(configs)
    File(outputBase, "Metadata.kt").writeText(metadataCode)
    println("  metadata: Metadata.kt (${configs.size} operations)")

    // 5. Generate ServiceAccessors.kt
    val accessorEmitter = ClientAccessorEmitter()
    val accessorCode = accessorEmitter.generate(services)
    File(outputBase, "ServiceAccessors.kt").writeText(accessorCode)
    println("  accessors: ServiceAccessors.kt (${services.size} services)")

    println("\nDone! Generated to: ${outputBase.absolutePath}")
}

/**
 * Reads the shipped constructor order for options classes. A missing file is
 * not an error — it means no order has been pinned yet, and every class is
 * emitted in natural order and pinned from this run on.
 */
private fun readOptionsParamOrder(file: File): Map<String, List<String>> {
    if (!file.exists()) return emptyMap()
    val root = Json.parseToJsonElement(file.readText()) as JsonObject
    return root.mapValues { (_, v) -> v.jsonArray.map { it.jsonPrimitive.content } }
}

/**
 * Renders the pin. Hand-rolled rather than serialized so the on-disk shape is
 * fixed by this repo, not by kotlinx.serialization's pretty-printer: it is a
 * compatibility record read in diffs, and one array per class keeps a reorder
 * visible as a one-line change.
 */
private fun renderOptionsParamOrder(order: Map<String, List<String>>): String {
    val body = order.entries.joinToString(",\n") { (className, params) ->
        "  ${jsonString(className)}: [${params.joinToString(", ") { jsonString(it) }}]"
    }
    return "{\n$body\n}\n"
}

private fun jsonString(value: String): String = "\"" + value.replace("\\", "\\\\").replace("\"", "\\\"") + "\""

/**
 * Find model types referenced by entity schemas that aren't in TYPE_ALIASES.
 * Recursively follows references so nested supporting types are also discovered.
 * E.g., TodoParent, TodoBucket, PersonCompany, WebhookDeliveryRequest, etc.
 */
private fun findSupportingModels(api: OpenApiParser): Map<String, String> {
    val result = mutableMapOf<String, String>()
    val known = TYPE_ALIASES.keys.toMutableSet()

    fun scanSchema(schemaName: String) {
        val schema = api.getSchema(schemaName) ?: return
        val properties = schema["properties"]?.let {
            (it as? kotlinx.serialization.json.JsonObject)?.entries
        } ?: return

        for ((_, propValue) in properties) {
            val propObj = propValue as? kotlinx.serialization.json.JsonObject ?: continue

            // Direct $ref
            val ref = propObj["\$ref"]?.let { (it as? kotlinx.serialization.json.JsonPrimitive)?.content }
            if (ref != null) {
                val refName = api.resolveRef(ref)
                if (refName !in known) {
                    known += refName
                    result[refName] = refName
                    scanSchema(refName)
                }
            }

            // Array items $ref
            val items = propObj["items"]?.let { it as? kotlinx.serialization.json.JsonObject }
            val itemRef = items?.get("\$ref")?.let { (it as? kotlinx.serialization.json.JsonPrimitive)?.content }
            if (itemRef != null) {
                val refName = api.resolveRef(itemRef)
                if (refName !in known) {
                    known += refName
                    result[refName] = refName
                    scanSchema(refName)
                }
            }
        }
    }

    for ((schemaName, _) in TYPE_ALIASES) {
        scanSchema(schemaName)
    }

    return result
}
