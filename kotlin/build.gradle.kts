plugins {
    alias(libs.plugins.kotlin.multiplatform) apply false
    alias(libs.plugins.kotlin.jvm) apply false
    alias(libs.plugins.kotlin.serialization) apply false
}

// Pin javac's source encoding for every module (#669). Kotlin itself is not
// exposed — the language spec fixes source files at UTF-8 and kotlinc has no
// encoding flag — but the kotlin-jvm plugin applies the `java` plugin, so each
// module carries JavaCompile tasks that default to the platform charset and
// would read UTF-8 prose as US-ASCII under a C locale. No module has Java
// sources today, so this is prophylactic here: it is the first `.java` file (or
// a KMP `withJava()`) that would otherwise reintroduce the bug silently, which
// is exactly the moment nobody would think to look. Configured once at the root
// rather than four times so a new module inherits it.
subprojects {
    tasks.withType<JavaCompile>().configureEach {
        options.encoding = "UTF-8"
    }
}
