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

// gradle-wrapper.properties is generator output, and `retries` is the one line
// in it we do not take from the generator's default. That default is 0, which
// makes the wrapper's own download loop a single attempt: one transient
// services.gradle.org fault ("Attempt 1/1 failed. Reason: ... 503") fails the
// job outright (#729). Configured here so `./gradlew wrapper` re-emits 3
// instead of silently resetting it — dependabot's gradle-wrapper bumps rewrite
// only distributionUrl, so they preserve it either way, but a hand-run
// regeneration would not. Keep in sync with the sibling build in
// spec/smithy-bare-arrays/, whose wrapper carries the identical settings.
//
// This retries the DISTRIBUTION DOWNLOAD, not the build: it cannot mask a
// flaky test or a real compile error.
tasks.wrapper {
    retries.set(3)
    retryBackOffMs.set(500)
}
