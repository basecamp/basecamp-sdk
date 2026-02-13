rootProject.name = "basecamp-sdk-kotlin"

dependencyResolutionManagement {
    repositories {
        mavenCentral()
    }
}

include(":basecamp-sdk")
project(":basecamp-sdk").projectDir = file("sdk")
include(":generator")
include(":conformance")
