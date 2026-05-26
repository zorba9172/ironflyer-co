package appsec

import "strings"

type Inventory struct {
	FileCount        int
	HasGoMod         bool
	HasPackageLock   bool
	HasPackageJSON   bool
	HasDockerfile    bool
	HasCompose       bool
	HasGitHubAction  bool
	HasPulumi        bool
	HasEnvFile       bool
	RuntimeEnabled   bool
	GoModPaths       []string
	PackageLockPaths []string
	Components       []Component
	Services         []ServiceAsset
}

type Component struct {
	Ecosystem  string
	Name       string
	Version    string
	Path       string
	Dev        bool
	Deprecated string
}

type ServiceAsset struct {
	ID       string
	Path     string
	Language string
	Runtime  string
}

func BuildInventory(target Target) Inventory {
	inv := Inventory{FileCount: len(target.Files), RuntimeEnabled: target.HasRuntime()}
	serviceSeen := map[string]bool{}
	for _, f := range target.Files {
		path := cleanPath(f.Path)
		low := strings.ToLower(path)
		switch {
		case strings.HasSuffix(low, "go.mod"):
			inv.HasGoMod = true
			inv.GoModPaths = append(inv.GoModPaths, path)
			inv.Components = append(inv.Components, parseGoModComponents(path, f.Content)...)
			addService(&inv, serviceSeen, serviceRoot(path), "go", "")
		case strings.HasSuffix(low, "package-lock.json"):
			inv.HasPackageLock = true
			inv.PackageLockPaths = append(inv.PackageLockPaths, path)
			inv.Components = append(inv.Components, parsePackageLockComponents(path, f.Content)...)
			addService(&inv, serviceSeen, serviceRoot(path), "node", "")
		case strings.HasSuffix(low, "package.json"):
			inv.HasPackageJSON = true
			addService(&inv, serviceSeen, serviceRoot(path), "node", "")
		case strings.HasSuffix(low, "dockerfile") || strings.Contains(low, ".dockerfile"):
			inv.HasDockerfile = true
		case strings.HasSuffix(low, "docker-compose.yml") || strings.HasSuffix(low, "docker-compose.yaml") || strings.Contains(low, "/compose/"):
			inv.HasCompose = true
		case strings.HasPrefix(low, ".github/workflows/") && (strings.HasSuffix(low, ".yml") || strings.HasSuffix(low, ".yaml")):
			inv.HasGitHubAction = true
		case strings.Contains(low, "pulumi.") || strings.HasSuffix(low, "pulumi.yaml"):
			inv.HasPulumi = true
		}
		if isEnvPath(low) {
			inv.HasEnvFile = true
		}
	}
	return inv
}

func addService(inv *Inventory, seen map[string]bool, root, language, runtimeName string) {
	if root == "" {
		root = "."
	}
	id := language + ":" + root
	if seen[id] {
		return
	}
	seen[id] = true
	inv.Services = append(inv.Services, ServiceAsset{
		ID:       id,
		Path:     root,
		Language: language,
		Runtime:  runtimeName,
	})
}

func serviceRoot(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "."
	}
	return path[:idx]
}

func isEnvPath(low string) bool {
	base := low
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	return strings.HasPrefix(base, ".env") || strings.HasSuffix(base, ".env")
}

func cleanPath(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	path = strings.TrimPrefix(path, "./")
	return path
}
