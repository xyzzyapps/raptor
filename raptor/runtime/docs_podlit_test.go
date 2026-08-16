package raptor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocsPodLitWeaves(t *testing.T) {
	docs := filepath.Join("..", "docs")
	if _, err := os.Stat(docs); err != nil {
		docs = "docs"
	}
	entries, err := os.ReadDir(docs)
	if err != nil {
		t.Fatalf("docs dir: %v", err)
	}
	var n int
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".pod") && !strings.HasSuffix(name, ".md") {
			continue
		}
		n++
		b, err := os.ReadFile(filepath.Join(docs, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		doc, err := ParsePodDoc(string(b))
		if err != nil {
			t.Fatalf("ParsePodDoc %s: %v", name, err)
		}
		md := WeaveMarkdown(doc)
		if strings.TrimSpace(md) == "" {
			t.Errorf("%s: empty weave", name)
		}
	}
	if n < 10 {
		t.Fatalf("expected many docs, got %d", n)
	}
}
