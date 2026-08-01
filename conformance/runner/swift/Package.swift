// swift-tools-version: 6.0
import PackageDescription

// Development-only conformance runner. Depends on the Swift SDK via the ROOT
// distribution manifest (../../../Package.swift), whose Basecamp target builds
// the same swift/Sources/Basecamp sources as the development manifest.
//
// Why not swift/Package.swift directly: SwiftPM derives package identity from
// the directory name, so a path dependency on ".../swift" collides with this
// package's own "conformance/runner/swift" identity and is silently treated
// as a self-reference ("product 'Basecamp' ... not found"). The root manifest
// avoids the collision without renaming either directory.
let package = Package(
    name: "ConformanceRunner",
    platforms: [
        .macOS(.v12)
    ],
    dependencies: [
        .package(name: "Basecamp", path: "../../..")
    ],
    targets: [
        .executableTarget(
            name: "ConformanceRunner",
            dependencies: [
                "Basecamp"
            ],
            path: "Sources/ConformanceRunner",
            swiftSettings: [
                .swiftLanguageMode(.v6)
            ]
        )
    ]
)
