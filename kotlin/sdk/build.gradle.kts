plugins {
    alias(libs.plugins.kotlin.multiplatform)
    alias(libs.plugins.kotlin.serialization)
    `maven-publish`
}

group = "com.basecamp"
version = "0.15.0"

kotlin {
    jvm {
        // Enforce the zero-generator-origin-deprecation-warning invariant (#406):
        // the generated common+jvm main sources compile with all warnings
        // (including DEPRECATION) promoted to errors. The generic
        // ModelEmitter/ServiceEmitter @Suppress("DEPRECATION") annotations must
        // hold for this to pass; a regression (e.g. a new deprecated-type
        // reference without suppression) then fails the build.
        compilations.named("main") {
            compileTaskProvider.configure {
                compilerOptions {
                    allWarningsAsErrors.set(true)
                }
            }
        }
    }

    sourceSets {
        commonMain.dependencies {
            api(libs.ktor.client.core)
            implementation(libs.ktor.client.content.negotiation)
            implementation(libs.ktor.serialization.kotlinx.json)
            api(libs.kotlinx.serialization.json)
            implementation(libs.kotlinx.coroutines.core)
        }
        jvmMain.dependencies {
            implementation(libs.ktor.client.cio)
        }
        commonTest.dependencies {
            implementation(kotlin("test"))
            implementation(libs.ktor.client.mock)
            implementation(libs.kotlinx.coroutines.test)
        }
        jvmTest.dependencies {
            implementation(libs.junit.jupiter)
            // Drives the deprecation diagnostic fixture (#406): compiles source
            // snippets against the SDK classpath at test time and asserts the
            // compiler's diagnostics, rather than just "it compiles".
            implementation("org.jetbrains.kotlin:kotlin-compiler-embeddable:${libs.versions.kotlin.get()}")
        }
    }
}

tasks.withType<Test> {
    useJUnitPlatform()
}

publishing {
    repositories {
        maven {
            name = "GitHubPackages"
            url = uri("https://maven.pkg.github.com/basecamp/basecamp-sdk")
            credentials {
                username = System.getenv("GITHUB_USER") ?: "x-access-token"
                password = System.getenv("GITHUB_ACCESS_TOKEN") ?: ""
            }
        }
    }
}
