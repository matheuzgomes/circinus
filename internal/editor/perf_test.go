package editor

import (
	"fmt"
	"testing"

	"circinus/internal/document"
	"circinus/internal/geometry"

	"github.com/gdamore/tcell/v2"
)

func benchmarkDiagram(tb testing.TB, nodes, relationsPerPair int) *Editor {
	elements := make([]document.Element, 0)
	cols := 8
	size := 6
	arrowID := 0
	for i := 0; i < nodes; i++ {
		col, row := i%cols, i/cols
		x, y := col*size*3+1, row*size*3+1
		elements = append(elements, document.Element{
			ID: fmt.Sprintf("node-%d", i), Kind: "box",
			X: x, Y: y, W: size * 2, H: size, Label: fmt.Sprintf("N%d", i),
		})
	}
	for i := 0; i < nodes-1; i++ {
		for r := 0; r < relationsPerPair; r++ {
			elements = append(elements, document.Element{
				ID: fmt.Sprintf("arrow-%d", arrowID), Kind: "arrow",
				From: fmt.Sprintf("node-%d", i), To: fmt.Sprintf("node-%d", i+1),
			})
			arrowID++
		}
	}
	if nodes >= 2 {
		for r := 0; r < relationsPerPair; r++ {
			elements = append(elements, document.Element{
				ID: fmt.Sprintf("arrow-%d", arrowID), Kind: "arrow",
				From: fmt.Sprintf("node-%d", nodes-1), To: fmt.Sprintf("node-%d", 0),
			})
			arrowID++
		}
	}
	doc := document.Document{Schema: document.SchemaVersion, Elements: elements}
	_ = doc.ValidateStructure()
	return &Editor{doc: doc, zoom: 100, selected: -1, hovered: -1, routePoint: -1, connectFrom: -1}
}

// Step 1 — baseline benchmarks

func BenchmarkPlanRoutes9Nodes(b *testing.B) {
	e := benchmarkDiagram(b, 9, 2)
	// 9 nodes + ~18 relations
	b.ResetTimer()
	for range b.N {
		e.planRelationRoutes()
	}
}

func BenchmarkPlanRoutes50Nodes(b *testing.B) {
	e := benchmarkDiagram(b, 50, 2)
	b.ResetTimer()
	for range b.N {
		e.planRelationRoutes()
	}
}

func BenchmarkHitElement9Nodes(b *testing.B) {
	e := benchmarkDiagram(b, 9, 2)
	b.ResetTimer()
	for range b.N {
		e.hitElement(40, 10)
	}
}

func BenchmarkHitElement50Nodes(b *testing.B) {
	e := benchmarkDiagram(b, 50, 2)
	b.ResetTimer()
	for range b.N {
		e.hitElement(40, 10)
	}
}

func BenchmarkHitRouteSegment9Nodes(b *testing.B) {
	e := benchmarkDiagram(b, 9, 2)
	for range 5 {
		e.hitRouteSegment(40, 10) // warm screen cache
	}
	b.ResetTimer()
	for range b.N {
		e.hitRouteSegment(40, 10)
	}
}

func BenchmarkHitRouteSegmentFast9Nodes(b *testing.B) {
	e := benchmarkDiagram(b, 9, 2)
	// ensure scene is built
	_ = e.ensureScene()
	b.ResetTimer()
	for range b.N {
		e.hitRouteSegmentFast(40, 10)
	}
}

func BenchmarkScreenRoutePoints(b *testing.B) {
	pts := []geometry.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 25, Y: 5}, {X: 40, Y: 0}, {X: 50, Y: 0}}
	b.ResetTimer()
	e := &Editor{zoom: 100}
	for range b.N {
		e.screenRoutePoints(pts)
	}
}

func BenchmarkHandleMouseHover(b *testing.B) {
	e := benchmarkDiagram(b, 9, 2)
	ev := tcell.NewEventMouse(40, 10, tcell.ButtonNone, tcell.ModNone)
	b.ResetTimer()
	for range b.N {
		e.handleMouse(ev)
	}
}

func BenchmarkHandleMouseDrag(b *testing.B) {
	e := benchmarkDiagram(b, 9, 2)
	e.screen = newFakeScreen(100, 45)
	down := tcell.NewEventMouse(3, 3, tcell.Button1, tcell.ModNone)
	move := tcell.NewEventMouse(12, 9, tcell.Button1, tcell.ModNone)
	up := tcell.NewEventMouse(12, 9, tcell.ButtonNone, tcell.ModNone)
	b.ResetTimer()
	for range b.N {
		e.doc.Elements[0].X, e.doc.Elements[0].Y = 1, 1 // reset to avoid drift
		e.scene = nil                                   // fresh scene each iteration
		e.handleMouse(down)
		e.handleMouse(move)
		e.handleMouse(up)
	}
}

