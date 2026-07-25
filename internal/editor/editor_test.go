package editor

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"circinus/internal/document"
	"circinus/internal/geometry"

	"github.com/gdamore/tcell/v2"
)

type fakeScreen struct {
	mu       sync.Mutex
	events   chan tcell.Event
	width    int
	height   int
	cells    [][]rune
	contents []string
}

func newFakeScreen(width, height int) *fakeScreen {
	cells := make([][]rune, height)
	for y := range cells {
		cells[y] = make([]rune, width)
	}
	return &fakeScreen{
		events: make(chan tcell.Event, 64),
		width:  width, height: height,
		cells: cells,
	}
}

func (s *fakeScreen) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for y := range s.cells {
		for x := range s.cells[y] {
			s.cells[y][x] = 0
		}
	}
	s.contents = nil
}

func (s *fakeScreen) Fill(r rune, style tcell.Style) {}

func (s *fakeScreen) SetContent(x, y int, r rune, comb []rune, style tcell.Style) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if y >= 0 && y < len(s.cells) && x >= 0 && x < len(s.cells[y]) {
		s.cells[y][x] = r
	}
}

func (s *fakeScreen) PutStrStyled(x, y int, text string, style tcell.Style) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contents = append(s.contents, text)
}

func (s *fakeScreen) Size() (int, int) { return s.width, s.height }

func (s *fakeScreen) Show()          {}
func (s *fakeScreen) Sync()          {}
func (s *fakeScreen) Suspend() error { return nil }
func (s *fakeScreen) Resume() error  { return nil }
func (s *fakeScreen) Fini()          {}

func (s *fakeScreen) PollEvent() tcell.Event {
	return <-s.events
}

func TestCtrlBCreatesDatabaseNodeFromPopup(t *testing.T) {
	fs := newFakeScreen(80, 24)
	fs.events <- tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	fs.events <- tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	e := &Editor{screen: fs, doc: document.New(), nextElement: 0}
	e.handleKey(tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModNone))
	if len(e.doc.Elements) != 1 {
		t.Fatalf("created elements = %d, want 1", len(e.doc.Elements))
	}
	node := e.doc.Elements[0]
	if node.Shape != string(document.ShapeDatabase) {
		t.Fatalf("node shape = %q, want %q", node.Shape, document.ShapeDatabase)
	}
	if node.W != 18 || node.H != 10 {
		t.Fatalf("database size = %dx%d, want 18x10", node.W, node.H)
	}
}

func TestDatabaseLinesUseLayeredCylinder(t *testing.T) {
	lines := databaseLines(18, 10)
	if len(lines) != 10 {
		t.Fatalf("database lines = %d, want 10", len(lines))
	}
	if !strings.HasPrefix(lines[0], " .") || !strings.HasSuffix(lines[len(lines)-1], "'") {
		t.Fatalf("database outline = %#v", lines)
	}
	if strings.Count(strings.Join(lines, "\n"), "o") != 3 {
		t.Fatalf("database layer markers = %d, want 3", strings.Count(strings.Join(lines, "\n"), "o"))
	}
}

func TestConnectionTargetSkipsSourceAndArrows(t *testing.T) {
	elements := []document.Element{
		{ID: "source", Kind: "box"},
		{ID: "arrow", Kind: "arrow"},
		{ID: "target", Kind: "box"},
	}
	if got := nextBoxIndex(elements, 0, 0); got != 2 {
		t.Fatalf("next connection target = %d, want 2", got)
	}
}

func TestConfirmConnectionRejectsRelationSource(t *testing.T) {
	e := &Editor{
		doc: document.Document{Elements: []document.Element{
			{ID: "source", Kind: "box"},
			{ID: "link", Kind: "arrow", From: "source", To: "target"},
			{ID: "target", Kind: "box"},
		}},
		selected: 2, connectFrom: 1, connectMode: true, nextElement: 3,
	}
	e.confirmConnection()
	if len(e.doc.Elements) != 3 {
		t.Fatalf("relation source accepted as node; elements = %d, want 3", len(e.doc.Elements))
	}
}

func TestConfirmConnectionRejectsRelationTarget(t *testing.T) {
	e := &Editor{
		doc: document.Document{Elements: []document.Element{
			{ID: "source", Kind: "box"},
			{ID: "target", Kind: "box"},
			{ID: "link", Kind: "arrow", From: "source", To: "target"},
		}},
		selected: 2, connectFrom: 0, connectMode: true, nextElement: 3,
	}
	e.confirmConnection()
	if len(e.doc.Elements) != 3 {
		t.Fatalf("relation target accepted as node; elements = %d, want 3", len(e.doc.Elements))
	}
}

