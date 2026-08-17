// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "Basecamp",
    platforms: [
        .iOS(.v16),
        .macOS(.v12),
    ],
    products: [
        .library(name: "Basecamp", targets: ["Basecamp"]),
    ],
    targets: [
        .target(
            name: "Basecamp",
            path: "Sources/Basecamp",
            swiftSettings: [
                .swiftLanguageMode(.v6),
            ]
        ),
        // Models a customer: depends on the `Basecamp` target and imports it
        // plainly, so only the public surface is visible (#735). Deliberately a
        // non-test target — `swift build` compiles it, which is broader cover
        // than `swift test`, and it is deliberately absent from `products` so it
        // is never built by a package that depends on this one. Its sources must
        // never use `@testable`; PublicInitCoverageTests enforces both properties.
        .target(
            name: "BasecampPublicAPIConsumer",
            dependencies: ["Basecamp"],
            path: "Sources/BasecampPublicAPIConsumer",
            swiftSettings: [
                .swiftLanguageMode(.v6),
            ]
        ),
        .executableTarget(
            name: "BasecampGenerator",
            path: "Sources/BasecampGenerator",
            swiftSettings: [
                .swiftLanguageMode(.v6),
            ]
        ),
        .testTarget(
            name: "BasecampTests",
            dependencies: ["Basecamp", "BasecampGenerator"],
            path: "Tests/BasecampTests",
            resources: [
                .copy("Fixtures"),
            ],
            swiftSettings: [
                .swiftLanguageMode(.v6),
            ]
        ),
    ]
)
