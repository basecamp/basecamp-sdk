// swift-tools-version: 6.0
// Distribution manifest — exposes the Basecamp library for SPM consumers.
// For full development (generator, tests), use swift/Package.swift.
import PackageDescription

let package = Package(
    name: "Basecamp",
    platforms: [.iOS(.v16), .macOS(.v12)],
    products: [
        .library(name: "Basecamp", targets: ["Basecamp"]),
    ],
    targets: [
        .target(
            name: "Basecamp",
            path: "swift/Sources/Basecamp",
            swiftSettings: [.swiftLanguageMode(.v6)]
        ),
    ]
)