func TestNewConnectionStartsWithoutInventedLabel(t *testing.T) {
	e := &Editor{
		doc: document.Document{Elements: []document.Element{
			{ID: "source", Kind: "box"},
			{ID: "target", Kind: "box"},
		}},
		selected: 1, connectFrom: 0, connectMode: true, nextElement: 2,
		history: []document.Document{}, future: []document.Document{},
	}
	e.confirmConnection()
	if got := e.doc.Elements[2].Label; got != "" {
		t.Fatalf("new relation label = %q, want empty", got)
	}
}

func TestUndoRestoresPreviousNodeState(t *testing.T) {
	elements := make([]document.Element, 1, 2)
	elements[0] = document.Element{ID: "node", Kind: "box", W: 10, H: 4}
	e := &Editor{
		doc:      document.Document{Elements: elements},
		selected: 0,
		history:  []document.Document{}, future: []document.Document{},
	}
	e.moveSelected(4, 3)
	e.undo()
	if got := e.doc.Elements[0].X; got != 0 {
		t.Fatalf("undo restored x = %d, want 0", got)
	}
	if got := e.doc.Elements[0].Y; got != 0 {
		t.Fatalf("undo restored y = %d, want 0", got)
	}
}

func TestRedoRestoresUndoneNodeState(t *testing.T) {
	e := &Editor{
		doc: document.Document{Elements: []document.Element{
			{ID: "node", Kind: "box", W: 10, H: 4},
		}},
		selected: 0,
		history:  []document.Document{}, future: []document.Document{},
	}
	e.moveSelected(4, 3)
	e.undo()
	e.redo()
	if got := e.doc.Elements[0].X; got != 4 {
		t.Fatalf("redo restored x = %d, want 4", got)
	}
	if got := e.doc.Elements[0].Y; got != 3 {
		t.Fatalf("redo restored y = %d, want 3", got)
	}
}

func TestDeleteKeyDeletesSelectedElement(t *testing.T) {
	e := &Editor{
		doc: document.Document{Elements: []document.Element{
			{ID: "node", Kind: "box", W: 10, H: 4},
		}},
		selected: 0,
		history:  []document.Document{}, future: []document.Document{},
	}
	_, err := e.handleKey(tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone))
	if err != nil {
		t.Fatal(err)
	}
	if len(e.doc.Elements) != 0 {
		t.Fatalf("Delete left %d elements, want 0", len(e.doc.Elements))
	}
}

func TestUndoAndRedoKeyboardShortcuts(t *testing.T) {
	e := &Editor{
		doc: document.Document{Elements: []document.Element{
			{ID: "node", Kind: "box", W: 10, H: 4},
		}},
		selected: 0,
		history:  []document.Document{}, future: []document.Document{},
	}
	e.moveSelected(4, 3)
	if _, err := e.handleKey(tcell.NewEventKey(tcell.KeyRune, 'u', tcell.ModNone)); err != nil {
		t.Fatal(err)
	}
	if got := e.doc.Elements[0].X; got != 0 {
		t.Fatalf("u restored x = %d, want 0", got)
	}
	if _, err := e.handleKey(tcell.NewEventKey(tcell.KeyCtrlR, 0, tcell.ModNone)); err != nil {
		t.Fatal(err)
	}
	if got := e.doc.Elements[0].X; got != 4 {
		t.Fatalf("Ctrl-r restored x = %d, want 4", got)
	}
}

func TestAnnotationCanBeMovedByKeyboard(t *testing.T) {
	e := &Editor{
		doc:      document.Document{Elements: []document.Element{{ID: "note", Kind: "annotation", W: 20, H: 4, Text: "note"}}},
		selected: 0, history: []document.Document{}, future: []document.Document{},
	}
	e.moveSelected(3, 2)
	if e.doc.Elements[0].X != 3 || e.doc.Elements[0].Y != 2 {
		t.Fatalf("annotation position = %d,%d, want 3,2", e.doc.Elements[0].X, e.doc.Elements[0].Y)
	}
}

