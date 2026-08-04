plugins {
    java
    `maven-publish`
}

group = "com.basecamp"
version = "1.0.0"

java {
    sourceCompatibility = JavaVersion.VERSION_11
    targetCompatibility = JavaVersion.VERSION_11
}

repositories {
    mavenCentral()
}

dependencies {
    implementation("software.amazon.smithy:smithy-openapi:1.72.1")
    // JUnit 5.x, not 6.x: 6.x publishes only a JVM-17 variant, and this project
    // targets 11, so Gradle refuses to resolve it and `./gradlew test` fails at
    // compileTestJava — which is how three mapper test classes sat unrunnable.
    testImplementation("org.junit.jupiter:junit-jupiter:5.11.4")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher:1.11.4")
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
