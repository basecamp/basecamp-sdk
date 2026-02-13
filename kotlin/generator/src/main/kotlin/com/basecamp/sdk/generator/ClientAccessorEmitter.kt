package com.basecamp.sdk.generator

/**
 * Generates service accessor properties on AccountClient.
 */
class ClientAccessorEmitter {

    /**
     * Generates ServiceAccessors.kt — an extension file that adds all
     * generated service properties to AccountClient.
     */
    fun generate(services: Map<String, ServiceDefinition>): String {
        val sb = StringBuilder()

        sb.appendLine("package com.basecamp.sdk.generated")
        sb.appendLine()
        sb.appendLine("import com.basecamp.sdk.AccountClient")
        sb.appendLine("import com.basecamp.sdk.generated.services.*")
        sb.appendLine()
        sb.appendLine("/**")
        sb.appendLine(" * Generated service accessor extensions for [AccountClient].")
        sb.appendLine(" *")
        sb.appendLine(" * These properties provide lazy, cached access to all Basecamp API services.")
        sb.appendLine(" *")
        sb.appendLine(" * @generated from OpenAPI spec — do not edit directly")
        sb.appendLine(" */")
        sb.appendLine()

        for ((name, service) in services.entries.sortedBy { it.key }) {
            val propertyName = name[0].lowercase() + name.substring(1)
            sb.appendLine("/** ${service.name} operations. */")
            sb.appendLine("val AccountClient.${propertyName}: ${service.className}")
            sb.appendLine("    get() = service(\"${name}\") { ${service.className}(this) }")
            sb.appendLine()
        }

        return sb.toString()
    }
}
