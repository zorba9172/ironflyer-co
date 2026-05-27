package compliance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// buildAuditTarGz packages the AuditBundle into a tar.gz with three
// canonical files:
//
//   - README.md          — human-readable summary + evidence template
//   - controls.json      — machine-readable control result set
//   - attestation.jwt    — the signed verification token
//
// Layout is deterministic so audits diff cleanly across re-exports.
func buildAuditTarGz(b AuditBundle) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	now := b.GeneratedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	readme := renderReadme(b)
	if err := writeTarFile(tw, "README.md", []byte(readme), now); err != nil {
		return nil, err
	}
	controlsJSON, err := json.MarshalIndent(b.Controls, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := writeTarFile(tw, "controls.json", controlsJSON, now); err != nil {
		return nil, err
	}
	if err := writeTarFile(tw, "attestation.jwt", []byte(b.AttestationJWT), now); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeTarFile is the boilerplate-free per-file helper.
func writeTarFile(tw *tar.Writer, name string, body []byte, mtime time.Time) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0o644,
		Size:    int64(len(body)),
		ModTime: mtime,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(body)
	return err
}

// renderReadme is the operator-facing summary the bundle ships with.
// Markdown only — no images, no inline HTML, so the bundle stays
// auditor-portable.
func renderReadme(b AuditBundle) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Ironflyer Compliance Audit Bundle\n\n")
	fmt.Fprintf(&sb, "- Project: `%s`\n", b.ProjectID)
	fmt.Fprintf(&sb, "- Framework: %s (`%s`)\n", b.Framework.Label, b.FrameworkKey)
	fmt.Fprintf(&sb, "- Generated at: %s\n", b.GeneratedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&sb, "- Controls evaluated: %d\n\n", len(b.Controls))

	pass, fail, na := 0, 0, 0
	for _, r := range b.Controls {
		switch r.Status {
		case StatusPass:
			pass++
		case StatusFail:
			fail++
		case StatusNA:
			na++
		}
	}
	fmt.Fprintf(&sb, "## Verdict\n\n- Pass: %d\n- Fail: %d\n- N/A: %d\n\n", pass, fail, na)

	if len(b.Framework.EvidenceTemplates) > 0 {
		sb.WriteString("## Evidence sections\n\n")
		for _, t := range b.Framework.EvidenceTemplates {
			fmt.Fprintf(&sb, "- %s\n", t)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Findings\n\n")
	if len(b.Controls) == 0 {
		sb.WriteString("_No controls evaluated._\n")
	}
	for _, r := range b.Controls {
		fmt.Fprintf(&sb, "### %s — %s\n\n", r.ControlKey, r.Status)
		if r.Path != "" {
			fmt.Fprintf(&sb, "- Path: `%s`\n", r.Path)
		}
		fmt.Fprintf(&sb, "- Severity: %s\n", r.Severity)
		fmt.Fprintf(&sb, "- Evidence: %s\n\n", r.Evidence)
	}

	sb.WriteString("## Attestation\n\nVerify the bundled `attestation.jwt` against the Ironflyer attestation public key.\n")
	return sb.String()
}

// inlineDataURL wraps the bundle bytes as a base64 data URL so the
// GraphQL resolver can hand the bundle to the browser without an S3
// dependency. Format: `data:application/gzip;base64,<payload>`.
func inlineDataURL(b []byte) string {
	return "data:application/gzip;base64," + base64.StdEncoding.EncodeToString(b)
}
