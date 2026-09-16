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
        // A file banner, NOT a KDoc: `/*`, not `/**`. Kotlin has no file-level
        // doc comment, so a `/**` block here documents whatever declaration
        // follows it — and the first thing this emitter writes after the banner
        // is another `/**`, which wins. The banner would then document nothing
        // and Dokka would render none of it. `/*` says "prose about the file"
        // and is the shape the hand-written sources use for the same job.
        // scripts/check-orphaned-doc-comments.py fails the build if this
        // regresses to `/**`. Mentions.kt is the hand-written instance of the
        // same banner, brackets and all.
        sb.appendLine("/*")
        sb.appendLine(" * Generated service accessor extensions for [AccountClient].")
        sb.appendLine(" *")
        sb.appendLine(" * These properties provide lazy, cached access to all Basecamp API services.")
        sb.appendLine(" *")
        sb.appendLine(" * @generated from OpenAPI spec — do not edit directly")
        sb.appendLine(" */")
        sb.appendLine()

        for ((name, service) in services.entries.sortedBy { it.key }) {
            val propertyName = name[0].lowercase() + name.substring(1)
            // Hand-written subclasses are constructed and declared by their
            // fully-qualified name so their convenience methods are visible
            // on the accessor without caller imports.
            val className = HAND_WRITTEN_SERVICES[name] ?: service.className
            sb.appendLine("/** ${service.name} operations. */")
            sb.appendLine("val AccountClient.${propertyName}: ${className}")
            sb.appendLine("    get() = service(\"${name}\") { ${className}(this) }")
            sb.appendLine()
        }

        return sb.toString()
    }
}
