package geometry

import (
	"testing"
)

func TestRoutePointsIncludesBothSegments(t *testing.T) {
	points := RoutePoints(Point{X: 2, Y: 2}, Point{X: 8, Y: 6})
	if !ContainsPoint(points, Point{X: 5, Y: 2}) || !ContainsPoint(points, Point{X: 5, Y: 6}) {
		t.Fatalf("route did not connect horizontal and vertical segments: %#v", points)
	}
}

func TestRouteAroundReservedRelationPath(t *testing.T) {
	from := Box{X: 0, Y: 0, W: 10, H: 4}
	to := Box{X: 30, Y: 0, W: 10, H: 4}
	fromCenter := Point{X: 5, Y: 2}
	toCenter := Point{X: 35, Y: 2}
	forwardStart, forwardEnd := ArrowEdgesWithOffsets(from, to, fromCenter, toCenter, -2, -2)
	reverseStart, reverseEnd := ArrowEdgesWithOffsets(to, from, toCenter, fromCenter, 2, 2)
	forward := RouteAroundBoxes(forwardStart, forwardEnd, nil)
	reverse := RouteAroundBoxesAndPoints(reverseStart, reverseEnd, nil, forward)
	if PathHitsPoints(reverse, forward) {
		t.Fatalf("reserved relation path was reused: forward=%#v reverse=%#v", forward, reverse)
	}
}

func TestLabelPlacementUsesLongestStraightSegment(t *testing.T) {
	points := append(SegmentPoints(Point{X: 2, Y: 4}, Point{X: 14, Y: 4}), SegmentPoints(Point{X: 14, Y: 4}, Point{X: 14, Y: 6})[1:]...)
	placement := PlaceLabel(points, "GET /orders")
	if !placement.Horizontal || placement.Y != 3 {
		t.Fatalf("label placement = %#v, want above longest horizontal segment", placement)
	}
	if placement.Width > 13 || placement.Width < 1 {
		t.Fatalf("label width = %d, want it constrained to the segment", placement.Width)
	}
}

func TestRouteAvoidsIntermediateBox(t *testing.T) {
	obstacle := Box{X: 5, Y: 1, W: 4, H: 5}
	start, end := Point{X: 2, Y: 3}, Point{X: 12, Y: 3}
	path := RouteAroundBoxes(start, end, []Box{obstacle})
	detoured := false
	for _, current := range path {
		if current.Y != start.Y {
			detoured = true
		}
	}
	if !detoured {
		t.Fatalf("route did not detour around obstacle: %#v", path)
	}
	for _, current := range path {
		if PointInBox(current, obstacle, 0) {
			t.Fatalf("route entered obstacle at %#v: %#v", current, path)
		}
	}
}

func TestArrowEdgesHorizontal(t *testing.T) {
	from := Box{X: 0, Y: 0, W: 10, H: 4}
	to := Box{X: 20, Y: 0, W: 10, H: 4}
	fromCenter := Point{X: 5, Y: 2}
	toCenter := Point{X: 25, Y: 2}
	start, end := ArrowEdges(from, to, fromCenter, toCenter)
	if start.X != 10 || start.Y != 2 {
		t.Fatalf("start edge = %#v, want (10, 2)", start)
	}
	if end.X != 19 || end.Y != 2 {
		t.Fatalf("end edge = %#v, want (19, 2)", end)
	}
}

func TestArrowEdgesWithOffsetStayOnNodeBoundaries(t *testing.T) {
	from := Box{X: 0, Y: 0, W: 10, H: 4}
	to := Box{X: 20, Y: 0, W: 10, H: 4}
	fromCenter := Point{X: 5, Y: 2}
	toCenter := Point{X: 25, Y: 2}
	start, end := ArrowEdgesWithOffset(from, to, fromCenter, toCenter, 99)
	if start != (Point{X: 10, Y: 3}) {
		t.Fatalf("clamped start = %#v, want (10, 3)", start)
	}
	if end != (Point{X: 19, Y: 3}) {
		t.Fatalf("clamped end = %#v, want (19, 3)", end)
	}
	start, end = ArrowEdgesWithOffset(from, to, fromCenter, toCenter, -99)
	if start != (Point{X: 10, Y: 0}) || end != (Point{X: 19, Y: 0}) {
		t.Fatalf("negative offset edges = %#v, %#v", start, end)
	}
}

func TestBlockedByBoxDetectsCollision(t *testing.T) {
	box := Box{X: 5, Y: 5, W: 10, H: 4}
	if !BlockedByBox(Point{X: 7, Y: 6}, []Box{box}) {
		t.Fatal("point inside box should block")
	}
	if BlockedByBox(Point{X: 3, Y: 3}, []Box{box}) {
		t.Fatal("point outside box margin should not block")
	}
	if !BlockedByBox(Point{X: 4, Y: 4}, []Box{box}) {
		t.Fatal("point at margin edge should block")
	}
}

func TestSegmentPointsStraightLine(t *testing.T) {
	points := SegmentPoints(Point{X: 1, Y: 3}, Point{X: 5, Y: 3})
	if len(points) != 5 {
		t.Fatalf("got %d points, want 5: %#v", len(points), points)
	}
	if points[0] != (Point{X: 1, Y: 3}) || points[4] != (Point{X: 5, Y: 3}) {
		t.Fatal("segment did not include endpoints")
	}
}
