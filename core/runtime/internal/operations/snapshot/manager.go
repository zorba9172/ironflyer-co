// Package snapshot owns the content layer of a portable workspace: a
// gzip tarball of the workspace root, stored under
//
//	s3://<bucket>/workspaces/<workspaceID>/<unix-ts>.tar.gz
//
// alongside a "LATEST" pointer file containing the timestamp of the
// most recent good snapshot. A workspace boot pulls LATEST, downloads
// that snapshot, and extracts it into a local working directory. A
// stop / periodic checkpoint uploads a fresh tarball, updates LATEST,
// and reaps anything older than the retention window (default 5).
//
// Encryption: every PutObject uses SSE-KMS with the bucket's CMK
// (aws:kms). The Pulumi data agent provisions the bucket + key; the
// runtime just consumes the WORKSPACE_BUCKET env var.
//
// Construction is forgiving: if no bucket is configured, every method
// is a no-op that returns nil. That keeps the dev / mock-driver path
// working unchanged and means call sites never need an `if enabled`
// guard.
package snapshot

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/rs/zerolog"
)

// Config wires the snapshot manager.
type Config struct {
	// Bucket is the S3 bucket name (WORKSPACE_BUCKET env). When empty,
	// the manager is a no-op.
	Bucket string
	// Region is the AWS region (AWS_REGION env). Empty falls back to
	// the SDK's default resolution (instance metadata / shared config).
	Region string
	// Prefix is the in-bucket key prefix. Defaults to "workspaces".
	Prefix string
	// Retention is the number of snapshots to keep per workspace; older
	// objects are deleted after each successful checkpoint. Defaults
	// to 5.
	Retention int
	// KMSKeyID, when set, requests SSE-KMS with the named key. Empty
	// uses the bucket's default SSE configuration (which Pulumi sets to
	// the workspace CMK).
	KMSKeyID string
}

// Manager is the snapshot client. Safe for concurrent use.
type Manager struct {
	cfg      Config
	logger   zerolog.Logger
	s3       *s3.Client
	uploader *manager.Uploader
	dl       *manager.Downloader
}

// New builds a Manager. When cfg.Bucket is empty the returned Manager
// implements every method as a no-op.
func New(ctx context.Context, cfg Config, logger zerolog.Logger) (*Manager, error) {
	if cfg.Prefix == "" {
		cfg.Prefix = "workspaces"
	}
	cfg.Prefix = strings.Trim(cfg.Prefix, "/")
	if cfg.Retention <= 0 {
		cfg.Retention = 5
	}
	m := &Manager{cfg: cfg, logger: logger}
	if cfg.Bucket == "" {
		return m, nil
	}
	opts := []func(*awsconfig.LoadOptions) error{}
	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	m.s3 = s3.NewFromConfig(awsCfg)
	m.uploader = manager.NewUploader(m.s3)
	m.dl = manager.NewDownloader(m.s3)
	return m, nil
}

// Enabled reports whether real S3 is wired.
func (m *Manager) Enabled() bool {
	return m != nil && m.s3 != nil && m.cfg.Bucket != ""
}

// prefixFor returns the workspace-scoped key prefix (no trailing slash).
func (m *Manager) prefixFor(workspaceID string) string {
	return m.cfg.Prefix + "/" + workspaceID
}

// snapshotKey is the per-checkpoint object key.
func (m *Manager) snapshotKey(workspaceID string, ts int64) string {
	return m.prefixFor(workspaceID) + "/" + strconv.FormatInt(ts, 10) + ".tar.gz"
}

// latestKey is the per-workspace pointer object key.
func (m *Manager) latestKey(workspaceID string) string {
	return m.prefixFor(workspaceID) + "/LATEST"
}

