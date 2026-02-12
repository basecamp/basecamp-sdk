rootProject.name = "basecamp-sdk-kotlin"

dependencyResolutionManagement {
    repositories {
        mavenCentral()
    }
}

include(":sdk")
include(":generator")
include(":conformance")
