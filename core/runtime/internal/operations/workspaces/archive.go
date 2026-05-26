package workspaces

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/klauspost/compress/zstd"
	"github.com/rs/zerolog"
	"golang.org/x/sync/singleflight"
)

// Archiver moves a workspace between EFS (warm) and S3 (cold). Designed
// to be safe under concurrent calls: singleflight collapses parallel
// requests for the same workspace, and a pair of semaphore channels caps
// the per-pod archive/restore parallelism so a wave of idle workspaces
// doesn't saturate the pod's network or CPU.
//
// When BucketName is empty, the Archiver still implements the same
// methods but every operation is a no-op: useful for dev/single-pod
// deployments without S3 credentials.
type Archiver struct {
	bucket   string
	region   string
	prefix   string
	efsRoot  string
	s3       *s3.Client
	uploader *manager.Uploader
	dl       *manager.Downloader

	logger   zerolog.Logger
	archives chan struct{}
	restores chan struct{}
	group    singleflight.Group
	store    Store
}

// ArchiverConfig bundles the knobs the runtime needs to wire S3.
type ArchiverConfig struct {
	Bucket       string
	Region       string
	Prefix       string // defaults to "archives"
	EFSRoot      string
	Concurrency  int    // per-direction cap; default 4
	Store        Store  // optional — used by Archive/Restore to update rows
}

// NewArchiver constructs an Archiver. If cfg.Bucket is empty the
// returned Archiver is still non-nil but every method is a no-op.
func NewArchiver(ctx context.Context, cfg ArchiverConfig, logger zerolog.Logger) (*Archiver, error) {
	a := &Archiver{
		bucket:  cfg.Bucket,
		region:  cfg.Region,
		prefix:  strings.Trim(cfg.Prefix, "/"),
		efsRoot: cfg.EFSRoot,
		logger:  logger,
		store:   cfg.Store,
	}
	if a.prefix == "" {
		a.prefix = "archives"
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}
	a.archives = make(chan struct{}, cfg.Concurrency)
	a.restores = make(chan struct{}, cfg.Concurrency)
	if a.bucket == "" {
		return a, nil
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	a.s3 = s3.NewFromConfig(awsCfg)
	a.uploader = manager.NewUploader(a.s3)
	a.dl = manager.NewDownloader(a.s3)
	return a, nil
}

// Enabled reports whether real S3 archival is wired.
func (a *Archiver) Enabled() bool { return a != nil && a.s3 != nil && a.bucket != "" }

// Key returns the S3 object key the workspace would archive to.
func (a *Archiver) Key(workspaceID string) string {
	return a.prefix + "/" + workspaceID + ".tar.zst"
}

// Archive tars + zstds the workspace directory and uploads it to S3.
// Concurrency-capped by the archives channel. Singleflight collapses
// parallel calls for the same workspace ID. When BucketName is empty
// this is a no-op returning nil.
func (a *Archiver) Archive(ctx context.Context, ws Record) error {
	if !a.Enabled() {
		return nil
	}
	_, err, _ := a.group.Do("archive:"+ws.ID, func() (any, error) {
		select {
		case a.archives <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		defer func() { <-a.archives }()
		return nil, a.archiveOne(ctx, ws)
	})
	return err
}

func (a *Archiver) archiveOne(ctx context.Context, ws Record) error {
	dir := ws.EFSPath
	if dir == "" {
		dir = filepath.Join(a.efsRoot, ws.ID)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("archive: workspace dir missing: %w", err)
	}
	pr, pw := io.Pipe()
	go func() {
		// tar -> zstd -> pipe writer; close the writer with the eventual error.
		zw, zerr := zstd.NewWriter(pw, zstd.WithEncoderLevel(zstd.SpeedDefault))
		if zerr != nil {
			_ = pw.CloseWithError(zerr)
			return
		}
		tw := tar.NewWriter(zw)
		walkErr := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}
			hdr, err := tar.FileInfoHeader(fi, "")
			if err != nil {
				return err
			}
			hdr.Name = filepath.ToSlash(rel)
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if !fi.Mode().IsRegular() {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, f)
			_ = f.Close()
			return err
		})
		if cerr := tw.Close(); walkErr == nil {
			walkErr = cerr
		}
		if cerr := zw.Close(); walkErr == nil {
			walkErr = cerr
		}
		_ = pw.CloseWithError(walkErr)
	}()

	objKey := a.Key(ws.ID)
	_, err = a.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(a.bucket),
		Key:         aws.String(objKey),
		Body:        pr,
		ContentType: aws.String("application/zstd"),
	})
	if err != nil {
		return fmt.Errorf("s3 upload: %w", err)
	}
	if a.store != nil {
		if uerr := a.store.UpdateArchive(ctx, ws.ID, objKey); uerr != nil {
			a.logger.Warn().Err(uerr).Str("workspace", ws.ID).Msg("archive: update row")
		}
	}
	if rerr := os.RemoveAll(dir); rerr != nil {
		a.logger.Warn().Err(rerr).Str("workspace", ws.ID).Msg("archive: cleanup efs dir")
	}
	a.logger.Info().Str("workspace", ws.ID).Str("key", objKey).Msg("workspace archived")
	return nil
}

