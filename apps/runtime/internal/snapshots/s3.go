package snapshots

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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

// S3Manager implements Manager against an S3-compatible bucket
// (AWS S3, Cloudflare R2, MinIO). When cfg.Bucket is empty it becomes a
// no-op fallback that returns ErrNotConfigured on every method — that
// keeps wireup simple: the integration agent constructs one and lets
// callers branch on Enabled().
type S3Manager struct {
	cfg      Config
	layout   Layout
	logger   zerolog.Logger
	metrics  *Metrics
	excludes []string

	client   *s3.Client
	uploader *manager.Uploader
	dl       *manager.Downloader
}

// NewS3Manager builds an S3-backed snapshot manager. cfg.Bucket=""
// returns a disabled instance whose every method returns
// ErrNotConfigured (callers may inspect Enabled() to soft-skip).
func NewS3Manager(ctx context.Context, cfg Config, logger zerolog.Logger) (*S3Manager, error) {
	if cfg.Prefix == "" {
		cfg.Prefix = "snapshots"
	}
	if cfg.Retention <= 0 {
		cfg.Retention = 5
	}
	excludes := cfg.Excludes
	if len(excludes) == 0 {
		excludes = LoadExcludesFromEnv()
	}
	m := &S3Manager{
		cfg:      cfg,
		layout:   NewLayout(cfg.Prefix),
		logger:   logger.With().Str("component", "snapshots").Logger(),
		metrics:  &Metrics{},
		excludes: excludes,
	}
	if cfg.Bucket == "" {
		return m, nil
	}
	opts := []func(*awsconfig.LoadOptions) error{}
	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("snapshots: aws config: %w", err)
	}
	s3Opts := []func(*s3.Options){}
	if cfg.Endpoint != "" {
		ep := cfg.Endpoint
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(ep)
			// R2 and MinIO need path-style addressing.
			o.UsePathStyle = true
		})
	}
	m.client = s3.NewFromConfig(awsCfg, s3Opts...)
	m.uploader = manager.NewUploader(m.client)
	m.dl = manager.NewDownloader(m.client)
	return m, nil
}

// Enabled reports whether the manager is wired to real object storage.
func (m *S3Manager) Enabled() bool { return m != nil && m.client != nil && m.cfg.Bucket != "" }

// Metrics returns the in-process metrics view; never nil.
func (m *S3Manager) MetricsView() *Metrics { return m.metrics }

// applySSE attaches SSE-KMS when configured; otherwise relies on bucket
// default encryption.
func (m *S3Manager) applySSE(in *s3.PutObjectInput) {
	if m.cfg.KMSKeyID == "" {
		return
	}
	in.ServerSideEncryption = s3types.ServerSideEncryptionAwsKms
	in.SSEKMSKeyId = aws.String(m.cfg.KMSKeyID)
}

// RestoreLatest implements Manager.
func (m *S3Manager) RestoreLatest(ctx context.Context, workspaceID, destDir string) (Metadata, error) {
	if !m.Enabled() {
		return Metadata{}, ErrNotConfigured
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return Metadata{}, err
	}
	tsStr, ok, err := m.readLatest(ctx, workspaceID)
	if err != nil {
		m.metrics.ObserveRestoreFailure()
		return Metadata{}, err
	}
	if !ok {
		return Metadata{}, ErrNoSnapshot
	}
	ts, perr := strconv.ParseInt(strings.TrimSpace(tsStr), 10, 64)
	if perr != nil {
		m.metrics.ObserveRestoreFailure()
		return Metadata{}, fmt.Errorf("snapshots: invalid LATEST contents %q: %w", tsStr, perr)
	}
	key := m.layout.CheckpointKey(workspaceID, ts)
	tmp, err := os.CreateTemp("", "ironflyer-snap-restore-*.tar.zst")
	if err != nil {
		m.metrics.ObserveRestoreFailure()
		return Metadata{}, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	start := time.Now()
	if _, err := m.dl.Download(ctx, tmp, &s3.GetObjectInput{
		Bucket: aws.String(m.cfg.Bucket),
		Key:    aws.String(key),
	}); err != nil {
		_ = tmp.Close()
		m.metrics.ObserveRestoreFailure()
		return Metadata{}, fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}
	st, _ := tmp.Stat()
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = tmp.Close()
		m.metrics.ObserveRestoreFailure()
		return Metadata{}, err
	}
	if err := untarZstd(tmp, destDir); err != nil {
		_ = tmp.Close()
		m.metrics.ObserveRestoreFailure()
		return Metadata{}, err
	}
	_ = tmp.Close()
	dur := time.Since(start)
	size := int64(0)
	if st != nil {
		size = st.Size()
	}
	m.metrics.ObserveRestore(size, dur)
	md := Metadata{
		WorkspaceID:  workspaceID,
		ObjectKey:    key,
		Bucket:       m.cfg.Bucket,
		SizeBytes:    size,
		CreatedAt:    time.Unix(ts, 0).UTC(),
		RestoredAt:   Now(),
		CompressedAs: "zstd",
	}
	m.logger.Info().Str("workspace", workspaceID).Str("key", key).Int64("bytes", size).Msg("snapshot restored")
	return md, nil
}

