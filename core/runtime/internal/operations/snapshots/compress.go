package snapshots

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// tarZstdDir streams (tar -> zstd) the contents of srcDir into w.
// Returns the total uncompressed byte count and any error. Entries
// inside any directory whose base name appears in excludes are
// skipped wholesale (whole subtree pruned).
func tarZstdDir(srcDir string, w io.Writer, excludes []string) (int64, error) {
	enc, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return 0, err
	}
	defer enc.Close()
	tw := tar.NewWriter(enc)
	defer tw.Close()

	exSet := make(map[string]struct{}, len(excludes))
	for _, e := range excludes {
		if e != "" {
			exSet[e] = struct{}{}
		}
	}

	var total int64
	walkErr := filepath.Walk(srcDir, func(path string, fi os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		rel, rerr := filepath.Rel(srcDir, path)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return nil
		}
		// Exclude any path that has an excluded directory anywhere in
		// its rel path — keeps the rule "node_modules anywhere" cheap.
		for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
			if _, hit := exSet[part]; hit {
				if fi.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		hdr, herr := tar.FileInfoHeader(fi, "")
		if herr != nil {
			return herr
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		f, oerr := os.Open(path)
		if oerr != nil {
			return oerr
		}
		n, cerr := io.Copy(tw, f)
		_ = f.Close()
		total += n
		return cerr
	})
	if walkErr != nil {
		return total, walkErr
	}
	if err := tw.Close(); err != nil {
		return total, err
	}
	if err := enc.Close(); err != nil {
		return total, err
	}
	return total, nil
}

// untarZstd extracts a zstd-compressed tar stream into destDir.
// Defends against tar-slip; rejects non-regular non-directory entries
// (symlinks, devices) for safety.
func untarZstd(r io.Reader, destDir string) error {
	zr, err := zstd.NewReader(r)
	if err != nil {
		return err
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	clean := filepath.Clean(destDir)
	for {
		hdr, terr := tr.Next()
		if errors.Is(terr, io.EOF) {
			return nil
		}
		if terr != nil {
			return terr
		}
		target := filepath.Join(destDir, hdr.Name)
		if !strings.HasPrefix(target, clean+string(os.PathSeparator)) && target != clean {
			return fmt.Errorf("%w: %s", ErrPathEscape, hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			_ = f.Close()
		default:
			// Skip symlinks/devices for safety.
		}
	}
}

// sha256File returns the hex SHA-256 of the contents of path. Used to
// stamp Metadata.Checksum after upload so a downstream re-pull can
// verify integrity.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
