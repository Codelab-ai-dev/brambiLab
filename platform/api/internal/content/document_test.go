package content

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func decodeDoc(t *testing.T, js string) Node {
	t.Helper()
	var n Node
	if err := DecodeStrict([]byte(js), &n); err != nil {
		t.Fatalf("decode %s: %v", js, err)
	}
	return n
}

const fullDoc = `{"type":"doc","content":[
 {"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Motores y ñandú"}]},
 {"type":"paragraph","content":[
   {"type":"text","text":"negrita","marks":[{"type":"bold"}]},
   {"type":"text","text":" enlace","marks":[{"type":"italic"},{"type":"link","attrs":{"href":"https://example.com/a?b=1"}}]},
   {"type":"hardBreak"},
   {"type":"text","text":"x := 1","marks":[{"type":"code"}]}]},
 {"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"uno"}]},
   {"type":"orderedList","attrs":{"start":3},"content":[{"type":"listItem","content":[{"type":"paragraph"}]}]}]}]},
 {"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"cita"}]}]},
 {"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"func main() {}\n"}]},
 {"type":"horizontalRule"},
 {"type":"table","content":[
   {"type":"tableRow","content":[{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"Pin"}]}]},{"type":"tableHeader","content":[{"type":"paragraph"}]}]},
   {"type":"tableRow","content":[{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"GPIO4","marks":[{"type":"code"}]}]}]},{"type":"tableCell","content":[{"type":"paragraph"}]}]}]},
 {"type":"youtube","attrs":{"videoId":"dQw4w9WgXcQ","start":42}},
 {"type":"paragraph","content":[{"type":"text","text":"relativo","marks":[{"type":"link","attrs":{"href":"/es/proyectos/rover"}}]},{"type":"text","text":" ancla","marks":[{"type":"link","attrs":{"href":"#pinout"}}]},{"type":"text","text":" correo","marks":[{"type":"link","attrs":{"href":"mailto:hola@example.com"}}]}]}
]}`

func TestFullDocumentIsValidAndCanonical(t *testing.T) {
	doc, plain, err := ValidateDocument(decodeDoc(t, fullDoc), DocumentOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Motores y ñandú", "negrita", "func main()", "GPIO4", "cita"} {
		if !strings.Contains(plain, want) {
			t.Errorf("plain text misses %q:\n%s", want, plain)
		}
	}
	// Marks are canonically ordered (link before italic) regardless of input order.
	link := doc.Content[1].Content[1]
	if link.Marks[0].Type != "link" || link.Marks[1].Type != "italic" {
		t.Fatalf("marks not canonical: %+v", link.Marks)
	}
	// Canonical form is a fixed point: validating it again changes nothing.
	b1, _ := CanonicalJSON(doc)
	again, _, err := ValidateDocument(decodeDoc(t, string(b1)), DocumentOptions{})
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := CanonicalJSON(again)
	if string(b1) != string(b2) {
		t.Fatalf("canonical form is not stable:\n%s\n%s", b1, b2)
	}
	if strings.Contains(string(b1), `\u003c`) {
		t.Fatal("canonical JSON must not HTML-escape")
	}
}