// Restore downloads the S3 archive for the workspace and extracts it
// back to EFS. Concurrency-capped by restores; singleflight collapses
// parallel restores of the same ID. Returns the EFS path on success.
func (a *Archiver) Restore(ctx context.Context, ws Record) (string, error) {
	if !a.Enabled() {
		return ws.EFSPath, nil
	}
	v, err, _ := a.group.Do("restore:"+ws.ID, func() (any, error) {
		select {
		case a.restores <- struct{}{}:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		defer func() { <-a.restores }()
		return a.restoreOne(ctx, ws)
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (a *Archiver) restoreOne(ctx context.Context, ws Record) (string, error) {
	objKey := ws.S3ArchiveKey
	if objKey == "" {
		objKey = a.Key(ws.ID)
	}
	dir := filepath.Join(a.efsRoot, ws.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Buffer the object into a temp file then re-open it as a reader. The
	// SDK's downloader needs an io.WriterAt; piping straight into tar via
	// a pipe + zstd would force us into single-stream mode anyway, so a
	// temp file keeps the code simple.
	tmp, err := os.CreateTemp("", "ironflyer-restore-*.tar.zst")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := a.dl.Download(ctx, tmp, &s3.GetObjectInput{
		Bucket: aws.String(a.bucket),
		Key:    aws.String(objKey),
	}); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("s3 download: %w", err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = tmp.Close()
		return "", err
	}
	zr, err := zstd.NewReader(tmp)
	if err != nil {
		_ = tmp.Close()
		return "", err
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		target := filepath.Join(dir, hdr.Name)
		if !strings.HasPrefix(target, dir) {
			return "", fmt.Errorf("archive escape: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return "", err
			}
			_ = out.Close()
		default:
			// Skip symlinks/devices for safety.
		}
	}
	if a.store != nil {
		_ = a.store.UpdateEFSPath(ctx, ws.ID, dir)
		_ = a.store.UpdateStatus(ctx, ws.ID, StatusRunning)
	}
	a.logger.Info().Str("workspace", ws.ID).Str("dir", dir).Msg("workspace restored")
	return dir, nil
}

// Scanner is the background loop that finds idle workspaces and ships
// them to S3. Runs as a single goroutine inside the runtime pod (no
// sidecar): one process, one place to look for archive logs, one place
// to wire metrics. Stops cleanly on ctx cancel.
type Scanner struct {
	store    Store
	archiver *Archiver
	idleFor  time.Duration
	tick     time.Duration
	logger   zerolog.Logger
}

// NewScanner builds the idle scanner.
func NewScanner(store Store, archiver *Archiver, idleFor, tick time.Duration, logger zerolog.Logger) *Scanner {
	if tick <= 0 {
		tick = 2 * time.Minute
	}
	return &Scanner{store: store, archiver: archiver, idleFor: idleFor, tick: tick, logger: logger}
}

// Run blocks until ctx is cancelled.
func (s *Scanner) Run(ctx context.Context) {
	if s == nil || !s.archiver.Enabled() {
		return
	}
	t := time.NewTicker(s.tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.pass(ctx)
		}
	}
}

func (s *Scanner) pass(ctx context.Context) {
	cutoff := time.Now().Add(-s.idleFor).UTC()
	candidates, err := s.store.IdleCandidates(ctx, cutoff, 16)
	if err != nil {
		s.logger.Warn().Err(err).Msg("idle scan: list")
		return
	}
	for _, r := range candidates {
		if err := s.archiver.Archive(ctx, r); err != nil {
			s.logger.Warn().Err(err).Str("workspace", r.ID).Msg("idle scan: archive")
		}
	}
}