func TestRouteModeAddsMovesAndDeletesControlPoint(t *testing.T) {
	e := &Editor{
		doc: document.Document{Elements: []document.Element{
			{ID: "a", Kind: "box", W: 10, H: 4},
			{ID: "b", Kind: "box", X: 30, W: 10, H: 4},
			{ID: "ab", Kind: "arrow", From: "a", To: "b"},
		}},
		selected: 2, history: []document.Document{}, future: []document.Document{}, zoom: 100,
	}
	e.beginRouteEdit()
	e.handleRouteKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if len(e.doc.Elements[2].ControlPoints) != 1 {
		t.Fatalf("control points = %d, want 1", len(e.doc.Elements[2].ControlPoints))
	}
	before := e.doc.Elements[2].ControlPoints[0]
	e.handleRouteKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if e.doc.Elements[2].ControlPoints[0].X != before.X+1 {
		t.Fatalf("moved control point = %#v, want x + 1", e.doc.Elements[2].ControlPoints[0])
	}
	e.handleRouteKey(tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone))
	if len(e.doc.Elements[2].ControlPoints) != 0 {
		t.Fatalf("control points after delete = %d, want 0", len(e.doc.Elements[2].ControlPoints))
	}
}

func TestZoomChangesWithoutChangingLogicalCoordinates(t *testing.T) {
	e := &Editor{doc: document.Document{Elements: []document.Element{{ID: "node", Kind: "box", X: 10, Y: 8, W: 10, H: 4}}}, selected: 0, zoom: 100}
	e.changeZoom(1)
	if e.zoom != 125 || e.doc.Elements[0].X != 10 || e.doc.Elements[0].Y != 8 {
		t.Fatalf("zoom/doc state = %d/%d,%d, want 125/10,8", e.zoom, e.doc.Elements[0].X, e.doc.Elements[0].Y)
	}
}

func TestMouseMovesAnnotation(t *testing.T) {
	e := &Editor{
		doc:      document.Document{Elements: []document.Element{{ID: "note", Kind: "annotation", W: 10, H: 4, Text: "note"}}},
		selected: -1, hovered: -1, zoom: 100,
	}
	e.handleMouse(tcell.NewEventMouse(1, 1, tcell.Button1, tcell.ModNone))
	e.handleMouse(tcell.NewEventMouse(3, 3, tcell.Button1, tcell.ModNone))
	e.handleMouse(tcell.NewEventMouse(3, 3, tcell.ButtonNone, tcell.ModNone))
	if got := e.doc.Elements[0]; got.X != 2 || got.Y != 2 {
		t.Fatalf("mouse moved annotation to %d,%d, want 2,2", got.X, got.Y)
	}
}

func TestReferenceCandidatesRespectElementKind(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "services"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "services", "api.go"), []byte("package services\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := referenceCandidates(root, "services", false)
	if err != nil || len(files) != 1 || files[0] != "services/api.go" {
		t.Fatalf("file candidates = %#v, err=%v", files, err)
	}
	all, err := referenceCandidates(root, "services", true)
	if err != nil || len(all) != 2 || all[0] != "services" {
		t.Fatalf("node candidates = %#v, err=%v", all, err)
	}
}

func TestMovingArrowDoesNotChangeItsGeometry(t *testing.T) {
	e := &Editor{
		doc:      document.Document{Elements: []document.Element{{ID: "arrow", Kind: "arrow"}}},
		selected: 0,
		history:  []document.Document{}, future: []document.Document{},
	}
	e.moveSelected(4, 3)
	if e.doc.Elements[0].X != 0 || e.doc.Elements[0].Y != 0 {
		t.Fatal("moving an arrow changed its unused position")
	}
	if e.dirty {
		t.Fatal("moving an arrow marked the document dirty")
	}
}

func TestQuitRequiresTwoQPressesWhenDirty(t *testing.T) {
	e := &Editor{dirty: true}
	q := tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone)
	if done, err := e.handleKey(q); err != nil || done {
		t.Fatalf("first q = done:%v err:%v, want wait", done, err)
	}
	if !e.quitArmed {
		t.Fatal("first q did not arm discard confirmation")
	}
	if done, err := e.handleKey(q); err != nil || !done {
		t.Fatalf("second q = done:%v err:%v, want quit", done, err)
	}
}

func TestPlannedOppositeRelationsUseDistinctRoutes(t *testing.T) {
	e := &Editor{doc: document.Document{Elements: []document.Element{
		{ID: "a", Kind: "box", X: 0, Y: 0, W: 10, H: 4},
		{ID: "b", Kind: "box", X: 30, Y: 0, W: 10, H: 4},
		{ID: "a-to-b", Kind: "arrow", From: "a", To: "b"},
		{ID: "b-to-a", Kind: "arrow", From: "b", To: "a"},
	}}}
	routes := e.planRelationRoutes()
	shared := sharedRouteInterior(routes[2].points, routes[3].points)
	if shared != 0 {
		t.Fatalf("opposite routes share %d interior points", shared)
	}
	if routes[2].warning || routes[3].warning {
		t.Fatalf("opposite routes unexpectedly fell back: %#v %#v", routes[2], routes[3])
	}
}