func TestRejectedDocuments(t *testing.T) {
	para := func(inner string) string {
		return `{"type":"doc","content":[{"type":"paragraph","content":[` + inner + `]}]}`
	}
	link := func(href string) string {
		return para(`{"type":"text","text":"x","marks":[{"type":"link","attrs":{"href":` + fmt.Sprintf("%q", href) + `}}]}`)
	}
	cases := map[string]string{
		"javascript link":         link("javascript:alert(1)"),
		"javascript mixed case":   link("JaVaScRiPt:alert(1)"),
		"javascript with spaces":  link(" javascript:alert(1)"),
		"javascript control char": link("java\tscript:alert(1)"),
		"data url":                link("data:text/html;base64,PHNjcmlwdD4="),
		"vbscript":                link("vbscript:msgbox"),
		"protocol relative":       link("//evil.example/x"),
		"backslash":               link("/\\evil.example"),
		"credentials":             link("https://user:pass@example.com"),
		"file scheme":             link("file:///etc/passwd"),
		"unknown node":            `{"type":"doc","content":[{"type":"script","content":[]}]}`,
		"html node":               `{"type":"doc","content":[{"type":"html","attrs":{"value":"<iframe>"}}]}`,
		"iframe node":             `{"type":"doc","content":[{"type":"iframe","attrs":{"src":"https://evil.example"}}]}`,
		"unknown mark":            para(`{"type":"text","text":"x","marks":[{"type":"underline"}]}`),
		"unknown attr":            `{"type":"doc","content":[{"type":"paragraph","attrs":{"style":"color:red"}}]}`,
		"link extra attr":         para(`{"type":"text","text":"x","marks":[{"type":"link","attrs":{"href":"https://a.b","onclick":"x"}}]}`),
		"heading level 1":         `{"type":"doc","content":[{"type":"heading","attrs":{"level":1}}]}`,
		"heading level float":     `{"type":"doc","content":[{"type":"heading","attrs":{"level":2.5}}]}`,
		"heading level string":    `{"type":"doc","content":[{"type":"heading","attrs":{"level":"2"}}]}`,
		"empty text":              para(`{"type":"text","text":""}`),
		"text in paragraph":       `{"type":"doc","content":[{"type":"paragraph","text":"x"}]}`,
		"block inside paragraph":  para(`{"type":"paragraph"}`),
		"inline at top":           `{"type":"doc","content":[{"type":"text","text":"x"}]}`,
		"root not doc":            `{"type":"paragraph"}`,
		"marks in code block":     `{"type":"doc","content":[{"type":"codeBlock","content":[{"type":"text","text":"x","marks":[{"type":"bold"}]}]}]}`,
		"bad code language":       `{"type":"doc","content":[{"type":"codeBlock","attrs":{"language":"<script>"}}]}`,
		"bad youtube id":          `{"type":"doc","content":[{"type":"youtube","attrs":{"videoId":"x\"onload=1"}}]}`,
		"youtube url instead id":  `{"type":"doc","content":[{"type":"youtube","attrs":{"videoId":"https://youtu.be/dQw4w9WgXcQ"}}]}`,
		"empty list":              `{"type":"doc","content":[{"type":"bulletList","content":[]}]}`,
		"table without header":    `{"type":"doc","content":[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","content":[{"type":"paragraph"}]}]}]}]}`,
		"table ragged":            `{"type":"doc","content":[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","content":[{"type":"paragraph"}]},{"type":"tableHeader","content":[{"type":"paragraph"}]}]},{"type":"tableRow","content":[{"type":"tableCell","content":[{"type":"paragraph"}]}]}]}]}`,
		"table merged cell":       `{"type":"doc","content":[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","attrs":{"colspan":2},"content":[{"type":"paragraph"}]}]}]}]}`,
		"table cell two blocks":   `{"type":"doc","content":[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","content":[{"type":"paragraph"},{"type":"paragraph"}]}]}]}]}`,
		"break in table cell":     `{"type":"doc","content":[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"hardBreak"}]}]}]}]}]}`,
		"duplicate mark":          para(`{"type":"text","text":"x","marks":[{"type":"bold"},{"type":"bold"}]}`),
		"unknown top-level key":   `{"type":"doc","content":[],"html":"<b>x</b>"}`,
		"unknown node key":        `{"type":"doc","content":[{"type":"paragraph","style":"x"}]}`,
	}
	for name, js := range cases {
		t.Run(name, func(t *testing.T) {
			var n Node
			if err := DecodeStrict([]byte(js), &n); err != nil {
				return // rejected already at decoding (unknown keys): fine
			}
			if _, _, err := ValidateDocument(n, DocumentOptions{}); err == nil {
				t.Fatalf("accepted: %s", js)
			}
		})
	}
}