// Checkpoint tars + gzips the contents of localDir and uploads them to
// S3. Updates the LATEST pointer on success, then reaps any extra
// snapshots beyond the retention window. When the manager is disabled
// this is a no-op returning nil.
//
// The tarball is built in-memory to keep the snapshot atomic — a
// partial upload (network error mid-stream) is discarded by the SDK's
// multipart abort, and the LATEST pointer is only flipped after the
// PutObject succeeds. That guarantees a reader that follows LATEST
// always sees a fully-uploaded snapshot.
func (m *Manager) Checkpoint(ctx context.Context, workspaceID, localDir string) (string, error) {
	if !m.Enabled() {
		return "", nil
	}
	info, err := os.Stat(localDir)
	if err != nil {
		return "", fmt.Errorf("snapshot: workspace dir %q: %w", localDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("snapshot: %q is not a directory", localDir)
	}
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	tw := tar.NewWriter(gz)
	if err := filepath.Walk(localDir, func(path string, fi os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		rel, rerr := filepath.Rel(localDir, path)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return nil
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
		_, cerr := io.Copy(tw, f)
		_ = f.Close()
		return cerr
	}); err != nil {
		return "", fmt.Errorf("snapshot: tar build: %w", err)
	}
	if err := tw.Close(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}

	ts := time.Now().UTC().Unix()
	key := m.snapshotKey(workspaceID, ts)
	put := &s3.PutObjectInput{
		Bucket:      aws.String(m.cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String("application/gzip"),
	}
	m.applySSE(put)
	if _, err := m.uploader.Upload(ctx, put); err != nil {
		return "", fmt.Errorf("snapshot upload: %w", err)
	}

	// Atomic-flip LATEST. The pointer is intentionally text (just the
	// timestamp) so an operator can `aws s3 cp` it for debugging.
	latest := &s3.PutObjectInput{
		Bucket:      aws.String(m.cfg.Bucket),
		Key:         aws.String(m.latestKey(workspaceID)),
		Body:        strings.NewReader(strconv.FormatInt(ts, 10)),
		ContentType: aws.String("text/plain"),
	}
	m.applySSE(latest)
	if _, err := m.uploader.Upload(ctx, latest); err != nil {
		return "", fmt.Errorf("snapshot latest pointer: %w", err)
	}

	if err := m.reapOlder(ctx, workspaceID, ts); err != nil {
		m.logger.Warn().Err(err).Str("workspace", workspaceID).Msg("snapshot reap older")
	}
	m.logger.Info().
		Str("workspace", workspaceID).
		Str("key", key).
		Int("bytes", buf.Len()).
		Msg("snapshot checkpoint")
	return key, nil
}

// Restore reads LATEST for the workspace, downloads the named snapshot,
// and extracts it into destDir. If LATEST is missing (fresh workspace
// that has never checkpointed), Restore creates destDir empty and
// returns nil — boot is free to proceed with an empty filesystem.
func (m *Manager) Restore(ctx context.Context, workspaceID, destDir string) error {
	if !m.Enabled() {
		return nil
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	tsStr, ok, err := m.readLatest(ctx, workspaceID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	ts, perr := strconv.ParseInt(strings.TrimSpace(tsStr), 10, 64)
	if perr != nil {
		return fmt.Errorf("snapshot: invalid LATEST contents %q: %w", tsStr, perr)
	}
	key := m.snapshotKey(workspaceID, ts)
	tmp, err := os.CreateTemp("", "ironflyer-snapshot-*.tar.gz")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := m.dl.Download(ctx, tmp, &s3.GetObjectInput{
		Bucket: aws.String(m.cfg.Bucket),
		Key:    aws.String(key),
	}); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("snapshot download: %w", err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = tmp.Close()
		return err
	}
	gz, err := gzip.NewReader(tmp)
	if err != nil {
		_ = tmp.Close()
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, hdr.Name)
		// Defence-in-depth against tar slip — every entry must resolve
		// inside destDir.
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) &&
			target != filepath.Clean(destDir) {
			return fmt.Errorf("snapshot: path escape %q", hdr.Name)
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
	m.logger.Info().Str("workspace", workspaceID).Str("key", key).Msg("snapshot restored")
	return nil
}

// DeletePrefix removes every snapshot object for the workspace,
// including LATEST. Called by Destroy.
func (m *Manager) DeletePrefix(ctx context.Context, workspaceID string) error {
	if !m.Enabled() {
		return nil
	}
	objs, err := m.listAll(ctx, workspaceID)
	if err != nil {
		return err
	}
	if len(objs) == 0 {
		return nil
	}
	ids := make([]s3types.ObjectIdentifier, 0, len(objs))
	for _, o := range objs {
		ids = append(ids, s3types.ObjectIdentifier{Key: o.Key})
	}
	// DeleteObjects supports up to 1000 keys per call; per-workspace
	// snapshot counts are bounded by Retention so a single call is
	// always sufficient.
	_, err = m.s3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(m.cfg.Bucket),
		Delete: &s3types.Delete{Objects: ids, Quiet: aws.Bool(true)},
	})
	return err
}

// applySSE attaches SSE-KMS to a PutObjectInput. When KMSKeyID is empty
// the bucket's default encryption configuration applies (Pulumi sets
// this to aws:kms with the workspace CMK).
func (m *Manager) applySSE(in *s3.PutObjectInput) {
	if m.cfg.KMSKeyID == "" {
		// Leave bucket defaults to enforce SSE-KMS.
		return
	}
	in.ServerSideEncryption = s3types.ServerSideEncryptionAwsKms
	in.SSEKMSKeyId = aws.String(m.cfg.KMSKeyID)
}

// readLatest returns the LATEST pointer contents, or (_, false, nil)
// when LATEST is absent.
func (m *Manager) readLatest(ctx context.Context, workspaceID string) (string, bool, error) {
	out, err := m.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(m.cfg.Bucket),
		Key:    aws.String(m.latestKey(workspaceID)),
	})
	if err != nil {
		// 404 is the "never checkpointed" case — surface as ok=false.
		if isNoSuchKey(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("snapshot read LATEST: %w", err)
	}
	defer out.Body.Close()
	buf, rerr := io.ReadAll(io.LimitReader(out.Body, 64))
	if rerr != nil {
		return "", false, rerr
	}
	return string(buf), true, nil
}

// reapOlder deletes snapshot tarballs beyond the configured retention
// window. The current LATEST snapshot is always kept regardless of
// position in the sorted list (it's the only one a fresh boot can
// land on without a custom restore).
func (m *Manager) reapOlder(ctx context.Context, workspaceID string, currentTS int64) error {
	objs, err := m.listSnapshots(ctx, workspaceID)
	if err != nil {
		return err
	}
	if len(objs) <= m.cfg.Retention {
		return nil
	}
	sort.Slice(objs, func(i, j int) bool { return objs[i].ts > objs[j].ts })
	excess := objs[m.cfg.Retention:]
	var ids []s3types.ObjectIdentifier
	for _, o := range excess {
		if o.ts == currentTS {
			continue
		}
		ids = append(ids, s3types.ObjectIdentifier{Key: aws.String(o.key)})
	}
	if len(ids) == 0 {
		return nil
	}
	_, err = m.s3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(m.cfg.Bucket),
		Delete: &s3types.Delete{Objects: ids, Quiet: aws.Bool(true)},
	})
	return err
}

