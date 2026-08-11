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

repositories {
    mavenCentral()
}

dependencies {
    implementation("software.amazon.smithy:smithy-openapi:1.72.1")
    testImplementation("org.junit.jupiter:junit-jupiter:6.1.2")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher:6.1.2")
}

tasks.test {
    useJUnitPlatform()
}

publishing {
    publications {
        create<MavenPublication>("maven") {
            from(components["java"])
        }
    }
}
