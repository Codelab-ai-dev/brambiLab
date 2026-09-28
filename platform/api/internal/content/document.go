// Package content implements private editorial content, translations and immutable revisions
// (web-v1.md §6–7). This file validates and canonicalizes the document format v1 (§7.1).
package content

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	SchemaVersion = 1
	MaxDepth      = 32
	MaxNodes      = 20000
	MaxTextChars  = 200000
	maxTableCols  = 20
	maxTableRows  = 500
	maxHrefLen    = 2000
)

// Node is one element of the canonical document. Unknown JSON keys are rejected when decoding.
type Node struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []Node         `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
}

type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// ValidationError points at the offending part of the document.
type ValidationError struct {
	Code    string // validation_failed or media_not_available
	Path    string
	Message string
}

func (e *ValidationError) Error() string { return e.Path + ": " + e.Message }

type DocumentOptions struct {
	// AllowMedia accepts image/video/download nodes. False until assets exist (WEB-004).
	AllowMedia bool
}

var (
	videoIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	languagePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9+#._-]{0,31}$`)
	uuidPattern     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// Allowed children per context.
var (
	blockTypes      = set("paragraph", "heading", "bulletList", "orderedList", "blockquote", "codeBlock", "horizontalRule", "table", "youtube", "image", "video", "download")
	listItemTypes   = set("paragraph", "bulletList", "orderedList", "codeBlock", "blockquote")
	blockquoteTypes = set("paragraph", "heading", "bulletList", "orderedList", "codeBlock", "blockquote")
	markOrder       = map[string]int{"link": 0, "bold": 1, "italic": 2, "code": 3}
)

func set(items ...string) map[string]bool {
	m := map[string]bool{}
	for _, i := range items {
		m[i] = true
	}
	return m
}

type validator struct {
	opts      DocumentOptions
	nodes     int
	textChars int
	plain     strings.Builder
}

// ValidateDocument checks a document against schema v1 and returns its canonical form and plain
// text. Canonical form: only known attributes with normalized types, marks in a fixed order.
func ValidateDocument(doc Node, opts DocumentOptions) (Node, string, error) {
	v := &validator{opts: opts}
	if doc.Type != "doc" {
		return Node{}, "", v.fail("body", "root must be a doc node")
	}
	if len(doc.Attrs) > 0 || doc.Text != "" || len(doc.Marks) > 0 {
		return Node{}, "", v.fail("body", "doc accepts only content")
	}
	children, err := v.children("body", doc.Content, blockTypes, 1, 0)
	if err != nil {
		return Node{}, "", err
	}
	return Node{Type: "doc", Content: children}, strings.TrimSpace(v.plain.String()), nil
}

func (v *validator) fail(path, msg string, args ...any) error {
	return &ValidationError{Code: "validation_failed", Path: path, Message: fmt.Sprintf(msg, args...)}
}

