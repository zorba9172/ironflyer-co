package deploy

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	gh "github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

// GitHubExporter creates a fresh repo under the user's account and pushes
// the project's files into it. It reuses the user's OAuth access token
// (stored by the existing github.Service integration); the orchestrator
// never holds long-lived push credentials of its own.
type GitHubExporter struct{}

// NewGitHubExporter returns a stateless exporter. The user's token is
// passed per call so we can serve multiple users without leaking state.
func NewGitHubExporter() *GitHubExporter { return &GitHubExporter{} }

// ExportRequest carries the desired repo name and the files to push.
type ExportRequest struct {
	Token       string       // GitHub OAuth access token (repo scope)
	Owner       string       // empty → authenticated user
	RepoName    string       // required
	Description string
	Private     bool
	Files       []SourceFile // Content takes precedence over OnDisk
	CommitMsg   string       // defaults to "Initial commit from Ironflyer"
	Branch      string       // defaults to "main"
}

// ExportResult is the URL the caller redirects the browser to.
type ExportResult struct {
	RepoURL  string `json:"repoUrl"`
	CloneURL string `json:"cloneUrl"`
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	Branch   string `json:"branch"`
}

// Export creates the repo and pushes every file in a single commit via the
// Git Data API. We deliberately avoid streaming individual blobs over HTTP
// (1 request per file) — for a typical Ironflyer project this is 3 calls:
// create-tree, create-commit, update-ref. Files larger than ~1MB are still
// supported because the tree API accepts a base64 blob inline.
func (e *GitHubExporter) Export(ctx context.Context, req ExportRequest) (ExportResult, error) {
	if strings.TrimSpace(req.Token) == "" {
		return ExportResult{}, errors.New("github token required")
	}
	if strings.TrimSpace(req.RepoName) == "" {
		return ExportResult{}, errors.New("repoName required")
	}
	if req.CommitMsg == "" {
		req.CommitMsg = "Initial commit from Ironflyer"
	}
	if req.Branch == "" {
		req.Branch = "main"
	}
	client := gh.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: req.Token})))

	// 1. Create the repo. auto_init=true gives us a base commit we can amend.
	newRepo := &gh.Repository{
		Name:        gh.String(req.RepoName),
		Description: gh.String(req.Description),
		Private:     gh.Bool(req.Private),
		AutoInit:    gh.Bool(true),
	}
	created, _, err := client.Repositories.Create(ctx, req.Owner, newRepo)
	if err != nil {
		return ExportResult{}, fmt.Errorf("create repo: %w", err)
	}
	owner := created.GetOwner().GetLogin()
	name := created.GetName()

	// Brief settle — auto_init is async on GitHub's side and the very next
	// call sometimes 404s the empty ref.
	time.Sleep(900 * time.Millisecond)

	// 2. Look up the base commit so we can build a tree on top of it.
	branch := req.Branch
	ref, _, err := client.Git.GetRef(ctx, owner, name, "refs/heads/"+branch)
	if err != nil {
		// auto_init may have used `main` even if the user asked for something
		// else — retry once on `main`.
		if branch != "main" {
			ref, _, err = client.Git.GetRef(ctx, owner, name, "refs/heads/main")
			branch = "main"
		}
		if err != nil {
			return ExportResult{}, fmt.Errorf("get ref: %w", err)
		}
	}
	baseCommit, _, err := client.Git.GetCommit(ctx, owner, name, ref.GetObject().GetSHA())
	if err != nil {
		return ExportResult{}, fmt.Errorf("get base commit: %w", err)
	}

	// 3. Build the new tree. We send every file as a base64 blob so binary
	// files survive transit. The tree is rooted on the existing base tree
	// so README.md from auto_init is preserved unless we overwrite it.
	entries := make([]*gh.TreeEntry, 0, len(req.Files))
	for _, f := range req.Files {
		path := strings.TrimPrefix(strings.TrimPrefix(f.Path, "/"), "./")
		if path == "" {
			continue
		}
		content := f.Content
		// Encode as base64 even for text so multi-line / unicode survives.
		blobSHA, err := createBlob(ctx, client, owner, name, content)
		if err != nil {
			return ExportResult{}, fmt.Errorf("create blob %s: %w", path, err)
		}
		entries = append(entries, &gh.TreeEntry{
			Path: gh.String(path),
			Mode: gh.String("100644"),
			Type: gh.String("blob"),
			SHA:  gh.String(blobSHA),
		})
	}
	tree, _, err := client.Git.CreateTree(ctx, owner, name, baseCommit.GetTree().GetSHA(), entries)
	if err != nil {
		return ExportResult{}, fmt.Errorf("create tree: %w", err)
	}

	// 4. Commit the tree on top of the base commit, then move the ref.
	commit, _, err := client.Git.CreateCommit(ctx, owner, name, &gh.Commit{
		Message: gh.String(req.CommitMsg),
		Tree:    tree,
		Parents: []*gh.Commit{{SHA: baseCommit.SHA}},
	}, nil)
	if err != nil {
		return ExportResult{}, fmt.Errorf("create commit: %w", err)
	}
	ref.Object.SHA = commit.SHA
	if _, _, err := client.Git.UpdateRef(ctx, owner, name, ref, false); err != nil {
		return ExportResult{}, fmt.Errorf("update ref: %w", err)
	}

	return ExportResult{
		RepoURL:  created.GetHTMLURL(),
		CloneURL: created.GetCloneURL(),
		Owner:    owner,
		Name:     name,
		Branch:   branch,
	}, nil
}

// createBlob uploads a single file body and returns its SHA. We always use
// base64 so binary safety isn't a special case.
func createBlob(ctx context.Context, client *gh.Client, owner, repo, content string) (string, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	blob, _, err := client.Git.CreateBlob(ctx, owner, repo, &gh.Blob{
		Content:  gh.String(encoded),
		Encoding: gh.String("base64"),
	})
	if err != nil {
		return "", err
	}
	return blob.GetSHA(), nil
}
