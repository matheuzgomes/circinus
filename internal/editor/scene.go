package editor

import (
	"sort"

	"circinus/internal/document"
	"circinus/internal/geometry"
)

type sceneCache struct {
	routes       map[int]relationRoute    // index → route
	screenPoints map[int][]geometry.Point // index → screen-space points
	hitMap       map[geometry.Point]int   // screen point → arrow element index
	obstacles    []geometry.Box           // all node + annotation boxes
	elementIdx   map[string]int           // element ID → index
	portOffsets  map[int]int              // arrow index → combined port offset
	dirty        bool
	lastZoom     int
	lastViewX    int
	lastViewY    int
}

func (e *Editor) ensureScene() *sceneCache {
	if e.scene == nil {
		e.scene = &sceneCache{}
	}
	sc := e.scene
	if sc.elementIdx == nil {
		sc.elementIdx = make(map[string]int)
		for i, el := range e.doc.Elements {
			sc.elementIdx[el.ID] = i
		}
	}
	if !sc.dirty && sc.routes != nil && sc.lastZoom == e.zoom && sc.lastViewX == e.viewportX && sc.lastViewY == e.viewportY {
		return sc
	}
	if sc.lastZoom != e.zoom || sc.lastViewX != e.viewportX || sc.lastViewY != e.viewportY {
		sc.screenPoints = nil
		sc.hitMap = nil
	}
	sc.lastZoom = e.zoom
	sc.lastViewX = e.viewportX
	sc.lastViewY = e.viewportY
	if sc.obstacles == nil {
		sc.obstacles = make([]geometry.Box, 0)
		for _, el := range e.doc.Elements {
			if el.Kind == "box" || el.Kind == "annotation" {
				sc.obstacles = append(sc.obstacles, geometry.Box{X: el.X, Y: el.Y, W: el.W, H: el.H})
			}
		}
	}
	if sc.portOffsets == nil {
		sc.portOffsets = e.computePortOffsets()
	}
	if sc.routes == nil {
		sc.routes = make(map[int]relationRoute)
		reserved := []geometry.Point{}
		for i, el := range e.doc.Elements {
			if el.Kind != "arrow" {
				continue
			}
			from, to, err := e.doc.RelationNodes(el)
			if err != nil {
				continue
			}
			start, end := e.relationEdges(i, from, to)
			obs := routeBoxesCached(el, sc)
			points := routeThroughControlPoints(start, end, el.ControlPoints, obs, reserved)
			warning := geometry.PathHitsObstacle(points, obs) || geometry.PathHitsPoints(points, reserved)
			sc.routes[i] = relationRoute{points: points, warning: warning}
			reserved = append(reserved, points...)
		}
	}
	if sc.screenPoints == nil {
		sc.screenPoints = make(map[int][]geometry.Point)
		for i, route := range sc.routes {
			sc.screenPoints[i] = e.routeToScreen(route.points)
		}
	}
	if sc.hitMap == nil {
		sc.hitMap = make(map[geometry.Point]int)
		for i, pts := range sc.screenPoints {
			for ptIdx, pt := range pts {
				if ptIdx == 0 || ptIdx == len(pts)-1 {
					continue // skip endpoints
				}
				if _, exists := sc.hitMap[pt]; !exists {
					sc.hitMap[pt] = i
				}
			}
		}
	}
	return sc
}

func (e *Editor) invalidateSceneRoutes() {
	if e.scene == nil {
		return
	}
	e.scene.routes = nil
	e.scene.screenPoints = nil
	e.scene.hitMap = nil
	e.scene.obstacles = nil
	e.scene.elementIdx = nil
	e.scene.portOffsets = nil
	e.scene.dirty = false
}

func (e *Editor) invalidateSceneScreen() {
	if e.scene == nil {
		return
	}
	e.scene.screenPoints = nil
	e.scene.hitMap = nil
}