func TestLimits(t *testing.T) {
	deep := strings.Repeat(`{"type":"blockquote","content":[`, MaxDepth+1) + `{"type":"paragraph"}` + strings.Repeat(`]}`, MaxDepth+1)
	if _, _, err := ValidateDocument(decodeDoc(t, `{"type":"doc","content":[`+deep+`]}`), DocumentOptions{}); err == nil || !strings.Contains(err.Error(), "deeper") {
		t.Fatalf("depth limit: %v", err)
	}
	many := strings.TrimSuffix(strings.Repeat(`{"type":"horizontalRule"},`, MaxNodes+1), ",")
	if _, _, err := ValidateDocument(decodeDoc(t, `{"type":"doc","content":[`+many+`]}`), DocumentOptions{}); err == nil || !strings.Contains(err.Error(), "nodes") {
		t.Fatalf("node limit: %v", err)
	}
	long := fmt.Sprintf(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":%q}]}]}`, strings.Repeat("a", MaxTextChars+1))
	if _, _, err := ValidateDocument(decodeDoc(t, long), DocumentOptions{}); err == nil || !strings.Contains(err.Error(), "characters") {
		t.Fatalf("text limit: %v", err)
	}
}

func TestMediaNodesWaitForAssets(t *testing.T) {
	const id = "0b8f3a3e-5d2c-4c1a-9a57-3f0e2b6d9c11"
	docs := []string{
		`{"type":"doc","content":[{"type":"image","attrs":{"assetId":"` + id + `","alt":"Placa","caption":"Frente"}}]}`,
		`{"type":"doc","content":[{"type":"video","attrs":{"assetId":"` + id + `","posterAssetId":"` + id + `"}}]}`,
		`{"type":"doc","content":[{"type":"download","attrs":{"assetId":"` + id + `","label":"STL v1"}}]}`,
	}
	for _, js := range docs {
		_, _, err := ValidateDocument(decodeDoc(t, js), DocumentOptions{})
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Code != "media_not_available" {
			t.Fatalf("without assets: %v", err)
		}
		// The versioned node shape is already defined for WEB-004.
		if _, _, err := ValidateDocument(decodeDoc(t, js), DocumentOptions{AllowMedia: true}); err != nil {
			t.Fatalf("media node shape rejected: %v", err)
		}
	}
	bad := `{"type":"doc","content":[{"type":"image","attrs":{"assetId":"fake-id","alt":""}}]}`
	if _, _, err := ValidateDocument(decodeDoc(t, bad), DocumentOptions{AllowMedia: true}); err == nil {
		t.Fatal("non-UUID asset id accepted")
	}
}

func TestSnapshotNormalizeAndHash(t *testing.T) {
	body := decodeDoc(t, `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hola"}]}]}`)
	a := "3f1c2b1e-0000-4000-8000-000000000002"
	b := "3f1c2b1e-0000-4000-8000-000000000001"
	s := Snapshot{Title: "  Rover  ", Slug: "rover", Body: body, TagIDs: []string{a, b},
		ProjectFields: &ProjectFields{Status: "in_development", Technologies: []string{"ESP32", " Go "},
			Links: []ProjectLink{{Label: "Repo", URL: "https://github.com/x/y"}}}}
	n1, plain, err := s.Normalize("project", DocumentOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if n1.Title != "Rover" || plain != "Hola" || n1.TagIDs[0] != b || n1.ProjectFields.Technologies[1] != "Go" {
		t.Fatalf("normalized: %+v plain=%q", n1, plain)
	}
	// Tag order and surrounding whitespace do not change the hash; content does.
	s2 := s
	s2.TagIDs = []string{b, a}
	s2.Title = "Rover"
	n2, _, _ := s2.Normalize("project", DocumentOptions{})
	if string(n1.Hash()) != string(n2.Hash()) {
		t.Fatal("equivalent snapshots hash differently")
	}
	s2.Summary = "otro"
	n3, _, _ := s2.Normalize("project", DocumentOptions{})
	if string(n1.Hash()) == string(n3.Hash()) {
		t.Fatal("different snapshots hash equally")
	}

	bad := Snapshot{Title: "", Slug: "Mal Slug", Body: body, ProjectFields: &ProjectFields{Status: "published",
		Links: []ProjectLink{{Label: "x", URL: "javascript:alert(1)"}}}}
	_, _, err = bad.Normalize("article", DocumentOptions{})
	var fe FieldErrors
	if !errors.As(err, &fe) {
		t.Fatalf("want FieldErrors, got %v", err)
	}
	for _, f := range []string{"title", "slug", "project_fields"} {
		if fe[f] == "" {
			t.Errorf("missing error for %s: %v", f, fe)
		}
	}
	_, _, err = bad.Normalize("project", DocumentOptions{})
	errors.As(err, &fe)
	for _, f := range []string{"project_fields.status", "project_fields.links[0].url"} {
		if fe[f] == "" {
			t.Errorf("missing error for %s: %v", f, fe)
		}
	}
}
