// Package finisher — schema-evolution helper. When the Coder evolves
// the data model, we ask the Migrator agent for a reversible
// migration patch instead of letting the project drift. The helper
// here builds the agent task + parses the structured output; the
// call site (loop.go) is intentionally left for a follow-up so we
// don't conflict with other in-flight changes.

package finisher

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/patch"
)

// MigrationTool names the database-migration toolchain the Migrator
// selected for this project.
type MigrationTool string

const (
	MigrationDrizzle   MigrationTool = "drizzle"
	MigrationPrisma    MigrationTool = "prisma"
	MigrationAlembic   MigrationTool = "alembic"
	MigrationRailsAR   MigrationTool = "rails-active-record"
	MigrationSequelize MigrationTool = "sequelize"
	MigrationFlyway    MigrationTool = "flyway"
	MigrationEFCore    MigrationTool = "ef-core"
)

// MigrationPlan is the parsed Migrator-agent reply. Files carries the
// new migration sources the helper has pre-classified as OpCreate
// patch entries so the caller can hand them straight to the patch
// engine.
type MigrationPlan struct {
	Tool    MigrationTool      `json:"tool"`
	Name    string             `json:"name"`
	Summary string             `json:"summary"`
	Files   []patch.FileChange `json:"files"` // ops are all OpCreate
}

// migratorReply mirrors the agent's wire-format. We re-emit Files as
// patch.FileChange because the agent only sends path + content; the Op
// is fixed (every migration entry is a fresh file) so we set it here.
type migratorReply struct {
	Tool    MigrationTool `json:"tool"`
	Name    string        `json:"name"`
	Summary string        `json:"summary"`
	Files   []struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	} `json:"files"`
}

// RunMigrator dispatches the Migrator agent and returns a parsed
// MigrationPlan. The caller decides whether to convert the files
// into a patch + push through Patches.Propose (recommended) or to
// apply directly (not recommended — bypasses gates).
func (e *Engine) RunMigrator(ctx context.Context, projectID string, desiredModel []domain.EntityDef) (MigrationPlan, error) {
	proj, err := e.projects.Get(projectID)
	if err != nil {
		return MigrationPlan{}, err
	}

	task := agents.Task{
		Role:        agents.RoleMigrator,
		Project:     &proj,
		Goal:        buildMigratorGoal(&proj, desiredModel),
		UserBearer:  bearerFromCtx(ctx),
		WorkspaceID: workspaceIDFromCtx(ctx),
	}

	res, err := e.registry.Run(ctx, task)
	if err != nil {
		return MigrationPlan{}, err
	}

	var raw migratorReply
	if err := unmarshalJSONFromText(res.Output, &raw); err != nil {
		return MigrationPlan{}, err
	}

	changes := make([]patch.FileChange, 0, len(raw.Files))
	nonEmpty := 0
	for _, f := range raw.Files {
		if strings.TrimSpace(f.Path) == "" {
			continue
		}
		if strings.TrimSpace(f.Content) != "" {
			nonEmpty++
		}
		changes = append(changes, patch.FileChange{
			Op:      patch.OpCreate,
			Path:    f.Path,
			Content: f.Content,
		})
	}
	if nonEmpty == 0 {
		return MigrationPlan{}, errors.New("migrator: empty plan")
	}

	return MigrationPlan{
		Tool:    raw.Tool,
		Name:    raw.Name,
		Summary: raw.Summary,
		Files:   changes,
	}, nil
}

// ToPatch converts the plan into a patch.Patch ready for the Engine's
// Patches.Propose flow so the migration goes through the same gates as
// any other Coder output.
func (p MigrationPlan) ToPatch(projectID string) patch.Patch {
	return patch.Patch{
		ProjectID: projectID,
		Title:     "migration: " + p.Name,
		Summary:   p.Summary,
		Author:    "migrator",
		Changes:   p.Files,
	}
}

// buildMigratorGoal renders the prompt body the Migrator agent reads:
// current data model on disk, desired data model the Coder is moving
// toward, and an evidence block listing the dependency manifests the
// agent should use to pick the right toolchain.
func buildMigratorGoal(p *domain.Project, desired []domain.EntityDef) string {
	var b strings.Builder
	b.WriteString("Emit a reversible migration patch that evolves the project's database schema from the CURRENT data model to the DESIRED data model.\n\n")

	b.WriteString("## Current data model\n")
	if cur := renderDataModelJSON(p.Spec.DataModel); cur != "" {
		b.WriteString(cur)
	} else {
		b.WriteString("(empty — first migration)\n")
	}
	b.WriteString("\n")

	b.WriteString("## Desired data model\n")
	if des := renderDataModelJSON(desired); des != "" {
		b.WriteString(des)
	} else {
		b.WriteString("(empty)\n")
	}
	b.WriteString("\n")

	b.WriteString("## Dependency manifests (evidence for tool selection)\n")
	manifests := collectDependencyManifests(p)
	if len(manifests) == 0 {
		b.WriteString("(no manifests on disk — pick a toolchain that matches the declared stack)\n")
	} else {
		for _, m := range manifests {
			b.WriteString("### " + m.Path + "\n")
			b.WriteString("```\n")
			b.WriteString(m.Content)
			if !strings.HasSuffix(m.Content, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("```\n")
		}
	}
	return b.String()
}

// renderDataModelJSON serialises a data model slice as compact JSON
// for the prompt; we use JSON rather than free text so the agent reads
// the structure unambiguously.
func renderDataModelJSON(model []domain.EntityDef) string {
	if len(model) == 0 {
		return ""
	}
	buf, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return ""
	}
	return string(buf) + "\n"
}

// manifestPaths is the set of files the Migrator inspects to decide
// which migration toolchain is already wired into the project. The
// list is conservative — adding a new ecosystem only requires another
// entry here.
var manifestPaths = []string{
	"package.json",
	"pnpm-lock.yaml",
	"yarn.lock",
	"package-lock.json",
	"requirements.txt",
	"pyproject.toml",
	"Pipfile",
	"poetry.lock",
	"Gemfile",
	"Gemfile.lock",
	"go.mod",
	"go.sum",
	"composer.json",
	"build.gradle",
	"build.gradle.kts",
	"pom.xml",
	"Cargo.toml",
	"*.csproj",
	"flyway.conf",
	"drizzle.config.ts",
	"drizzle.config.js",
	"prisma/schema.prisma",
	"alembic.ini",
}

type manifestFile struct {
	Path    string
	Content string
}

// collectDependencyManifests scans the project's in-memory file tree
// for known manifest filenames and returns the ones present. The
// matcher honours a single trailing-wildcard pattern (e.g. *.csproj).
func collectDependencyManifests(p *domain.Project) []manifestFile {
	if p == nil || len(p.Files) == 0 {
		return nil
	}
	out := make([]manifestFile, 0, 4)
	for i := range p.Files {
		f := p.Files[i]
		if !isManifestPath(f.Path) {
			continue
		}
		out = append(out, manifestFile{Path: f.Path, Content: f.Content})
	}
	return out
}

func isManifestPath(path string) bool {
	base := path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	for _, pat := range manifestPaths {
		if strings.HasPrefix(pat, "*.") {
			if strings.HasSuffix(base, pat[1:]) {
				return true
			}
			continue
		}
		// Allow either a plain basename match or a full-path match
		// (so prisma/schema.prisma resolves correctly even when the
		// file lives under a deeper directory).
		if base == pat || path == pat || strings.HasSuffix(path, "/"+pat) {
			return true
		}
	}
	return false
}
