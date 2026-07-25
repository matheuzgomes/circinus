package cli

import (
	"os"
	"path/filepath"
	"testing"

	"circinus/internal/document"
)

func TestRepoPathRejectsAbsoluteAndEscapingPaths(t *testing.T) {
	root := t.TempDir()
	for _, input := range []string{"../outside.diagram.json", "/tmp/outside.diagram.json"} {
		if _, err := repoPath(root, input); err == nil {
			t.Fatalf("expected %q to be rejected", input)
		}
	}
}

func TestFindDocumentsListsOnlyNativeDiagramFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"one.diagram.json", "nested/two.diagram.json", "nested/other.json"} {
		if err := os.WriteFile(filepath.Join(root, path), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := findDocuments(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"nested/two.diagram.json", "one.diagram.json"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("findDocuments = %#v, want %#v", got, want)
	}
}

func TestDocumentWarningsDetectOverlap(t *testing.T) {
	warnings := documentWarnings([]document.Element{
		{ID: "one", Kind: "box", X: 0, Y: 0, W: 10, H: 4},
		{ID: "two", Kind: "box", X: 5, Y: 2, W: 10, H: 4},
	})
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want one overlap warning", warnings)
	}
}

func TestNewDocumentCreatesValidFile(t *testing.T) {
	root := t.TempDir()
	input := "test.diagram.json"
	if err := newDocument(root, input); err != nil {
		t.Fatal(err)
	}
	doc, err := document.Load(filepath.Join(root, input))
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(root); err != nil {
		t.Fatalf("created document fails validation: %v", err)
	}
}

func TestCheckDocumentValidatesAndReportsWarnings(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "client.go"), []byte("package client\n\nfunc Call() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	input := "arch.diagram.json"
	if err := newDocument(root, input); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, input)
	doc, err := document.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Elements = []document.Element{
		{ID: "a", Kind: "box", X: 0, Y: 0, W: 10, H: 4},
		{ID: "b", Kind: "box", X: 5, Y: 2, W: 10, H: 4},
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	if err := checkDocument(root, input); err != nil {
		t.Fatalf("checkDocument failed: %v", err)
	}
}
