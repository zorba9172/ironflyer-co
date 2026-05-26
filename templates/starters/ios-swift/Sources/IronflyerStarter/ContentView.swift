import SwiftUI

struct ContentView: View {
    var body: some View {
        NavigationStack {
            ZStack {
                Color.nearBlack
                    .ignoresSafeArea()

                VStack(spacing: 24) {
                    Spacer()

                    VStack(spacing: 12) {
                        Text("Ironflyer Starter")
                            .font(.system(size: 40, weight: .bold, design: .rounded))
                            .foregroundStyle(Color.white)
                            .multilineTextAlignment(.center)

                        Text("Ships gated mobile builds.")
                            .font(.system(size: 17, weight: .regular))
                            .foregroundStyle(Color.white.opacity(0.72))
                            .multilineTextAlignment(.center)
                    }
                    .padding(.horizontal, 32)

                    Spacer()

                    NavigationLink(value: Destination.dashboard) {
                        Text("Open dashboard")
                            .font(.system(size: 17, weight: .semibold))
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 6)
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(Color.violet)
                    .controlSize(.large)
                    .padding(.horizontal, 24)
                    .padding(.bottom, 32)
                }
            }
            .navigationDestination(for: Destination.self) { destination in
                switch destination {
                case .dashboard:
                    DashboardView()
                }
            }
        }
        .tint(Color.violet)
    }
}

enum Destination: Hashable {
    case dashboard
}

#Preview {
    ContentView()
}
