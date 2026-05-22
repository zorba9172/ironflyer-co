// Package deploy holds the provider clients the orchestrator uses to push a
// finished project to a public URL (Fly, Railway), or export it (zip,
// GitHub). Each client is a thin wrapper that prefers HTTP APIs but is
// allowed to shell out to vendor CLIs when the API surface is impractical.
package deploy

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SourceFile is a single file destined for the export. Content takes
// precedence; if Content is empty and OnDisk is set, the file is streamed
// from disk. This dual shape lets callers feed both in-memory project
// snapshots (domain.Project.Files) and on-disk workspace trees.
type SourceFile struct {
	Path    string // relative path inside the archive (no leading slash)
	Content string // when non-empty, written verbatim
	OnDisk  string // absolute fs path used when Content is empty
	Mode    os.FileMode
}

// WriteZip streams a zip archive of the given files to w. It returns the
// number of bytes written and the first error encountered. Symlinks and
// .git directories are skipped to keep archives small and reproducible.
func WriteZip(w io.Writer, files []SourceFile) (int64, error) {
	if w == nil {
		return 0, errors.New("nil writer")
	}
	cw := &countingWriter{w: w}
	zw := zip.NewWriter(cw)
	defer zw.Close()
	for _, f := range files {
		clean := strings.TrimPrefix(filepath.ToSlash(f.Path), "/")
		if clean == "" || strings.Contains(clean, "/.git/") || strings.HasPrefix(clean, ".git/") {
			continue
		}
		mode := f.Mode
		if mode == 0 {
			mode = 0o644
		}
		hdr := &zip.FileHeader{Name: clean, Method: zip.Deflate}
		hdr.SetMode(mode)
		entry, err := zw.CreateHeader(hdr)
		if err != nil {
			return cw.n, err
		}
		if f.Content != "" {
			if _, err := entry.Write([]byte(f.Content)); err != nil {
				return cw.n, err
			}
			continue
		}
		if f.OnDisk == "" {
			continue
		}
		src, err := os.Open(f.OnDisk)
		if err != nil {
			return cw.n, err
		}
		_, err = io.Copy(entry, src)
		_ = src.Close()
		if err != nil {
			return cw.n, err
		}
	}
	if err := zw.Close(); err != nil {
		return cw.n, err
	}
	return cw.n, nil
}

// WalkDirToSources walks a directory tree and returns a SourceFile list
// referencing each regular file by absolute path. .git, node_modules, and
// build artifact dirs are pruned so exported zips don't carry tens of MBs of
// noise.
func WalkDirToSources(root string) ([]SourceFile, error) {
	var out []SourceFile
	skipDirs := map[string]struct{}{
		".git": {}, "node_modules": {}, ".next": {}, "dist": {}, "build": {},
		".turbo": {}, ".cache": {}, "target": {},
	}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if _, skip := skipDirs[info.Name()]; skip && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, SourceFile{
			Path:   filepath.ToSlash(rel),
			OnDisk: path,
			Mode:   info.Mode(),
		})
		return nil
	})
	return out, err
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
