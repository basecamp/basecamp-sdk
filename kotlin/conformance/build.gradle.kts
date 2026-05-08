plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.kotlin.serialization)
    application
}

application {
    mainClass.set("com.basecamp.sdk.conformance.MainKt")
}

tasks.named<JavaExec>("run") {
    workingDir = rootProject.projectDir
}

// Wire-replay runner entry point (PR 3). The mock-mode runner above is
// untouched; this task targets ReplayRunner.kt's `main` and is invoked by
// the Makefile target `conformance-kotlin-replay` when WIRE_REPLAY_DIR is
// set. Reads canonical wire snapshots written by the TS live runner and
// writes per-test decode-result snapshots.
tasks.register<JavaExec>("runReplay") {
    group = "application"
    description = "Run the wire-replay conformance runner (set WIRE_REPLAY_DIR + BASECAMP_BACKEND)."
    classpath = sourceSets["main"].runtimeClasspath
    mainClass.set("com.basecamp.sdk.conformance.ReplayRunnerKt")
    workingDir = rootProject.projectDir
    // JavaExec inherits the parent process environment by default; we don't
    // explicitly set WIRE_REPLAY_DIR/BASECAMP_BACKEND because forcing them in
    // (with `?: ""` fallbacks) would smuggle empty strings into the child env
    // and bypass the runner's "is required" gate.
}

dependencies {
    implementation(project(":basecamp-sdk"))
    implementation(libs.kotlinx.serialization.json)
    implementation(libs.ktor.client.mock)
    implementation(libs.kotlinx.coroutines.core)
}
