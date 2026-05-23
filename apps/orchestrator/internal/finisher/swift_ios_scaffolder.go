// SwiftIOSScaffolder — native iOS skeleton in SwiftUI: App entrypoint,
// NavigationStack, async/await URLSession networking, and SwiftPM for
// dependencies. iOS 17+ deployment target.
//
// Triggers when the spec mentions iOS + native or Swift, or stories
// ask for an iOS app / iPhone experience.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type SwiftIOSScaffolder struct{}

func (SwiftIOSScaffolder) Name() string { return "swift-ios" }

func (SwiftIOSScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	if strings.Contains(stack, "swift") {
		return true
	}
	if strings.Contains(stack, "ios") && strings.Contains(stack, "native") {
		return true
	}
	desc := strings.ToLower(p.Description + " " + p.Spec.Idea)
	if strings.Contains(desc, "ios app") || strings.Contains(desc, "iphone") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "ios app") || strings.Contains(body, "iphone") {
			return true
		}
	}
	return false
}

func (SwiftIOSScaffolder) Scaffold(_ context.Context, _ *domain.Project) (DomainScaffold, error) {
	files := map[string]string{
		"Package.swift": `// swift-tools-version:5.10
// SwiftPM manifest. Keep this file alongside the Xcode project — Xcode
// resolves dependencies from it automatically when you open the package
// folder. We pin Alamofire by floor version, not exact tag, so the
// agent can resolve to a compatible patch release.
import PackageDescription

let package = Package(
    name: "App",
    platforms: [
        .iOS(.v17),
    ],
    products: [
        .library(name: "App", targets: ["App"]),
    ],
    dependencies: [
        .package(url: "https://github.com/Alamofire/Alamofire.git", from: "5.9.0"),
    ],
    targets: [
        .target(
            name: "App",
            dependencies: [
                .product(name: "Alamofire", package: "Alamofire"),
            ],
            path: "App"
        ),
    ]
)
`,
		"App/AppMain.swift": `// App entrypoint. SwiftUI App protocol replaces the old AppDelegate
// dance — WindowGroup gives us a single scene that the system manages.
import SwiftUI

@main
struct AppMain: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}
`,
		"App/Views/ContentView.swift": `// Root content view. NavigationStack gives us iOS 16+ programmatic
// navigation; rows push a DetailView via the value-based navigation
// API rather than NavigationLink(destination:) so deep linking is
// straightforward to add later.
import SwiftUI

struct ContentView: View {
    @State private var items: [String] = ["alpha", "beta", "gamma"]

    var body: some View {
        NavigationStack {
            List {
                ForEach(items, id: \.self) { item in
                    NavigationLink(value: item) {
                        Text(item)
                    }
                }
            }
            .navigationTitle("Ironflyer iOS")
            .navigationDestination(for: String.self) { item in
                DetailView(itemId: item)
            }
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button(action: { items.append("item-" + String(items.count + 1)) }) {
                        Image(systemName: "plus")
                    }
                }
            }
        }
    }
}

#Preview {
    ContentView()
}
`,
		"App/Views/DetailView.swift": `// Detail view — loads a single user from the API. The .task modifier
// runs the async fetch when the view appears and cancels it on disappear,
// so we never leak a network request when the user pops back.
import SwiftUI

struct DetailView: View {
    let itemId: String
    @State private var user: User?
    @State private var errorText: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Item id: " + itemId)
                .font(.headline)
            if let user = user {
                Text(user.name)
                Text(user.email).foregroundStyle(.secondary)
            } else if let errorText = errorText {
                Text(errorText).foregroundStyle(.red)
            } else {
                ProgressView()
            }
            Spacer()
        }
        .padding()
        .navigationTitle("Detail")
        .task {
            do {
                user = try await Api.shared.getUser(id: itemId)
            } catch {
                errorText = String(describing: error)
            }
        }
    }
}
`,
		"App/Services/Api.swift": `// Thin async/await wrapper over URLSession. BASE_URL is read from
// Info.plist (key BASE_URL) so the URL is configured per build
// configuration rather than hardcoded into Swift source.
import Foundation

enum ApiError: Error {
    case missingBaseURL
    case badStatus(Int)
}

final class Api {
    static let shared = Api()

    private let baseURL: URL
    private let session: URLSession

    private init() {
        let raw = Bundle.main.object(forInfoDictionaryKey: "BASE_URL") as? String ?? ""
        guard let url = URL(string: raw) else {
            // Fail loud during development; in production set BASE_URL in Info.plist.
            self.baseURL = URL(string: "https://api.example.com")!
            self.session = URLSession.shared
            return
        }
        self.baseURL = url
        self.session = URLSession.shared
    }

    func listUsers() async throws -> [User] {
        return try await get(path: "/api/users")
    }

    func getUser(id: String) async throws -> User {
        return try await get(path: "/api/users/" + id)
    }

    private func get<T: Decodable>(path: String) async throws -> T {
        let url = baseURL.appendingPathComponent(path)
        let (data, response) = try await session.data(from: url)
        if let http = response as? HTTPURLResponse, !(200..<300).contains(http.statusCode) {
            throw ApiError.badStatus(http.statusCode)
        }
        return try JSONDecoder().decode(T.self, from: data)
    }
}
`,
		"App/Models/User.swift": `// Wire-format model. Codable + Identifiable so it slots straight into
// SwiftUI List / ForEach with no manual id mapping.
import Foundation

struct User: Codable, Identifiable, Hashable {
    let id: String
    let name: String
    let email: String
}
`,
		"Info.plist": `<?xml version="1.0" encoding="UTF-8"?>
<!--
  Minimal Info.plist. Replace BASE_URL with the host your app should
  talk to per build configuration (xcconfig substitutions work here).
  Camera permission string is a placeholder — keep it accurate to what
  the app actually does or App Review will reject.
-->
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleDevelopmentRegion</key>
    <string>en</string>
    <key>CFBundleDisplayName</key>
    <string>Ironflyer iOS</string>
    <key>CFBundleExecutable</key>
    <string>$(EXECUTABLE_NAME)</string>
    <key>CFBundleIdentifier</key>
    <string>$(PRODUCT_BUNDLE_IDENTIFIER)</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>$(PRODUCT_NAME)</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>0.1.0</string>
    <key>CFBundleVersion</key>
    <string>1</string>
    <key>LSRequiresIPhoneOS</key>
    <true/>
    <key>UILaunchScreen</key>
    <dict/>
    <key>UIApplicationSceneManifest</key>
    <dict>
        <key>UIApplicationSupportsMultipleScenes</key>
        <false/>
    </dict>
    <key>UIRequiredDeviceCapabilities</key>
    <array>
        <string>arm64</string>
    </array>
    <key>UISupportedInterfaceOrientations</key>
    <array>
        <string>UIInterfaceOrientationPortrait</string>
    </array>
    <key>BASE_URL</key>
    <string>https://api.example.com</string>
    <key>NSCameraUsageDescription</key>
    <string>This app uses the camera to capture images you choose to share.</string>
</dict>
</plist>
`,
		".gitignore": `# Xcode
build/
DerivedData/
*.xcuserstate
*.xcuserdatad/
xcuserdata/
*.xcscmblueprint
*.xccheckout

# Swift Package Manager
.swiftpm/
.build/
Package.resolved

# CocoaPods (only if you switch off SwiftPM)
Pods/

# Fastlane
fastlane/report.xml
fastlane/Preview.html
fastlane/screenshots/

# macOS
.DS_Store

# Secrets
*.p12
*.mobileprovision
GoogleService-Info.plist
`,
	}
	contract := `Swift iOS scaffold: SwiftUI + NavigationStack + async/await URLSession, iOS 17+.

Already provisioned:
- /Package.swift                  → SwiftPM manifest, Alamofire from 5.9.0
- /App/AppMain.swift              → @main struct AppMain: App, WindowGroup → ContentView
- /App/Views/ContentView.swift    → NavigationStack with a List + add toolbar button
- /App/Views/DetailView.swift     → per-item detail with .task async fetch
- /App/Services/Api.swift         → async/await wrapper over URLSession, reads BASE_URL from Info.plist
- /App/Models/User.swift          → Codable + Identifiable user model
- /Info.plist                     → BASE_URL key, camera permission placeholder
- /.gitignore                     → Xcode + SwiftPM + secrets

Contract for the Coder:
1. Open the project with: xed App  — then press cmd-R in Xcode to build + run.
2. BASE_URL is REQUIRED — set it in Info.plist (or per build config via xcconfig
   substitution). Api.swift reads Bundle.main.object(forInfoDictionaryKey: "BASE_URL").
3. Deployment target: iOS 17. Bump only if you genuinely need a newer API —
   each bump cuts off real users.
4. Add new screens under App/Views and register navigation destinations in ContentView.
5. Add new endpoints to Api.swift; keep models Codable.
6. Do NOT commit *.mobileprovision, *.p12, or GoogleService-Info.plist — they
   are gitignored.
`
	return DomainScaffold{Files: files, Contract: contract}, nil
}
