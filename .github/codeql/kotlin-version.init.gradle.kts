// Gradle init script for the CodeQL job's Kotlin build only.
//
// CodeQL's Kotlin extractor refuses a compiler newer than the bundle it ships
// with ("Kotlin version 2.4.20 is too recent. CodeQL currently supports
// versions below 2.4.20"), and dependabot moves the version catalog to each
// Kotlin release days before a CodeQL release supports it. This overrides the
// catalog's `kotlin` version -- which the Kotlin Gradle plugins, and with them
// the compiler and the stdlib the plugin adds, are resolved from -- to the
// newest version CodeQL supports, for this build alone. The repository's own
// build, tests and releases keep the catalog's version.
//
// The catalog is overridden rather than the plugin requests because the root
// project requests the plugins with `apply false` and the modules request them
// again through the same aliases; Gradle insists both requests carry the same
// version, so the version has to change where both read it.
//
// Bump the pin when CodeQL supports a newer Kotlin and the sources need it;
// until then a lower compiler analysing the same sources is the point.
val codeqlKotlinVersion = "2.4.10"

// By the time settings are evaluated Gradle has already imported the
// conventional gradle/libs.versions.toml as `libs`, so the catalog is looked up
// rather than created (a second `from` is an error) and one version replaced.
settingsEvaluated {
    dependencyResolutionManagement.versionCatalogs.maybeCreate("libs")
        .version("kotlin", codeqlKotlinVersion)
}