// Checkpoint implements Manager.
func (m *S3Manager) Checkpoint(ctx context.Context, workspaceID, srcDir string, kind CheckpointKind) (Metadata, error) {
	if !m.Enabled() {
		return Metadata{}, ErrNotConfigured
	}
	info, err := os.Stat(srcDir)
	if err != nil {
		return Metadata{}, fmt.Errorf("snapshots: srcDir %q: %w", srcDir, err)
	}
	if !info.IsDir() {
		return Metadata{}, fmt.Errorf("snapshots: %q is not a directory", srcDir)
	}
	// Stream tar.zst into an in-memory buffer. Workspaces are bounded
	// by tenant quota; for very large workspaces the future cut is to
	// pipe straight into manager.Uploader.
	buf := &bytes.Buffer{}
	uncompressed, err := tarZstdDir(srcDir, buf, m.excludes)
	if err != nil {
		m.metrics.ObserveCheckpointFailure()
		return Metadata{}, err
	}
	ts := Now().Unix()
	key := m.layout.CheckpointKey(workspaceID, ts)
	put := &s3.PutObjectInput{
		Bucket:      aws.String(m.cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String("application/zstd"),
	}
	m.applySSE(put)
	start := time.Now()
	if _, err := m.uploader.Upload(ctx, put); err != nil {
		m.metrics.ObserveCheckpointFailure()
		return Metadata{}, fmt.Errorf("%w: %v", ErrUploadFailed, err)
	}
	// Flip LATEST only after the snapshot upload succeeds.
	latest := &s3.PutObjectInput{
		Bucket:      aws.String(m.cfg.Bucket),
		Key:         aws.String(m.layout.LatestKey(workspaceID)),
		Body:        strings.NewReader(strconv.FormatInt(ts, 10)),
		ContentType: aws.String("text/plain"),
	}
	m.applySSE(latest)
	if _, err := m.uploader.Upload(ctx, latest); err != nil {
		m.metrics.ObserveCheckpointFailure()
		return Metadata{}, fmt.Errorf("%w: latest pointer: %v", ErrUploadFailed, err)
	}
	dur := time.Since(start)
	m.metrics.ObserveCheckpoint(int64(buf.Len()), dur)
	if err := m.reapOlder(ctx, workspaceID, ts); err != nil {
		m.logger.Warn().Err(err).Str("workspace", workspaceID).Msg("snapshots reap older")
	}
	md := Metadata{
		WorkspaceID:  workspaceID,
		ObjectKey:    key,
		Bucket:       m.cfg.Bucket,
		SizeBytes:    int64(buf.Len()),
		Kind:         kind,
		CreatedAt:    time.Unix(ts, 0).UTC(),
		CompressedAs: "zstd",
	}
	m.logger.Info().
		Str("workspace", workspaceID).
		Str("key", key).
		Str("kind", string(kind)).
		Int64("uncompressed", uncompressed).
		Int("compressed", buf.Len()).
		Dur("dur", dur).
		Msg("snapshot checkpointed")
	return md, nil
}

// Archive implements Manager: copies the latest checkpoint to the
// archives/ key so the hot prefix can be reaped without losing state.
func (m *S3Manager) Archive(ctx context.Context, workspaceID string) (Metadata, error) {
	if !m.Enabled() {
		return Metadata{}, ErrNotConfigured
	}
	tsStr, ok, err := m.readLatest(ctx, workspaceID)
	if err != nil {
		m.metrics.ObserveArchiveFailure()
		return Metadata{}, err
	}
	if !ok {
		return Metadata{}, ErrNoSnapshot
	}
	ts, perr := strconv.ParseInt(strings.TrimSpace(tsStr), 10, 64)
	if perr != nil {
		m.metrics.ObserveArchiveFailure()
		return Metadata{}, fmt.Errorf("snapshots: invalid LATEST contents %q: %w", tsStr, perr)
	}
	src := m.layout.CheckpointKey(workspaceID, ts)
	dst := m.layout.ArchiveKey(workspaceID)
	// CopyObject keeps the upload bandwidth in-region. Source is
	// already encrypted via bucket defaults; we re-apply SSE on the
	// destination too so archived objects carry an explicit KMS key.
	copyIn := &s3.CopyObjectInput{
		Bucket:     aws.String(m.cfg.Bucket),
		Key:        aws.String(dst),
		CopySource: aws.String(m.cfg.Bucket + "/" + src),
	}
	if m.cfg.KMSKeyID != "" {
		copyIn.ServerSideEncryption = s3types.ServerSideEncryptionAwsKms
		copyIn.SSEKMSKeyId = aws.String(m.cfg.KMSKeyID)
	}
	if _, err := m.client.CopyObject(ctx, copyIn); err != nil {
		m.metrics.ObserveArchiveFailure()
		return Metadata{}, fmt.Errorf("snapshots: archive copy: %w", err)
	}
	// Stat the destination to record size; ignore failures (object
	// exists either way).
	var size int64
	if head, err := m.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(m.cfg.Bucket),
		Key:    aws.String(dst),
	}); err == nil && head.ContentLength != nil {
		size = *head.ContentLength
	}
	m.metrics.ObserveArchive(size)
	md := Metadata{
		WorkspaceID:  workspaceID,
		ObjectKey:    dst,
		Bucket:       m.cfg.Bucket,
		SizeBytes:    size,
		CreatedAt:    Now(),
		CompressedAs: "zstd",
	}
	m.logger.Info().Str("workspace", workspaceID).Str("archive", dst).Int64("bytes", size).Msg("snapshot archived")
	return md, nil
}

