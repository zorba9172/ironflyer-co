package appsec

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

const osvBatchURL = "https://api.osv.dev/v1/querybatch"

type OSVScanner struct{}

func (OSVScanner) ID() string { return "osv-dev" }

func (OSVScanner) Supports(inv Inventory) bool { return len(inv.Components) > 0 }

func (s OSVScanner) Scan(ctx context.Context, target Target, inv Inventory) ([]Finding, error) {
	if !ResolveConfig(target).EnableNetworkScanners {
		return nil, nil
	}
	reqBody, refs := buildOSVBatchRequest(inv.Components)
	if len(refs) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, osvBatchURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 8 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, nil
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, res.Body)
		return nil, nil
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, nil
	}
	return parseOSVBatchResponse(s.ID(), refs, raw), nil
}

type osvBatchRequest struct {
	Queries []osvQuery `json:"queries"`
}

type osvQuery struct {
	Package osvPackage `json:"package"`
	Version string     `json:"version,omitempty"`
}

type osvPackage struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
}

type osvRef struct {
	Component Component
	QueryIdx  int
}

func buildOSVBatchRequest(components []Component) (osvBatchRequest, []osvRef) {
	var req osvBatchRequest
	var refs []osvRef
	seen := map[string]bool{}
	for _, c := range components {
		ecosystem := osvEcosystem(c.Ecosystem)
		if ecosystem == "" || c.Name == "" || c.Version == "" {
			continue
		}
		version := normaliseVersion(c.Version)
		key := ecosystem + "\x00" + c.Name + "\x00" + version
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, osvRef{Component: c, QueryIdx: len(req.Queries)})
		req.Queries = append(req.Queries, osvQuery{
			Package: osvPackage{Ecosystem: ecosystem, Name: c.Name},
			Version: version,
		})
	}
	return req, refs
}

func parseOSVBatchResponse(toolID string, refs []osvRef, raw []byte) []Finding {
	var doc struct {
		Results []struct {
			Vulns []struct {
				ID       string   `json:"id"`
				Summary  string   `json:"summary,omitempty"`
				Details  string   `json:"details,omitempty"`
				Aliases  []string `json:"aliases,omitempty"`
				Modified string   `json:"modified,omitempty"`
				Severity []struct {
					Type  string `json:"type"`
					Score string `json:"score"`
				} `json:"severity,omitempty"`
				DatabaseSpecific map[string]any `json:"database_specific,omitempty"`
			} `json:"vulns,omitempty"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	refByIndex := map[int]Component{}
	for _, ref := range refs {
		refByIndex[ref.QueryIdx] = ref.Component
	}
	var out []Finding
	for i, result := range doc.Results {
		c, ok := refByIndex[i]
		if !ok {
			continue
		}
		for _, vuln := range result.Vulns {
			id := strings.TrimSpace(vuln.ID)
			if id == "" {
				continue
			}
			summary := firstNonEmpty(vuln.Summary, firstSentence(vuln.Details), id+" affects "+componentName(c))
			out = append(out, Finding{
				Tool:        toolID,
				Category:    CategoryDeps,
				Severity:    osvSeverity(vuln.DatabaseSpecific, vuln.Severity),
				RuleID:      "osv-" + strings.ToLower(id),
				Path:        c.Path,
				Package:     componentName(c),
				Summary:     summary,
				Remediation: "upgrade or replace the affected package; verify with OSV/govulncheck/npm audit after updating",
				Metadata: map[string]string{
					"ecosystem":  c.Ecosystem,
					"dependency": c.Name,
					"version":    c.Version,
					"osv_id":     id,
				},
			})
		}
	}
	return out
}

func osvEcosystem(ecosystem string) string {
	switch strings.ToLower(strings.TrimSpace(ecosystem)) {
	case "npm":
		return "npm"
	case "go":
		return "Go"
	case "pypi", "python":
		return "PyPI"
	default:
		return ""
	}
}

func osvSeverity(databaseSpecific map[string]any, scores []struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}) Severity {
	for _, key := range []string{"severity", "Severity"} {
		if raw, ok := databaseSpecific[key].(string); ok {
			switch strings.ToUpper(strings.TrimSpace(raw)) {
			case "CRITICAL":
				return SeverityCritical
			case "HIGH":
				return SeverityHigh
			case "MODERATE", "MEDIUM":
				return SeverityMedium
			case "LOW":
				return SeverityLow
			}
		}
	}
	for _, score := range scores {
		if strings.Contains(score.Score, "/C:H/") || strings.Contains(score.Score, "/I:H/") || strings.Contains(score.Score, "/A:H/") {
			return SeverityHigh
		}
	}
	return SeverityMedium
}

func firstSentence(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 180 {
		s = s[:180]
	}
	if idx := strings.Index(s, ". "); idx > 0 && idx < len(s) {
		return s[:idx+1]
	}
	return s
}
