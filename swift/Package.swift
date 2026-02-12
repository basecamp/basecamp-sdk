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
        .executableTarget(
            name: "BasecampGenerator",
            path: "Sources/BasecampGenerator",
            swiftSettings: [
                .swiftLanguageMode(.v6),
            ]
        ),
        .testTarget(
            name: "BasecampTests",
            dependencies: ["Basecamp"],
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
