package document

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentJSONIsDeterministic(t *testing.T) {
	line := 4
	doc := Document{
		Schema: SchemaVersion,
		Canvas: Canvas{Background: "default"},
		Elements: []Element{
			{ID: "bff", Kind: "box", X: 1, Y: 2, W: 10, H: 4, Label: "BFF"},
			{ID: "call", Kind: "arrow", From: "bff", To: "api", Ref: &Reference{Path: "src/client.go", Line: &line}},
		},
	}
	first, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("same document produced different JSON")
	}
}

func TestValidateDocumentChecksReferencesAndArrows(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "client.go"), []byte("package client\n\nfunc Call() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	line := 3
	doc := Document{
		Schema: SchemaVersion,
		Elements: []Element{
			{ID: "bff", Kind: "box", W: 10, H: 4},
			{ID: "api", Kind: "box", W: 10, H: 4},
			{ID: "call", Kind: "arrow", From: "bff", To: "api", Ref: &Reference{Path: "src/client.go", Line: &line}},
		},
	}
	if err := doc.Validate(root); err != nil {
		t.Fatalf("valid document rejected: %v", err)
	}

	doc.Elements[2].To = "missing"
	if err := doc.Validate(root); err == nil || !strings.Contains(err.Error(), "missing element") {
		t.Fatalf("expected missing target error, got %v", err)
	}
}

func TestValidateRejectsRelationsThatDoNotConnectDistinctNodes(t *testing.T) {
	cases := []struct {
		name string
		from string
		to   string
		want string
	}{
		{name: "relation source", from: "link", to: "target", want: "must connect boxes"},
		{name: "relation target", from: "source", to: "link", want: "must connect boxes"},
		{name: "self relation", from: "source", to: "source", want: "cannot connect a node to itself"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := Document{
				Schema: SchemaVersion,
				Elements: []Element{
					{ID: "source", Kind: "box", W: 10, H: 4},
					{ID: "target", Kind: "box", W: 10, H: 4},
					{ID: "link", Kind: "arrow", From: "source", To: "target"},
					{ID: "invalid", Kind: "arrow", From: tc.from, To: tc.to},
				},
			}
			if err := doc.ValidateStructure(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateStructure error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestShapeCatalogProvidesExtensibleSpecializedNodes(t *testing.T) {
	definitions := SpecializedShapeDefinitions()
	if len(definitions) != 3 {
		t.Fatalf("specialized shape count = %d, want 3", len(definitions))
	}
	for _, definition := range definitions {
		if definition.Shape == ShapeBox {
			t.Fatal("default box should not be in the specialized catalog")
		}
		if definition.DefaultWidth <= 0 || definition.DefaultHeight <= 0 {
			t.Fatalf("shape %q has invalid default size", definition.Shape)
		}
	}
}

func TestValidateRejectsUnknownAndUndersizedShapes(t *testing.T) {
	base := []Element{{ID: "node", Kind: "box", W: 18, H: 10}}
	for _, element := range []Element{
		{ID: "node", Kind: "box", W: 18, H: 10, Shape: "unknown"},
		{ID: "node", Kind: "box", W: 18, H: 9, Shape: string(ShapeDatabase)},
	} {
		doc := Document{Schema: SchemaVersion, Elements: append([]Element(nil), base...)}
		doc.Elements[0] = element
		if err := doc.ValidateStructure(); err == nil {
			t.Fatalf("shape %q was accepted", element.Shape)
		}
	}
}

func TestRepoPathRejectsAbsoluteAndEscapingPaths(t *testing.T) {
	root := t.TempDir()
	for _, input := range []string{"../outside.diagram.json", "/tmp/outside.diagram.json"} {
		if _, err := repoPath(root, input); err == nil {
			t.Fatalf("expected %q to be rejected", input)
		}
	}
}

func TestArrowPositionIsNotPersisted(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "architecture.diagram.json")
	doc := Document{
		Schema: SchemaVersion,
		Elements: []Element{
			{ID: "from", Kind: "box", W: 10, H: 4},
			{ID: "to", Kind: "box", W: 10, H: 4},
			{ID: "link", Kind: "arrow", From: "from", To: "to", X: 99, Y: 99},
		},
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"x": 99`) || strings.Contains(string(data), `"y": 99`) {
		t.Fatal("arrow position leaked into persisted JSON")
	}
}

func TestAnnotationAndControlPointsRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "architecture.diagram.json")
	doc := Document{
		Schema: SchemaVersion,
		Elements: []Element{
			{ID: "from", Kind: "box", W: 10, H: 4},
			{ID: "to", Kind: "box", W: 10, H: 4},
			{ID: "link", Kind: "arrow", From: "from", To: "to", ControlPoints: []Point{{X: 15, Y: 2}}},
			{ID: "note", Kind: "annotation", X: 2, Y: 8, W: 24, H: 4, Text: "plain\ntext"},
		},
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Elements[2].ControlPoints; len(got) != 1 || got[0] != (Point{X: 15, Y: 2}) {
		t.Fatalf("control points after round trip = %#v", got)
	}
	if loaded.Elements[3].Text != "plain\ntext" {
		t.Fatalf("annotation text after round trip = %q", loaded.Elements[3].Text)
	}
	if err := loaded.ValidateStructure(); err != nil {
		t.Fatalf("round-tripped annotation rejected: %v", err)
	}
}

func TestShapeRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "architecture.diagram.json")
	doc := Document{
		Schema:   SchemaVersion,
		Elements: []Element{{ID: "db", Kind: "box", W: 18, H: 10, Shape: string(ShapeDatabase)}},
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Elements[0].Shape != string(ShapeDatabase) {
		t.Fatalf("shape after round trip = %q, want %q", loaded.Elements[0].Shape, ShapeDatabase)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "architecture.diagram.json")
	doc := New()
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Schema != doc.Schema || loaded.Canvas.Background != "default" {
		t.Fatalf("round trip changed document: %#v", loaded)
	}
}

func TestHasArrow(t *testing.T) {
	doc := Document{
		Elements: []Element{
			{ID: "a", Kind: "arrow", From: "x", To: "y"},
			{ID: "b", Kind: "arrow", From: "y", To: "z"},
		},
	}
	if !doc.HasArrow("x", "y") {
		t.Fatal("HasArrow(x, y) should be true")
	}
	if doc.HasArrow("x", "z") {
		t.Fatal("HasArrow(x, z) should be false")
	}
}

func TestNextID(t *testing.T) {
	doc := Document{
		Elements: []Element{
			{ID: "box-0"}, {ID: "arrow-2"}, {ID: "box-5"},
		},
	}
	if got := doc.NextID(); got != 6 {
		t.Fatalf("NextID = %d, want 6", got)
	}
}