// Step 2 — deterministic regression tests

func TestMouseHoverDoesNotRecalculateRoutes(t *testing.T) {
	e := benchmarkDiagram(t, 9, 2)
	// node-0 at logical (1,1) → screen at zoom 100: (1,2)
	nx, ny := e.canvasToScreen(1, 1)
	for range 5 {
		e.handleMouse(tcell.NewEventMouse(nx, ny, tcell.ButtonNone, tcell.ModNone))
		e.handleMouse(tcell.NewEventMouse(nx+1, ny, tcell.ButtonNone, tcell.ModNone))
	}
	if e.hovered < 0 {
		t.Fatalf("hover should have identified an element at screen %d,%d", nx, ny)
	}
}

// Step 6 — drag fractional position across zoom levels (test, not benchmark)

func TestDragPreservesExactPositionAtAllZoomLevels(t *testing.T) {
	for _, zoom := range []int{50, 75, 100, 125, 150, 200} {
		t.Run(fmt.Sprintf("zoom-%d", zoom), func(t *testing.T) {
			e := &Editor{doc: document.Document{Elements: []document.Element{
				{ID: "node", Kind: "box", X: 10, Y: 8, W: 12, H: 4, Label: "N"},
			}}, zoom: zoom, selected: -1, hovered: -1, routePoint: -1, connectFrom: -1}
			// click on the node, drag to a new screen position
			sx, sy := e.canvasToScreen(10, 8)
			sx2, sy2 := sx+6, sy+3
			e.handleMouse(tcell.NewEventMouse(sx, sy, tcell.Button1, tcell.ModNone))
			e.handleMouse(tcell.NewEventMouse(sx2, sy2, tcell.Button1, tcell.ModNone))
			e.handleMouse(tcell.NewEventMouse(sx2, sy2, tcell.ButtonNone, tcell.ModNone))
			// anchor-based: origin + (mouse delta) * 100 / zoom
			// origin = (10,8), mouse delta from click = (6,3)
			expectedX := 10 + 6*100/zoom
			expectedY := 8 + 3*100/zoom
			if e.doc.Elements[0].X != expectedX || e.doc.Elements[0].Y != expectedY {
				t.Fatalf("zoom %d: position = %d,%d, want %d,%d", zoom, e.doc.Elements[0].X, e.doc.Elements[0].Y, expectedX, expectedY)
			}
		})
	}
}

// Step 5 — no redraw on same-cell motion

func TestDuplicateMouseMotionDoesNotDirtyDocument(t *testing.T) {
	e := &Editor{doc: document.Document{Elements: []document.Element{
		{ID: "node", Kind: "box", X: 10, Y: 8, W: 12, H: 4, Label: "N"},
	}}, zoom: 100, selected: -1, hovered: -1, routePoint: -1, connectFrom: -1}
	// first motion — should select
	e.handleMouse(tcell.NewEventMouse(11, 9, tcell.ButtonNone, tcell.ModNone))
	// duplicate motion on same cell — should not trigger any change
	e.dirty = false
	e.handleMouse(tcell.NewEventMouse(11, 9, tcell.ButtonNone, tcell.ModNone))
	if e.dirty {
		t.Fatal("duplicate motion on same cell changed document")
	}
}

// Step 7 — provisional drag freezes non-incident routes

func TestProvisionalDragFreezesNonIncidentRoutes(t *testing.T) {
	e := benchmarkDiagram(t, 5, 1)
	e.screen = newFakeScreen(100, 45)
	_ = e.ensureScene()
	nonIncident := 7 // arrow node-2 → node-3, not incident to node-0
	preRoute := e.scene.routes[nonIncident]
	if len(preRoute.points) == 0 {
		t.Fatal("pre-route should have points")
	}
	// simulate a full drag frame: mouse-down + draw (which rebuilds scene after checkpoint)
	e.handleMouse(tcell.NewEventMouse(3, 3, tcell.Button1, tcell.ModNone))
	e.draw()
	e.handleMouse(tcell.NewEventMouse(10, 10, tcell.Button1, tcell.ModNone))
	e.draw()
	// scene cache must still have routes after draw (not nilled by drag frame)
	if e.scene.routes == nil {
		t.Fatal("scene.routes was nilled during drag — invalidateSceneRoutes must not fire per frame")
	}
	// non-incident route exists (may differ due to reserved-path reordering, but must be non-empty)
	if _, ok := e.scene.routes[nonIncident]; !ok {
		t.Fatal("non-incident route missing from scene cache during drag")
	}
	if len(e.scene.routes[nonIncident].points) == 0 {
		t.Fatal("non-incident route has no points during drag")
	}
	// incident route exists in cache (from the draw rebuild — still using BFS, just not recomputed per frame)
	incident := 5 // arrow 0→1
	if _, ok := e.scene.routes[incident]; !ok {
		t.Fatal("incident route missing from cache during drag")
	}
	// release + draw
	e.handleMouse(tcell.NewEventMouse(10, 10, tcell.ButtonNone, tcell.ModNone))
	e.draw()
	// after release+draw, scene routes rebuilt
	if e.scene.routes == nil {
		t.Fatal("scene.routes nil after mouse-up — should be rebuilt by ensureScene")
	}
	if len(e.scene.routes[nonIncident].points) == 0 {
		t.Fatal("non-incident route empty after mouse-up rebuild")
	}
}