func (v *validator) children(path string, nodes []Node, allowed map[string]bool, depth, min int) ([]Node, error) {
	if len(nodes) < min {
		return nil, v.fail(path+".content", "needs at least %d child node(s)", min)
	}
	out := make([]Node, 0, len(nodes))
	for i, n := range nodes {
		p := fmt.Sprintf("%s.content[%d]", path, i)
		if !allowed[n.Type] {
			return nil, v.fail(p, "node type %q is not allowed here", n.Type)
		}
		c, err := v.node(p, n, depth)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (v *validator) node(path string, n Node, depth int) (Node, error) {
	if depth > MaxDepth {
		return Node{}, v.fail(path, "document is nested deeper than %d levels", MaxDepth)
	}
	v.nodes++
	if v.nodes > MaxNodes {
		return Node{}, v.fail(path, "document has more than %d nodes", MaxNodes)
	}
	if n.Type != "text" && (n.Text != "" || len(n.Marks) > 0) {
		return Node{}, v.fail(path, "only text nodes carry text or marks")
	}
	out := Node{Type: n.Type}
	var err error

	switch n.Type {
	case "paragraph":
		err = v.noAttrs(path, n)
		if err == nil {
			out.Content, err = v.inline(path, n.Content, depth, true)
		}
		v.plain.WriteString("\n")
	case "heading":
		var level int
		level, err = v.intAttr(path, n.Attrs, "level", true, 2, 4)
		if err == nil {
			err = v.onlyAttrs(path, n.Attrs, "level")
		}
		if err == nil {
			out.Attrs = map[string]any{"level": level}
			out.Content, err = v.inline(path, n.Content, depth, true)
		}
		v.plain.WriteString("\n")
	case "bulletList":
		err = v.noAttrs(path, n)
		if err == nil {
			out.Content, err = v.children(path, n.Content, set("listItem"), depth+1, 1)
		}
	case "orderedList":
		var start int
		start, err = v.intAttr(path, n.Attrs, "start", false, 1, 1000000)
		if err == nil {
			err = v.onlyAttrs(path, n.Attrs, "start")
		}
		if err == nil {
			if start > 1 {
				out.Attrs = map[string]any{"start": start}
			}
			out.Content, err = v.children(path, n.Content, set("listItem"), depth+1, 1)
		}
	case "listItem":
		err = v.noAttrs(path, n)
		if err == nil {
			out.Content, err = v.children(path, n.Content, listItemTypes, depth+1, 1)
		}
	case "blockquote":
		err = v.noAttrs(path, n)
		if err == nil {
			out.Content, err = v.children(path, n.Content, blockquoteTypes, depth+1, 1)
		}
	case "codeBlock":
		out, err = v.codeBlock(path, n)
	case "horizontalRule":
		err = v.leaf(path, n)
	case "table":
		out, err = v.table(path, n, depth)
	case "youtube":
		out, err = v.youtube(path, n)
	case "image", "video", "download":
		out, err = v.media(path, n)
	default:
		err = v.fail(path, "unknown node type %q", n.Type)
	}
	return out, err
}

// inline validates paragraph-like content: text and (optionally) hard breaks.
func (v *validator) inline(path string, nodes []Node, depth int, allowBreak bool) ([]Node, error) {
	out := make([]Node, 0, len(nodes))
	for i, n := range nodes {
		p := fmt.Sprintf("%s.content[%d]", path, i)
		v.nodes++
		if v.nodes > MaxNodes {
			return nil, v.fail(p, "document has more than %d nodes", MaxNodes)
		}
		switch n.Type {
		case "text":
			t, err := v.text(p, n, true)
			if err != nil {
				return nil, err
			}
			out = append(out, t)
		case "hardBreak":
			if !allowBreak {
				return nil, v.fail(p, "line breaks are not supported inside table cells")
			}
			if err := v.leaf(p, n); err != nil {
				return nil, err
			}
			v.plain.WriteString("\n")
			out = append(out, Node{Type: "hardBreak"})
		default:
			return nil, v.fail(p, "node type %q is not allowed inside text", n.Type)
		}
	}
	return out, nil
}

func (v *validator) text(path string, n Node, allowMarks bool) (Node, error) {
	if n.Text == "" {
		return Node{}, v.fail(path, "text nodes cannot be empty")
	}
	if len(n.Attrs) > 0 || len(n.Content) > 0 {
		return Node{}, v.fail(path, "text nodes accept only text and marks")
	}
	if !utf8.ValidString(n.Text) {
		return Node{}, v.fail(path, "text is not valid UTF-8")
	}
	v.textChars += utf8.RuneCountInString(n.Text)
	if v.textChars > MaxTextChars {
		return Node{}, v.fail(path, "document has more than %d characters", MaxTextChars)
	}
	v.plain.WriteString(n.Text)
	out := Node{Type: "text", Text: n.Text}
	if len(n.Marks) == 0 {
		return out, nil
	}
	if !allowMarks {
		return Node{}, v.fail(path, "marks are not allowed here")
	}
	seen := map[string]bool{}
	for i, m := range n.Marks {
		mp := fmt.Sprintf("%s.marks[%d]", path, i)
		if _, ok := markOrder[m.Type]; !ok {
			return Node{}, v.fail(mp, "unknown mark %q", m.Type)
		}
		if seen[m.Type] {
			return Node{}, v.fail(mp, "duplicate mark %q", m.Type)
		}
		seen[m.Type] = true
		cm := Mark{Type: m.Type}
		if m.Type == "link" {
			href, err := v.stringAttr(mp, m.Attrs, "href", true, maxHrefLen)
			if err != nil {
				return Node{}, err
			}
			if err := v.onlyAttrs(mp, m.Attrs, "href"); err != nil {
				return Node{}, err
			}
			if err := checkHref(href); err != nil {
				return Node{}, v.fail(mp+".attrs.href", "%s", err.Error())
			}
			cm.Attrs = map[string]any{"href": href}
		} else if len(m.Attrs) > 0 {
			return Node{}, v.fail(mp, "mark %q takes no attributes", m.Type)
		}
		out.Marks = append(out.Marks, cm)
	}
	sort.Slice(out.Marks, func(i, j int) bool { return markOrder[out.Marks[i].Type] < markOrder[out.Marks[j].Type] })
	return out, nil
}

func (v *validator) codeBlock(path string, n Node) (Node, error) {
	lang, err := v.stringAttr(path, n.Attrs, "language", false, 32)
	if err != nil {
		return Node{}, err
	}
	if err := v.onlyAttrs(path, n.Attrs, "language"); err != nil {
		return Node{}, err
	}
	if lang != "" && !languagePattern.MatchString(lang) {
		return Node{}, v.fail(path+".attrs.language", "invalid language name")
	}
	out := Node{Type: "codeBlock"}
	if lang != "" {
		out.Attrs = map[string]any{"language": lang}
	}
	for i, c := range n.Content {
		p := fmt.Sprintf("%s.content[%d]", path, i)
		v.nodes++
		if c.Type != "text" {
			return Node{}, v.fail(p, "code blocks contain only plain text")
		}
		t, err := v.text(p, c, false)
		if err != nil {
			return Node{}, err
		}
		out.Content = append(out.Content, t)
	}
	v.plain.WriteString("\n")
	return out, nil
}

// table accepts simple tables only: a header row, then body rows, same width, one paragraph per cell.
func (v *validator) table(path string, n Node, depth int) (Node, error) {
	if err := v.noAttrs(path, n); err != nil {
		return Node{}, err
	}
	if len(n.Content) == 0 || len(n.Content) > maxTableRows {
		return Node{}, v.fail(path, "tables need 1 to %d rows", maxTableRows)
	}
	out := Node{Type: "table"}
	width := -1
	for r, row := range n.Content {
		rp := fmt.Sprintf("%s.content[%d]", path, r)
		v.nodes++
		if row.Type != "tableRow" {
			return Node{}, v.fail(rp, "tables contain only rows")
		}
		if err := v.noAttrs(rp, row); err != nil {
			return Node{}, err
		}
		if len(row.Content) == 0 || len(row.Content) > maxTableCols {
			return Node{}, v.fail(rp, "rows need 1 to %d cells", maxTableCols)
		}
		if width == -1 {
			width = len(row.Content)
		} else if len(row.Content) != width {
			return Node{}, v.fail(rp, "all rows must have %d cells", width)
		}
		cellType := "tableCell"
		if r == 0 {
			cellType = "tableHeader"
		}
		outRow := Node{Type: "tableRow"}
		for c, cell := range row.Content {
			cp := fmt.Sprintf("%s.content[%d]", rp, c)
			v.nodes++
			if cell.Type != cellType {
				return Node{}, v.fail(cp, "expected %s (first row is the header, merged cells are not supported)", cellType)
			}
			if err := v.noAttrs(cp, cell); err != nil {
				return Node{}, err
			}
			if len(cell.Content) != 1 || cell.Content[0].Type != "paragraph" {
				return Node{}, v.fail(cp, "each cell holds exactly one paragraph")
			}
			para := cell.Content[0]
			pp := cp + ".content[0]"
			v.nodes++
			if err := v.noAttrs(pp, para); err != nil {
				return Node{}, err
			}
			inline, err := v.inline(pp, para.Content, depth+3, false)
			if err != nil {
				return Node{}, err
			}
			v.plain.WriteString(" ")
			outRow.Content = append(outRow.Content, Node{Type: cellType, Content: []Node{{Type: "paragraph", Content: inline}}})
		}
		v.plain.WriteString("\n")
		out.Content = append(out.Content, outRow)
	}
	return out, nil
}

func (v *validator) youtube(path string, n Node) (Node, error) {
	id, err := v.stringAttr(path, n.Attrs, "videoId", true, 11)
	if err != nil {
		return Node{}, err
	}
	if !videoIDPattern.MatchString(id) {
		return Node{}, v.fail(path+".attrs.videoId", "invalid YouTube video id")
	}
	start, err := v.intAttr(path, n.Attrs, "start", false, 0, 86400)
	if err != nil {
		return Node{}, err
	}
	if err := v.onlyAttrs(path, n.Attrs, "videoId", "start"); err != nil {
		return Node{}, err
	}
	if len(n.Content) > 0 {
		return Node{}, v.fail(path, "youtube takes no content")
	}
	attrs := map[string]any{"videoId": id}
	if start > 0 {
		attrs["start"] = start
	}
	return Node{Type: "youtube", Attrs: attrs}, nil
}

// media validates the WEB-004 nodes; they are rejected while assets do not exist.
func (v *validator) media(path string, n Node) (Node, error) {
	if !v.opts.AllowMedia {
		return Node{}, &ValidationError{Code: "media_not_available", Path: path,
			Message: fmt.Sprintf("%s nodes require uploaded assets (available with WEB-004)", n.Type)}
	}
	if len(n.Content) > 0 {
		return Node{}, v.fail(path, "%s takes no content", n.Type)
	}
	attrs := map[string]any{}
	id, err := v.stringAttr(path, n.Attrs, "assetId", true, 36)
	if err != nil {
		return Node{}, err
	}
	if !uuidPattern.MatchString(id) {
		return Node{}, v.fail(path+".attrs.assetId", "assetId must be a lowercase UUID")
	}
	attrs["assetId"] = id
	var allowed []string
	switch n.Type {
	case "image":
		allowed = []string{"assetId", "alt", "caption"}
		alt, err := v.stringAttr(path, n.Attrs, "alt", true, 300)
		if err != nil {
			return Node{}, err
		}
		attrs["alt"] = alt
	case "video":
		allowed = []string{"assetId", "posterAssetId", "caption"}
		poster, err := v.stringAttr(path, n.Attrs, "posterAssetId", false, 36)
		if err != nil {
			return Node{}, err
		}
		if poster != "" {
			if !uuidPattern.MatchString(poster) {
				return Node{}, v.fail(path+".attrs.posterAssetId", "posterAssetId must be a lowercase UUID")
			}
			attrs["posterAssetId"] = poster
		}
	case "download":
		allowed = []string{"assetId", "label"}
		label, err := v.stringAttr(path, n.Attrs, "label", true, 200)
		if err != nil {
			return Node{}, err
		}
		if strings.TrimSpace(label) == "" {
			return Node{}, v.fail(path+".attrs.label", "download label is required")
		}
		attrs["label"] = label
	}
	if n.Type != "download" {
		caption, err := v.stringAttr(path, n.Attrs, "caption", false, 500)
		if err != nil {
			return Node{}, err
		}
		if caption != "" {
			attrs["caption"] = caption
		}
	}
	if err := v.onlyAttrs(path, n.Attrs, allowed...); err != nil {
		return Node{}, err
	}
	return Node{Type: n.Type, Attrs: attrs}, nil
}

func (v *validator) noAttrs(path string, n Node) error {
	if len(n.Attrs) > 0 {
		return v.fail(path, "%s takes no attributes", n.Type)
	}
	return nil
}

func (v *validator) leaf(path string, n Node) error {
	if len(n.Attrs) > 0 || len(n.Content) > 0 {
		return v.fail(path, "%s takes no attributes or content", n.Type)
	}
	return nil
}

func (v *validator) onlyAttrs(path string, attrs map[string]any, allowed ...string) error {
	ok := set(allowed...)
	for k := range attrs {
		if !ok[k] {
			return v.fail(path+".attrs", "unknown attribute %q", k)
		}
	}
	return nil
}

func (v *validator) stringAttr(path string, attrs map[string]any, key string, required bool, maxLen int) (string, error) {
	raw, present := attrs[key]
	if !present || raw == nil {
		if required {
			return "", v.fail(path+".attrs."+key, "is required")
		}
		return "", nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", v.fail(path+".attrs."+key, "must be a string")
	}
	if utf8.RuneCountInString(s) > maxLen || !utf8.ValidString(s) {
		return "", v.fail(path+".attrs."+key, "must be valid text of at most %d characters", maxLen)
	}
	return s, nil
}

// intAttr accepts JSON integers only (decoded with UseNumber).
func (v *validator) intAttr(path string, attrs map[string]any, key string, required bool, min, max int) (int, error) {
	raw, present := attrs[key]
	if !present || raw == nil {
		if required {
			return 0, v.fail(path+".attrs."+key, "is required")
		}
		return 0, nil
	}
	num, ok := raw.(json.Number)
	if !ok {
		return 0, v.fail(path+".attrs."+key, "must be an integer")
	}
	i, err := num.Int64()
	if err != nil || i < int64(min) || i > int64(max) {
		return 0, v.fail(path+".attrs."+key, "must be an integer between %d and %d", min, max)
	}
	return int(i), nil
}

// checkHref allows http(s), mailto, site-relative paths and in-page anchors; nothing executable.
func checkHref(href string) error {
	if href == "" {
		return fmt.Errorf("link target is empty")
	}
	for _, r := range href {
		if r < 0x20 || r == 0x7f || r == '\\' {
			return fmt.Errorf("link contains control characters or backslashes")
		}
	}
	if strings.HasPrefix(href, "#") || (strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "//")) {
		return nil
	}
	u, err := url.Parse(href)
	if err != nil {
		return fmt.Errorf("link is not a valid URL")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Host == "" {
			return fmt.Errorf("link needs a host")
		}
		if u.User != nil {
			return fmt.Errorf("links with credentials are not allowed")
		}
		return nil
	case "mailto":
		if u.Opaque == "" {
			return fmt.Errorf("mailto link needs an address")
		}
		return nil
	default:
		return fmt.Errorf("link scheme %q is not allowed", u.Scheme)
	}
}
