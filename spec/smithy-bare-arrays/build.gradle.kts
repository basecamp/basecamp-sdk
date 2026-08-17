plugins {
    java
    `maven-publish`
}

group = "com.basecamp"
version = "1.0.0"

java {
    // 17 to match the fleet toolchain (.mise.toml and every CI workflow pin
    // temurin-17); this jar is only consumed in-repo by `smithy build`. The
    // previous 11 was boilerplate from the project's first commit, and it made
    // JUnit 6.x (JVM-17-only) unresolvable — which blocked dependabot's test
    // dependency updates.
    sourceCompatibility = JavaVersion.VERSION_17
    targetCompatibility = JavaVersion.VERSION_17
}

// javac takes its source encoding from the platform default charset, which is
// US-ASCII wherever the ambient locale is not UTF-8 — a non-interactive ssh
// shell, cron, a locale-less container image. These sources carry em-dashes in
// their prose (#669), and the current toolchain does not stop when it cannot
// read them: Gradle 9.7 prints `unmappable character (0xE2) for encoding
// US-ASCII`, substitutes U+FFFD and reports BUILD SUCCESSFUL, so a non-ASCII
// string literal would ship silently mangled rather than fail loudly. Plain
// `javac` on the same input exits 1. Pin the encoding so neither can happen.
tasks.withType<JavaCompile>().configureEach {
    options.encoding = "UTF-8"
}

repositories {
    mavenCentral()
}

dependencies {
    implementation("software.amazon.smithy:smithy-openapi:1.73.0")
    testImplementation("org.junit.jupiter:junit-jupiter:6.1.3")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher:6.1.3")
}

tasks.test {
    useJUnitPlatform()
}

// See kotlin/build.gradle.kts for the full rationale: the Wrapper task defaults
// `retries` to 0, so one transient services.gradle.org fault fails the job
// (#729). Configured here too so `./gradlew wrapper` re-emits it, and so this
// wrapper cannot drift away from the Kotlin SDK's.
tasks.wrapper {
    retries.set(3)
    retryBackOffMs.set(500)
}

publishing {
    publications {
        create<MavenPublication>("maven") {
            from(components["java"])
        }
    }
}