// Delete implements Manager: archive-first, then drop the hot
// workspace prefix. Archive object is never deleted by this method —
// retention worker (out of scope here) owns archive expiry.
func (m *S3Manager) Delete(ctx context.Context, workspaceID string) error {
	if !m.Enabled() {
		return ErrNotConfigured
	}
	// Make sure an archive exists before we delete the hot prefix.
	dst := m.layout.ArchiveKey(workspaceID)
	if _, err := m.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(m.cfg.Bucket),
		Key:    aws.String(dst),
	}); err != nil {
		// No archive yet — create one (best-effort).
		if _, aerr := m.Archive(ctx, workspaceID); aerr != nil && !errors.Is(aerr, ErrNoSnapshot) {
			return fmt.Errorf("snapshots: pre-delete archive: %w", aerr)
		}
	}
	// Drop every object under the workspace's hot prefix.
	prefix := m.layout.WorkspaceDir(workspaceID) + "/"
	objs, err := m.listAll(ctx, prefix)
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
	_, err = m.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(m.cfg.Bucket),
		Delete: &s3types.Delete{Objects: ids, Quiet: aws.Bool(true)},
	})
	return err
}

// Metadata implements Manager: returns a synthetic Metadata derived
// from LATEST. A future Postgres-backed metadata store is wired
// alongside without changing this signature.
func (m *S3Manager) Metadata(ctx context.Context, workspaceID string) (Metadata, error) {
	if !m.Enabled() {
		return Metadata{}, ErrNotConfigured
	}
	tsStr, ok, err := m.readLatest(ctx, workspaceID)
	if err != nil {
		return Metadata{}, err
	}
	if !ok {
		return Metadata{}, ErrNoSnapshot
	}
	ts, perr := strconv.ParseInt(strings.TrimSpace(tsStr), 10, 64)
	if perr != nil {
		return Metadata{}, fmt.Errorf("snapshots: invalid LATEST: %w", perr)
	}
	key := m.layout.CheckpointKey(workspaceID, ts)
	var size int64
	if head, herr := m.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(m.cfg.Bucket),
		Key:    aws.String(key),
	}); herr == nil && head.ContentLength != nil {
		size = *head.ContentLength
	}
	return Metadata{
		WorkspaceID:  workspaceID,
		ObjectKey:    key,
		Bucket:       m.cfg.Bucket,
		SizeBytes:    size,
		CreatedAt:    time.Unix(ts, 0).UTC(),
		CompressedAs: "zstd",
	}, nil
}

// readLatest returns (timestamp-string, found, err).
func (m *S3Manager) readLatest(ctx context.Context, workspaceID string) (string, bool, error) {
	out, err := m.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(m.cfg.Bucket),
		Key:    aws.String(m.layout.LatestKey(workspaceID)),
	})
	if err != nil {
		if isNoSuchKey(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("snapshots: read LATEST: %w", err)
	}
	defer out.Body.Close()
	buf, rerr := io.ReadAll(io.LimitReader(out.Body, 64))
	if rerr != nil {
		return "", false, rerr
	}
	return string(buf), true, nil
}

// reapOlder trims hot checkpoints beyond Retention.
func (m *S3Manager) reapOlder(ctx context.Context, workspaceID string, currentTS int64) error {
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
	_, err = m.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(m.cfg.Bucket),
		Delete: &s3types.Delete{Objects: ids, Quiet: aws.Bool(true)},
	})
	return err
}

type snapObj struct {
	key string
	ts  int64
}

func (m *S3Manager) listSnapshots(ctx context.Context, workspaceID string) ([]snapObj, error) {
	prefix := m.layout.WorkspaceDir(workspaceID) + "/"
	out, err := m.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
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
		if !strings.HasSuffix(base, ".tar.zst") {
			continue
		}
		tsStr := strings.TrimSuffix(base, ".tar.zst")
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			continue
		}
		snaps = append(snaps, snapObj{key: key, ts: ts})
	}
	return snaps, nil
}

func (m *S3Manager) listAll(ctx context.Context, prefix string) ([]s3types.Object, error) {
	var out []s3types.Object
	var token *string
	for {
		page, err := m.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
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

// isNoSuchKey tolerates both the typed and string forms the v2 SDK
// emits depending on the operation path.
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

// Ensure interface satisfaction at compile time.
var _ Manager = (*S3Manager)(nil)
