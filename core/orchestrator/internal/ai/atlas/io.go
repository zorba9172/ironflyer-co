// io.go holds the small IO helpers used by the Atlas indexer. Keeping
// them out of indexer.go makes the AST + regex extractor logic easier
// to read.

package atlas

import (
	"errors"
	"io"
	"os"
)

// errFileTooLarge is the sentinel readFileLimited uses when a source
// file exceeds the per-file cap. The indexer skips capability emission
// for such files — they're almost always generated code or build
// artifacts that slipped past the SkipDirs prune.
var errFileTooLarge = errors.New("atlas: file exceeds extractor size cap")

// readFileLimited reads up to limit bytes from path. Returning early
// when the file is huge avoids OOM if a stray binary lands under a
// monitored directory.
func readFileLimited(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() > limit {
		return nil, errFileTooLarge
	}
	return io.ReadAll(io.LimitReader(f, limit))
}