func (e *Editor) routeToScreen(points []geometry.Point) []geometry.Point {
	if len(points) == 0 {
		return nil
	}
	result := make([]geometry.Point, 0, len(points)*2)
	appendPoint := func(p geometry.Point) {
		if len(result) == 0 || result[len(result)-1] != p {
			result = append(result, p)
		}
	}
	for i, pt := range points {
		x, y := e.canvasToScreen(pt.X, pt.Y)
		if i == 0 {
			appendPoint(geometry.Point{X: x, Y: y})
			continue
		}
		prev := result[len(result)-1]
		steps := geometry.Max(geometry.Abs(x-prev.X), geometry.Abs(y-prev.Y))
		for step := 1; step <= steps; step++ {
			appendPoint(geometry.Point{
				X: prev.X + (x-prev.X)*step/steps,
				Y: prev.Y + (y-prev.Y)*step/steps,
			})
		}
	}
	return result
}

func routeBoxesCached(rel document.Element, sc *sceneCache) []geometry.Box {
	boxes := make([]geometry.Box, 0, len(sc.obstacles))
	for _, obs := range sc.obstacles {
		// exclude source and target nodes
		if el, ok := sc.elementIdx[rel.From]; ok && boxMatches(elem(sc, el), obs) {
			continue
		}
		if el, ok := sc.elementIdx[rel.To]; ok && boxMatches(elem(sc, el), obs) {
			continue
		}
		boxes = append(boxes, obs)
	}
	return boxes
}

func elem(sc *sceneCache, idx int) geometry.Box {
	// caller guarantees idx is valid; this is internal
	return sc.obstacles[idx]
}

func boxMatches(el geometry.Box, obs geometry.Box) bool {
	return el.X == obs.X && el.Y == obs.Y && el.W == obs.W && el.H == obs.H
}

// computePortOffsets assigns lane offsets sorted by destination position,
// so arrows on the same side exit in spatial order instead of JSON index order.
// ponytail: single pass, no extra allocations beyond the result map.
func (e *Editor) computePortOffsets() map[int]int {
	type portKey struct {
		nodeID     string
		horizontal bool
		side       int
	}
	groups := map[portKey][]int{} // port → arrow indices
	for i, el := range e.doc.Elements {
		if el.Kind != "arrow" {
			continue
		}
		from, to, err := e.doc.RelationNodes(el)
		if err != nil {
			continue
		}
		fromPort := relationPortFor(from, to)
		toPort := relationPortFor(to, from)
		for _, p := range []struct {
			pk    portKey
			other document.Element
		}{{portKey{from.ID, fromPort.horizontal, fromPort.side}, to}, {portKey{to.ID, toPort.horizontal, toPort.side}, from}} {
			groups[p.pk] = append(groups[p.pk], i)
		}
	}
	offsets := make(map[int]int, len(e.doc.Elements))
	for pk, indices := range groups {
		// sort by perpendicular coordinate of the other endpoint
		sort.Slice(indices, func(a, b int) bool {
			ela, elb := e.doc.Elements[indices[a]], e.doc.Elements[indices[b]]
			otherA := otherEndpoint(ela, pk.nodeID)
			otherB := otherEndpoint(elb, pk.nodeID)
			nodeA, _ := e.doc.FindByID(otherA)
			nodeB, _ := e.doc.FindByID(otherB)
			posA, posB := 0, 0
			if pk.horizontal {
				posA, posB = nodeA.Y+nodeA.H/2, nodeB.Y+nodeB.H/2
			} else {
				posA, posB = nodeA.X+nodeA.W/2, nodeB.X+nodeB.W/2
			}
			if posA != posB {
				return posA < posB
			}
			return indices[a] < indices[b] // tie-break: document order
		})
		count := len(indices)
		for ord, idx := range indices {
			offsets[idx] += ord*2 - (count - 1)
		}
	}
	return offsets
}

func otherEndpoint(el document.Element, nodeID string) string {
	if el.From == nodeID {
		return el.To
	}
	return el.From
}
