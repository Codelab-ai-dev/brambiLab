package content

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type SEO struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type ProjectLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// ProjectFields describe the project itself; Status is the technical state, never publication.
type ProjectFields struct {
	Objective    string        `json:"objective,omitempty"`
	Status       string        `json:"status,omitempty"`
	Technologies []string      `json:"technologies,omitempty"`
	Links        []ProjectLink `json:"links,omitempty"`
	Results      string        `json:"results,omitempty"`
}

// Snapshot is everything a revision freezes: body and all editable metadata.
type Snapshot struct {
	Title         string         `json:"title"`
	Slug          string         `json:"slug"`
	Summary       string         `json:"summary"`
	Body          Node           `json:"body"`
	SEO           SEO            `json:"seo"`
	ProjectFields *ProjectFields `json:"project_fields,omitempty"`
	CategoryID    *string        `json:"category_id"`
	TagIDs        []string       `json:"tag_ids"`
	// CoverAssetID is a ready image (WEB-004). omitempty keeps hashes of older revisions stable.
	CoverAssetID *string `json:"cover_asset_id,omitempty"`
}

// FieldErrors maps snapshot fields to messages (the "fields" of a validation_failed error).
type FieldErrors map[string]string

func (f FieldErrors) Error() string { return "invalid snapshot" }

var projectStatuses = set("idea", "in_development", "paused", "completed")

// Normalize validates s for a content of the given kind and returns its canonical form and the
// body's plain text. A media node yields a *ValidationError with code media_not_available.
func (s Snapshot) Normalize(kind string, opts DocumentOptions) (Snapshot, string, error) {
	errs := FieldErrors{}
	out := Snapshot{
		Title:   strings.TrimSpace(s.Title),
		Slug:    s.Slug,
		Summary: strings.TrimSpace(s.Summary),
		SEO:     SEO{Title: strings.TrimSpace(s.SEO.Title), Description: strings.TrimSpace(s.SEO.Description)},
		TagIDs:  []string{},
	}
	checkText(errs, "title", out.Title, 1, 200)
	if !slugPattern.MatchString(out.Slug) || len(out.Slug) > 120 {
		errs["slug"] = "use 1-120 lowercase letters, digits and single hyphens"
	}
	checkText(errs, "summary", out.Summary, 0, 500)
	checkText(errs, "seo.title", out.SEO.Title, 0, 70)
	checkText(errs, "seo.description", out.SEO.Description, 0, 160)

	if s.CategoryID != nil && *s.CategoryID != "" {
		id := strings.ToLower(*s.CategoryID)
		if !uuidPattern.MatchString(id) {
			errs["category_id"] = "must be a UUID"
		}
		out.CategoryID = &id
	}
	if s.CoverAssetID != nil && *s.CoverAssetID != "" {
		id := strings.ToLower(*s.CoverAssetID)
		if !uuidPattern.MatchString(id) {
			errs["cover_asset_id"] = "must be a UUID"
		}
		out.CoverAssetID = &id
	}
	seen := map[string]bool{}
	for _, t := range s.TagIDs {
		id := strings.ToLower(t)
		switch {
		case !uuidPattern.MatchString(id):
			errs["tag_ids"] = "every tag id must be a UUID"
		case seen[id]:
			errs["tag_ids"] = "tag ids must be unique"
		}
		seen[id] = true
		out.TagIDs = append(out.TagIDs, id)
	}
	if len(out.TagIDs) > 20 {
		errs["tag_ids"] = "at most 20 tags"
	}
	sort.Strings(out.TagIDs)

	switch {
	case kind == "project" && s.ProjectFields != nil:
		pf := normalizeProject(errs, *s.ProjectFields)
		out.ProjectFields = &pf
	case kind != "project" && s.ProjectFields != nil:
		errs["project_fields"] = "only projects have project fields"
	}

	body, plain, err := ValidateDocument(s.Body, opts)
	if err != nil {
		if ve, ok := err.(*ValidationError); ok && ve.Code == "media_not_available" {
			return Snapshot{}, "", err
		}
		errs["body"] = err.Error()
	}
	out.Body = body
	if len(errs) > 0 {
		return Snapshot{}, "", errs
	}
	return out, plain, nil
}

