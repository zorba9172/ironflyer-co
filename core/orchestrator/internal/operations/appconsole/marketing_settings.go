package appconsole

import (
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Marketing — SEO settings (config) + derived audit.
// ---------------------------------------------------------------------------

func (s *Store) SeoSettings(projectID, projectName string) SeoSettings {
	s.mu.RLock()
	cur, ok := s.seo[projectID]
	s.mu.RUnlock()
	if ok {
		return cur
	}
	def := SeoSettings{
		ProjectID:      projectID,
		Title:          projectName,
		Description:    "",
		Keywords:       []string{},
		OgImageURL:     "",
		TwitterHandle:  "",
		CanonicalURL:   "",
		Robots:         "index,follow",
		SitemapEnabled: true,
		UpdatedAt:      time.Now().UTC(),
	}
	s.mu.Lock()
	if _, exists := s.seo[projectID]; !exists {
		s.seo[projectID] = def
	}
	out := s.seo[projectID]
	s.mu.Unlock()
	return out
}

// SeoPatch carries the optional fields of an SEO update; nil leaves the
// current value untouched.
type SeoPatch struct {
	Title          *string
	Description    *string
	Keywords       []string
	OgImageURL     *string
	TwitterHandle  *string
	CanonicalURL   *string
	Robots         *string
	SitemapEnabled *bool
}

func (s *Store) UpdateSeoSettings(projectID, projectName string, p SeoPatch) SeoSettings {
	cur := s.SeoSettings(projectID, projectName)
	if p.Title != nil {
		cur.Title = *p.Title
	}
	if p.Description != nil {
		cur.Description = *p.Description
	}
	if p.Keywords != nil {
		cur.Keywords = p.Keywords
	}
	if p.OgImageURL != nil {
		cur.OgImageURL = *p.OgImageURL
	}
	if p.TwitterHandle != nil {
		cur.TwitterHandle = *p.TwitterHandle
	}
	if p.CanonicalURL != nil {
		cur.CanonicalURL = *p.CanonicalURL
	}
	if p.Robots != nil {
		cur.Robots = *p.Robots
	}
	if p.SitemapEnabled != nil {
		cur.SitemapEnabled = *p.SitemapEnabled
	}
	cur.UpdatedAt = time.Now().UTC()
	s.mu.Lock()
	s.seo[projectID] = cur
	s.mu.Unlock()
	return cur
}

func (s *Store) SeoAudit(projectID, projectName string) SeoAudit {
	c := s.SeoSettings(projectID, projectName)
	checks := []SeoCheck{
		{Key: "title", Label: "Title tag", Passed: len(c.Title) >= 10 && len(c.Title) <= 60, Detail: titleDetail(c.Title)},
		{Key: "description", Label: "Meta description", Passed: len(c.Description) >= 50 && len(c.Description) <= 160, Detail: descDetail(c.Description)},
		{Key: "keywords", Label: "Keywords", Passed: len(c.Keywords) > 0, Detail: keywordDetail(c.Keywords)},
		{Key: "og_image", Label: "Open Graph image", Passed: c.OgImageURL != "", Detail: presentDetail(c.OgImageURL != "", "social preview image set", "no og:image — links unfurl blank")},
		{Key: "canonical", Label: "Canonical URL", Passed: c.CanonicalURL != "", Detail: presentDetail(c.CanonicalURL != "", "canonical set", "missing canonical URL")},
		{Key: "sitemap", Label: "Sitemap", Passed: c.SitemapEnabled, Detail: presentDetail(c.SitemapEnabled, "sitemap.xml served", "sitemap disabled")},
		{Key: "robots", Label: "Robots policy", Passed: c.Robots != "" && !strings.Contains(c.Robots, "noindex"), Detail: presentDetail(c.Robots != "" && !strings.Contains(c.Robots, "noindex"), c.Robots, "noindex blocks search engines")},
	}
	passed := 0
	for _, ch := range checks {
		if ch.Passed {
			passed++
		}
	}
	return SeoAudit{Score: int(float64(passed) / float64(len(checks)) * 100), Checks: checks}
}

func titleDetail(s string) string {
	if s == "" {
		return "missing title"
	}
	if len(s) > 60 {
		return "too long (>60 chars)"
	}
	if len(s) < 10 {
		return "too short (<10 chars)"
	}
	return "good length"
}
func descDetail(s string) string {
	if s == "" {
		return "missing description"
	}
	if len(s) > 160 {
		return "too long (>160 chars)"
	}
	if len(s) < 50 {
		return "too short (<50 chars)"
	}
	return "good length"
}
func keywordDetail(k []string) string {
	if len(k) == 0 {
		return "no keywords set"
	}
	return strings.Join(k, ", ")
}
func presentDetail(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

// ---------------------------------------------------------------------------
// Settings — general + env vars (config).
// ---------------------------------------------------------------------------

func (s *Store) Settings(projectID, projectName string) Settings {
	s.mu.RLock()
	cur, ok := s.settings[projectID]
	s.mu.RUnlock()
	if ok {
		return cur
	}
	def := Settings{
		ProjectID:    projectID,
		DisplayName:  projectName,
		Visibility:   "private",
		Region:       "us-east",
		SupportEmail: "",
		EnvVars:      []EnvVar{},
		UpdatedAt:    time.Now().UTC(),
	}
	s.mu.Lock()
	if _, exists := s.settings[projectID]; !exists {
		s.settings[projectID] = def
	}
	out := s.settings[projectID]
	s.mu.Unlock()
	return out
}

// SettingsPatch carries the optional general-settings fields.
type SettingsPatch struct {
	DisplayName  *string
	Visibility   *string
	Region       *string
	SupportEmail *string
}

func (s *Store) UpdateSettings(projectID, projectName string, p SettingsPatch) Settings {
	cur := s.Settings(projectID, projectName)
	if p.DisplayName != nil {
		cur.DisplayName = *p.DisplayName
	}
	if p.Visibility != nil {
		cur.Visibility = *p.Visibility
	}
	if p.Region != nil {
		cur.Region = *p.Region
	}
	if p.SupportEmail != nil {
		cur.SupportEmail = *p.SupportEmail
	}
	cur.UpdatedAt = time.Now().UTC()
	s.mu.Lock()
	s.settings[projectID] = cur
	s.mu.Unlock()
	return cur
}

func (s *Store) SetEnvVar(projectID, projectName, key, value string, secret bool) Settings {
	cur := s.Settings(projectID, projectName)
	ev := EnvVar{Key: key, raw: value, Secret: secret, ValuePreview: maskValue(value, secret), UpdatedAt: time.Now().UTC()}
	replaced := false
	for i := range cur.EnvVars {
		if cur.EnvVars[i].Key == key {
			cur.EnvVars[i] = ev
			replaced = true
			break
		}
	}
	if !replaced {
		cur.EnvVars = append(cur.EnvVars, ev)
	}
	sort.Slice(cur.EnvVars, func(i, j int) bool { return cur.EnvVars[i].Key < cur.EnvVars[j].Key })
	cur.UpdatedAt = time.Now().UTC()
	s.mu.Lock()
	s.settings[projectID] = cur
	s.mu.Unlock()
	return cur
}

func (s *Store) DeleteEnvVar(projectID, projectName, key string) Settings {
	cur := s.Settings(projectID, projectName)
	out := cur.EnvVars[:0]
	for _, e := range cur.EnvVars {
		if e.Key != key {
			out = append(out, e)
		}
	}
	cur.EnvVars = out
	cur.UpdatedAt = time.Now().UTC()
	s.mu.Lock()
	s.settings[projectID] = cur
	s.mu.Unlock()
	return cur
}

// maskValue keeps secrets opaque on the wire: only the last 4 chars of a
// secret survive; non-secret values pass through.
func maskValue(v string, secret bool) string {
	if !secret {
		return v
	}
	if len(v) <= 4 {
		return "••••"
	}
	return "••••" + v[len(v)-4:]
}
