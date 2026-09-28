package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The shared fixtures (platform/contracts/fixtures/documents) are the contract between the
// TypeScript editor/Markdown adapters and this validator: each must be valid and already canonical.
func TestSharedFixturesAreValidCanonicalDocuments(t *testing.T) {
	files, err := filepath.Glob("../../../contracts/fixtures/documents/*.json")
	if err != nil || len(files) < 7 {
		t.Fatalf("fixtures not found: %v %v", files, err)
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			var doc Node
			if err := DecodeStrict(raw, &doc); err != nil {
				t.Fatal(err)
			}
			media := strings.HasSuffix(f, "media.json")
			if _, _, err := ValidateDocument(doc, DocumentOptions{}); media != (err != nil) {
				t.Fatalf("without media (media fixture=%v): %v", media, err)
			}
			canon, _, err := ValidateDocument(doc, DocumentOptions{AllowMedia: true})
			if err != nil {
				t.Fatal(err)
			}
			got, _ := CanonicalJSON(canon)
			want, _ := CanonicalJSON(doc)
			if string(got) != string(want) {
				t.Fatalf("fixture is not in canonical form:\n got %s\nwant %s", got, want)
			}
		})
	}
}