type snapObj struct {
	key string
	ts  int64
}

func (m *Manager) listSnapshots(ctx context.Context, workspaceID string) ([]snapObj, error) {
	prefix := m.prefixFor(workspaceID) + "/"
	out, err := m.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(m.cfg.Bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, err
	}
	var snaps []snapObj
	for _, o := range out.Contents {
		if o.Key == nil {
			continue
		}
		key := *o.Key
		base := strings.TrimPrefix(key, prefix)
		if !strings.HasSuffix(base, ".tar.gz") {
			continue
		}
		tsStr := strings.TrimSuffix(base, ".tar.gz")
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			continue
		}
		snaps = append(snaps, snapObj{key: key, ts: ts})
	}
	return snaps, nil
}

func (m *Manager) listAll(ctx context.Context, workspaceID string) ([]s3types.Object, error) {
	prefix := m.prefixFor(workspaceID) + "/"
	var out []s3types.Object
	var token *string
	for {
		page, err := m.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(m.cfg.Bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, page.Contents...)
		if page.IsTruncated == nil || !*page.IsTruncated {
			break
		}
		token = page.NextContinuationToken
	}
	return out, nil
}

// isNoSuchKey is a forgiving check — the v2 SDK exposes NoSuchKey as a
// typed error in some paths and as a generic API error string in
// others. Substring match keeps the check stable across SDK upgrades.
func isNoSuchKey(err error) bool {
	if err == nil {
		return false
	}
	var nsk *s3types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "NoSuchKey") || strings.Contains(msg, "status code: 404")
}