func TestPlannedFanOutUsesDistinctSourcePorts(t *testing.T) {
	e := &Editor{doc: document.Document{Elements: []document.Element{
		{ID: "source", Kind: "box", X: 0, Y: 0, W: 10, H: 4},
		{ID: "left-target", Kind: "box", X: 30, Y: 0, W: 10, H: 4},
		{ID: "lower-target", Kind: "box", X: 30, Y: 10, W: 10, H: 4},
		{ID: "to-left", Kind: "arrow", From: "source", To: "left-target"},
		{ID: "to-lower", Kind: "arrow", From: "source", To: "lower-target"},
	}}}
	routes := e.planRelationRoutes()
	if routes[3].points[0] == routes[4].points[0] {
		t.Fatalf("fan-out relations share source port %#v", routes[3].points[0])
	}
}

func sharedRouteInterior(left, right []geometry.Point) int {
	if len(left) < 2 || len(right) < 2 {
		return 0
	}
	shared := 0
	for _, point := range left[1 : len(left)-1] {
		for _, other := range right[1 : len(right)-1] {
			if point == other {
				shared++
				break
			}
		}
	}
	return shared
}

func TestRelationsForTheSamePairUseSeparateLanes(t *testing.T) {
	elements := []document.Element{
		{ID: "one", Kind: "arrow", From: "bff", To: "api"},
		{ID: "other", Kind: "arrow", From: "bff", To: "api"},
		{ID: "reverse", Kind: "arrow", From: "api", To: "bff"},
	}
	if got := relationPairLane(elements, 0); got != -2 {
		t.Fatalf("first relation lane = %d, want -2", got)
	}
	if got := relationPairLane(elements, 1); got != 0 {
		t.Fatalf("second relation lane = %d, want 0", got)
	}
	if got := relationPairLane(elements, 2); got != 2 {
		t.Fatalf("reverse relation lane = %d, want 2", got)
	}
}

func TestUndoRestoresDeletedNodeAndRelations(t *testing.T) {
	e := &Editor{doc: document.Document{Elements: []document.Element{
		{ID: "bff", Kind: "box", W: 10, H: 4},
		{ID: "api", Kind: "box", W: 10, H: 4},
		{ID: "call", Kind: "arrow", From: "bff", To: "api"},
	}}, selected: 1, history: []document.Document{}, future: []document.Document{}}
	e.deleteSelected()
	if len(e.doc.Elements) != 1 {
		t.Fatalf("after delete elements = %d, want 1", len(e.doc.Elements))
	}
	e.undo()
	if len(e.doc.Elements) != 3 {
		t.Fatalf("after undo elements = %d, want 3", len(e.doc.Elements))
	}
}

func TestDeleteSelectedRemovesOrphanedRelations(t *testing.T) {
	e := &Editor{doc: document.Document{Elements: []document.Element{
		{ID: "a", Kind: "box"},
		{ID: "b", Kind: "box"},
		{ID: "c", Kind: "box"},
		{ID: "a-b", Kind: "arrow", From: "a", To: "b"},
		{ID: "b-c", Kind: "arrow", From: "b", To: "c"},
	}}, selected: 1, history: []document.Document{}, future: []document.Document{}}
	e.deleteSelected()
	for _, el := range e.doc.Elements {
		if el.Kind == "box" && el.ID == "b" {
			t.Fatal("box b should have been deleted")
		}
		if el.Kind == "arrow" && (el.From == "b" || el.To == "b") {
			t.Fatalf("orphaned relation %q still exists", el.ID)
		}
	}
}

func TestDrawWithFakeScreen(t *testing.T) {
	fs := newFakeScreen(80, 24)
	e := &Editor{
		screen: fs,
		doc: document.Document{Elements: []document.Element{
			{ID: "a", Kind: "box", X: 0, Y: 0, W: 10, H: 4, Label: "API"},
			{ID: "b", Kind: "box", X: 20, Y: 0, W: 10, H: 4, Label: "BFF"},
			{ID: "ab", Kind: "arrow", From: "a", To: "b", Label: "GET"},
		}},
		path: "test.diagram.json", selected: 0,
	}
	e.draw()
	if len(fs.contents) == 0 {
		t.Fatal("draw produced no output")
	}
	hasTitle := false
	for _, c := range fs.contents {
		if c == " Circinus  test.diagram.json " {
			hasTitle = true
		}
	}
	if !hasTitle {
		t.Fatal("draw did not render title bar")
	}
}