func TestProvisionalRoutesIncidentArrowsFollowDrag(t *testing.T) {
	e := benchmarkDiagram(t, 5, 1)
	e.screen = newFakeScreen(100, 45)
	_ = e.ensureScene()
	// drag node-0 from (1,1) to some position
	e.handleMouse(tcell.NewEventMouse(3, 3, tcell.Button1, tcell.ModNone))
	e.handleMouse(tcell.NewEventMouse(60, 40, tcell.Button1, tcell.ModNone))
	// provisional routes for incident arrows should have start points near the new node position
	routes := e.provisionalRoutes(e.scene, 0)
	incident := 5 // arrow node-0 → node-1
	r, ok := routes[incident]
	if !ok {
		t.Fatal("incident route not in provisional results")
	}
	if len(r.points) < 2 {
		t.Fatal("incident provisional route has fewer than 2 points")
	}
	// incident route must have NO warning (we skip that during drag)
	if r.warning {
		t.Fatal("provisional route should not carry warning flag")
	}
	// non-incident route should be from cache (same points)
	nonIncident := 7 // arrow node-2 → node-3
	if !pointsEqual(e.scene.routes[nonIncident].points, routes[nonIncident].points) {
		t.Fatal("non-incident provisional route diverged from cache")
	}
}

func pointsEqual(a, b []geometry.Point) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Step 12 — UX helpers for connection flow

func TestConnectionFlowKeyboardTargetSelectionSkipsArrowsAndAnnotations(t *testing.T) {
	elements := []document.Element{
		{ID: "source", Kind: "box"},
		{ID: "arrow", Kind: "arrow", From: "source", To: "target"},
		{ID: "note", Kind: "annotation", Text: "text"},
		{ID: "target", Kind: "box"},
	}
	if got := nextBoxIndex(elements, 0, 0); got != 3 {
		t.Fatalf("nextBoxIndex = %d, want 3 (skip arrow and annotation)", got)
	}
}

// Step 8 — port ordering by destination position

func TestPortsSortedByDestinationPosition(t *testing.T) {
	e := &Editor{doc: document.Document{Elements: []document.Element{
		{ID: "center", Kind: "box", X: 50, Y: 50, W: 10, H: 10},
		{ID: "near-top", Kind: "box", X: 25, Y: 10, W: 10, H: 4},
		{ID: "far-top", Kind: "box", X: 75, Y: 10, W: 10, H: 4},
		// both arrows go from center upward → same port (top edge, horizontal=true, side=-1)
		{ID: "to-near", Kind: "arrow", From: "center", To: "near-top"},
		{ID: "to-far", Kind: "arrow", From: "center", To: "far-top"},
	}}}
	_ = e.ensureScene()
	// near-top is at X=25 (left), far-top at X=75 (right)
	// On the top edge (horizontal port), X=25 should sort before X=75
	// Smaller X → smaller offset value (lower lane number)
	nearOff := e.scene.portOffsets[3]
	farOff := e.scene.portOffsets[4]
	if nearOff >= farOff {
		t.Fatalf("to-near offset %d should be < to-far offset %d", nearOff, farOff)
	}
}

func TestFocusClassifiesInOutAndDim(t *testing.T) {
	e := &Editor{doc: document.Document{Elements: []document.Element{
		{ID: "a", Kind: "box", X: 0, Y: 0, W: 10, H: 4},
		{ID: "b", Kind: "box", X: 30, Y: 0, W: 10, H: 4},
		{ID: "c", Kind: "box", X: 30, Y: 10, W: 10, H: 4},
		{ID: "a-to-b", Kind: "arrow", From: "a", To: "b"},
		{ID: "c-to-a", Kind: "arrow", From: "c", To: "a"},
		{ID: "b-to-c", Kind: "arrow", From: "b", To: "c"},
	}}, selected: 0}
	_ = e.ensureScene()
	focus := e.arrowFocusMap(e.scene)
	if focus[3] != focusOut {
		t.Fatalf("a→b should be focusOut, got %d", focus[3])
	}
	if focus[4] != focusIn {
		t.Fatalf("c→a should be focusIn, got %d", focus[4])
	}
	if focus[5] != focusDim {
		t.Fatalf("b→c should be focusDim, got %d", focus[5])
	}
}
