import SwiftUI

// Ironflyer brand tokens. Values mirror the design system used across the
// web surfaces. Keep these in sync with packages/design-tokens.
extension Color {
    /// Primary brand violet (#8F4DFF).
    static let violet = Color(red: 0.561, green: 0.302, blue: 1.0)

    /// CTA gradient anchor — coral (#FF4D4D).
    static let coral = Color(red: 1.0, green: 0.302, blue: 0.302)

    /// CTA gradient anchor — magenta (#FF4DD1).
    static let magenta = Color(red: 1.0, green: 0.302, blue: 0.820)

    /// Chrome / surface base — near-black (#050507).
    static let nearBlack = Color(red: 0.020, green: 0.020, blue: 0.027)
}
