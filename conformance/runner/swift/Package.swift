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
        // Assertion contracts with no SDK dependency, split out of the
        // executable so their bounds branches can be unit-tested: a target
        // carrying @main cannot host XCTest cleanly, and these branches never
        // execute against a fixture that passes. #563 shipped a
        // delayBetweenRequests check that vacuously passed when the gap it
        // named did not exist, in four runners at once.
        .target(
            name: "ConformanceSupport",
            path: "Sources/ConformanceSupport",
            swiftSettings: [
                .swiftLanguageMode(.v6)
            ]
        ),
        .executableTarget(
            name: "ConformanceRunner",
            dependencies: [
                "Basecamp",
                "ConformanceSupport"
            ],
            path: "Sources/ConformanceRunner",
            swiftSettings: [
                .swiftLanguageMode(.v6)
            ]
        ),
        .testTarget(
            name: "ConformanceSupportTests",
            dependencies: [
                "ConformanceSupport"
            ],
            path: "Tests/ConformanceSupportTests",
            swiftSettings: [
                .swiftLanguageMode(.v6)
            ]
        )
    ]
)
