import SwiftUI

struct DashboardView: View {
    private let rows: [DashboardRow] = [
        DashboardRow(title: "Builds", systemImage: "hammer.fill", tint: .violet),
        DashboardRow(title: "Crashes", systemImage: "exclamationmark.triangle.fill", tint: .coral),
        DashboardRow(title: "Analytics", systemImage: "chart.line.uptrend.xyaxis", tint: .magenta),
    ]

    var body: some View {
        List(rows) { row in
            Label {
                Text(row.title)
                    .font(.system(size: 17, weight: .medium))
            } icon: {
                Image(systemName: row.systemImage)
                    .foregroundStyle(row.tint)
            }
            .padding(.vertical, 4)
        }
        .listStyle(.insetGrouped)
        .navigationTitle("Dashboard")
        .navigationBarTitleDisplayMode(.large)
    }
}

private struct DashboardRow: Identifiable {
    let id = UUID()
    let title: String
    let systemImage: String
    let tint: Color
}

#Preview {
    NavigationStack {
        DashboardView()
    }
}
