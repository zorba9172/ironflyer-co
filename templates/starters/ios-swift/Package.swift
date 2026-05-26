// swift-tools-version: 5.10
// Ironflyer iOS starter — SPM manifest used by SourceKit-LSP / VSCode Swift
// extension for editor support. The actual buildable iOS app target is
// produced by `make generate` (xcodegen) and built with `xcodebuild`.

import PackageDescription

let package = Package(
    name: "IronflyerStarter",
    platforms: [
        .iOS(.v15),
    ],
    products: [
        .library(
            name: "IronflyerStarter",
            targets: ["IronflyerStarter"]
        ),
    ],
    targets: [
        .target(
            name: "IronflyerStarter",
            path: "Sources/IronflyerStarter"
        ),
    ]
)