func normalizeProject(errs FieldErrors, pf ProjectFields) ProjectFields {
	out := ProjectFields{
		Objective: strings.TrimSpace(pf.Objective),
		Status:    pf.Status,
		Results:   strings.TrimSpace(pf.Results),
	}
	checkText(errs, "project_fields.objective", out.Objective, 0, 1000)
	checkText(errs, "project_fields.results", out.Results, 0, 2000)
	if out.Status != "" && !projectStatuses[out.Status] {
		errs["project_fields.status"] = "must be idea, in_development, paused or completed"
	}
	if len(pf.Technologies) > 30 {
		errs["project_fields.technologies"] = "at most 30 technologies"
	}
	seen := map[string]bool{}
	for _, t := range pf.Technologies {
		t = strings.TrimSpace(t)
		if t == "" || utf8.RuneCountInString(t) > 50 || seen[strings.ToLower(t)] {
			errs["project_fields.technologies"] = "technologies must be unique, non-empty and at most 50 characters"
			continue
		}
		seen[strings.ToLower(t)] = true
		out.Technologies = append(out.Technologies, t)
	}
	if len(pf.Links) > 20 {
		errs["project_fields.links"] = "at most 20 links"
	}
	for i, l := range pf.Links {
		label := strings.TrimSpace(l.Label)
		key := fmt.Sprintf("project_fields.links[%d]", i)
		if label == "" || utf8.RuneCountInString(label) > 100 {
			errs[key+".label"] = "label is required (at most 100 characters)"
		}
		u, err := url.Parse(l.URL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || len(l.URL) > maxHrefLen {
			errs[key+".url"] = "must be an http or https URL"
		}
		out.Links = append(out.Links, ProjectLink{Label: label, URL: l.URL})
	}
	return out
}

func checkText(errs FieldErrors, field, v string, min, max int) {
	n := utf8.RuneCountInString(v)
	switch {
	case !utf8.ValidString(v):
		errs[field] = "must be valid UTF-8"
	case n < min:
		errs[field] = "is required"
	case n > max:
		errs[field] = fmt.Sprintf("at most %d characters", max)
	default:
		for _, r := range v {
			if unicode.IsControl(r) && r != '\n' && r != '\t' {
				errs[field] = "contains control characters"
				return
			}
		}
	}
}

// AssetRef is one use of an asset inside a snapshot, as recorded in revision_assets.
type AssetRef struct {
	ID    string
	Usage string // image, video, poster, download, cover
}

// AssetRefs lists every asset the snapshot uses (body media nodes and the cover).
func (s Snapshot) AssetRefs() []AssetRef {
	var refs []AssetRef
	seen := map[AssetRef]bool{}
	add := func(id, usage string) {
		r := AssetRef{ID: id, Usage: usage}
		if id != "" && !seen[r] {
			seen[r] = true
			refs = append(refs, r)
		}
	}
	var walk func(n Node)
	walk = func(n Node) {
		str := func(k string) string { v, _ := n.Attrs[k].(string); return v }
		switch n.Type {
		case "image":
			add(str("assetId"), "image")
		case "video":
			add(str("assetId"), "video")
			add(str("posterAssetId"), "poster")
		case "download":
			add(str("assetId"), "download")
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(s.Body)
	if s.CoverAssetID != nil {
		add(*s.CoverAssetID, "cover")
	}
	return refs
}

// Hash is the SHA-256 of the canonical JSON of a normalized snapshot: equal snapshots, equal hash.
func (s Snapshot) Hash() []byte {
	b, _ := CanonicalJSON(s)
	sum := sha256.Sum256(b)
	return sum[:]
}

// CanonicalJSON marshals without HTML escaping; struct fields keep declaration order and map keys
// are sorted by encoding/json, so the output is deterministic.
func CanonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// DecodeStrict decodes JSON rejecting unknown fields and keeping numbers exact (json.Number).
func DecodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("unexpected data after JSON value")
	}
	return nil
}
