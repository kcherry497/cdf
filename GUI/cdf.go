package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// CDF v1 schema (mirrors ~/.claude/skills/CDF/spec.md).
// Unknown fields are round-tripped via the RawFields catch-all so we don't
// silently drop stuff a future spec adds.

type Meta struct {
	CDFVersion int    `json:"cdf_version"`
	Title      string `json:"title"`
	Subtitle   string `json:"subtitle,omitempty"`
	Updated    string `json:"updated"`
	RawFields  map[string]any `json:"-"`
}

type Row struct {
	Date      string         `json:"date"`
	Action    string         `json:"action"`
	Detail    string         `json:"detail,omitempty"`
	RawFields map[string]any `json:"-"`
}

type Section struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Note      string         `json:"note,omitempty"`
	Rows      []Row          `json:"rows"`
	RawFields map[string]any `json:"-"`
}

type CDF struct {
	Meta      Meta           `json:"meta"`
	Sections  []Section      `json:"sections"`
	RawFields map[string]any `json:"-"`
}

// ─────────────────────────── row classification ───────────────────────────

type Marker int

const (
	MarkerNone Marker = iota
	MarkerMilestone
	MarkerExposure
	MarkerDeferred
)

func (r Row) Marker() Marker {
	a := strings.TrimSpace(r.Action)
	if strings.HasPrefix(a, "MILESTONE:") {
		return MarkerMilestone
	}
	if strings.HasPrefix(a, "EXPOSURE:") {
		return MarkerExposure
	}
	d := strings.TrimSpace(r.Date)
	if d == "—" || d == "-" {
		return MarkerDeferred
	}
	return MarkerNone
}

// ─────────────────────────── io ───────────────────────────

func LoadCDF(path string) (*CDF, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c CDF
	if err := json.Unmarshal(buf, &c); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if c.Meta.CDFVersion != 1 {
		return nil, fmt.Errorf("unsupported cdf_version: %d", c.Meta.CDFVersion)
	}
	if c.Meta.Title == "" {
		return nil, fmt.Errorf("missing meta.title")
	}
	if c.Sections == nil {
		c.Sections = []Section{}
	}
	for i := range c.Sections {
		if c.Sections[i].ID == "" {
			return nil, fmt.Errorf("section %d missing id", i)
		}
		if c.Sections[i].Rows == nil {
			c.Sections[i].Rows = []Row{}
		}
	}
	return &c, nil
}

func SaveCDF(path string, c *CDF) error {
	c.Meta.Updated = time.Now().Format("2006-01-02")
	buf, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	return os.WriteFile(path, buf, 0644)
}

func NewSkeleton(title string) *CDF {
	return &CDF{
		Meta: Meta{
			CDFVersion: 1,
			Title:      title,
			Updated:    time.Now().Format("2006-01-02"),
		},
		Sections: []Section{
			{
				ID:   "section-1",
				Name: "New section",
				Rows: []Row{
					{Date: time.Now().Format("2006-01-02"), Action: "First action"},
				},
			},
		},
	}
}

// SectionIDFromName produces a URL-friendly id from a section name.
func SectionIDFromName(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash && b.Len() > 0 {
			b.WriteRune('-')
			prevDash = true
		}
	}
	s := strings.TrimRight(b.String(), "-")
	if s == "" {
		s = fmt.Sprintf("section-%d", time.Now().Unix())
	}
	return s
}
