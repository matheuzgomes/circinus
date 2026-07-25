package geometry

type Point struct {
	X int
	Y int
}

type Box struct {
	X int
	Y int
	W int
	H int
}

type LabelPlacement struct {
	X          int
	Y          int
	Width      int
	Horizontal bool
}

func Abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func SegmentPoints(start, end Point) []Point {
	points := []Point{start}
	x, y := start.X, start.Y
	for x != end.X {
		if end.X > x {
			x++
		} else {
			x--
		}
		points = append(points, Point{X: x, Y: y})
	}
	for y != end.Y {
		if end.Y > y {
			y++
		} else {
			y--
		}
		points = append(points, Point{X: x, Y: y})
	}
	return points
}

func RoutePoints(start, end Point) []Point {
	if start == end {
		return []Point{start}
	}
	if start.X == end.X || start.Y == end.Y {
		return SegmentPoints(start, end)
	}
	mid := Point{X: start.X + (end.X-start.X)/2, Y: start.Y}
	points := SegmentPoints(start, mid)
	points = append(points, SegmentPoints(mid, Point{X: mid.X, Y: end.Y})[1:]...)
	points = append(points, SegmentPoints(Point{X: mid.X, Y: end.Y}, end)[1:]...)
	return points
}

func ContainsPoint(points []Point, wanted Point) bool {
	for _, current := range points {
		if current == wanted {
			return true
		}
	}
	return false
}

func PointInBox(current Point, box Box, margin int) bool {
	return current.X >= box.X-margin && current.X <= box.X+box.W-1+margin &&
		current.Y >= box.Y-margin && current.Y <= box.Y+box.H-1+margin
}

func BlockedByBox(current Point, obstacles []Box) bool {
	for _, obstacle := range obstacles {
		if PointInBox(current, obstacle, 1) {
			return true
		}
	}
	return false
}

func PathHitsObstacle(path []Point, obstacles []Box) bool {
	for _, current := range path {
		if BlockedByBox(current, obstacles) {
			return true
		}
	}
	return false
}

func RouteAroundBoxes(start, end Point, obstacles []Box) []Point {
	return RouteAroundBoxesAndPoints(start, end, obstacles, nil)
}

func RouteAroundBoxesAndPoints(start, end Point, obstacles []Box, blocked []Point) []Point {
	if start == end {
		return []Point{start}
	}
	minX, maxX := Min(start.X, end.X), Max(start.X, end.X)
	minY, maxY := Min(start.Y, end.Y), Max(start.Y, end.Y)
	for _, obstacle := range obstacles {
		minX = Min(minX, obstacle.X-2)
		maxX = Max(maxX, obstacle.X+obstacle.W+1)
		minY = Min(minY, obstacle.Y-2)
		maxY = Max(maxY, obstacle.Y+obstacle.H+1)
	}
	for _, point := range blocked {
		minX = Min(minX, point.X-2)
		maxX = Max(maxX, point.X+2)
		minY = Min(minY, point.Y-2)
		maxY = Max(maxY, point.Y+2)
	}
	minX -= 8
	maxX += 8
	minY -= 8
	maxY += 8

	queue := []Point{start}
	previous := map[Point]Point{start: start}
	directions := [...]Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		if current == end {
			break
		}
		for _, direction := range directions {
			next := Point{X: current.X + direction.X, Y: current.Y + direction.Y}
			if next.X < minX || next.X > maxX || next.Y < minY || next.Y > maxY {
				continue
			}
			if _, seen := previous[next]; seen || (next != end && (BlockedByBox(next, obstacles) || ContainsPoint(blocked, next))) {
				continue
			}
			previous[next] = current
			queue = append(queue, next)
		}
	}
	if _, found := previous[end]; !found {
		return RoutePoints(start, end)
	}
	path := []Point{}
	for current := end; ; current = previous[current] {
		path = append(path, current)
		if current == start {
			break
		}
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return path
}

func PathHitsPoints(path, blocked []Point) bool {
	for _, point := range path {
		if ContainsPoint(blocked, point) {
			return true
		}
	}
	return false
}

func ArrowEdges(from, to Box, fromCenter, toCenter Point) (Point, Point) {
	return ArrowEdgesWithOffsets(from, to, fromCenter, toCenter, 0, 0)
}

func ArrowEdgesWithOffset(from, to Box, fromCenter, toCenter Point, offset int) (Point, Point) {
	return ArrowEdgesWithOffsets(from, to, fromCenter, toCenter, offset, offset)
}

func ArrowEdgesWithOffsets(from, to Box, fromCenter, toCenter Point, fromOffset, toOffset int) (Point, Point) {
	if Abs(toCenter.X-fromCenter.X) >= Abs(toCenter.Y-fromCenter.Y) {
		fromY := clamp(fromCenter.Y+fromOffset, from.Y, from.Y+from.H-1)
		toY := clamp(toCenter.Y+toOffset, to.Y, to.Y+to.H-1)
		if toCenter.X >= fromCenter.X {
			return Point{X: from.X + from.W, Y: fromY}, Point{X: to.X - 1, Y: toY}
		}
		return Point{X: from.X - 1, Y: fromY}, Point{X: to.X + to.W, Y: toY}
	}
	fromX := clamp(fromCenter.X+fromOffset, from.X, from.X+from.W-1)
	toX := clamp(toCenter.X+toOffset, to.X, to.X+to.W-1)
	if toCenter.Y >= fromCenter.Y {
		return Point{X: fromX, Y: from.Y + from.H}, Point{X: toX, Y: to.Y - 1}
	}
	return Point{X: fromX, Y: from.Y - 1}, Point{X: toX, Y: to.Y + to.H}
}

func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func PlaceLabel(points []Point, label string) LabelPlacement {
	if len(points) < 2 || label == "" {
		return LabelPlacement{}
	}
	type segment struct {
		start, end Point
		horizontal bool
		length     int
	}
	segments := make([]segment, 0, len(points)/2)
	for i := 0; i < len(points)-1; {
		start := i
		horizontal := points[i].Y == points[i+1].Y
		for i+1 < len(points)-1 {
			nextHorizontal := points[i+1].Y == points[i+2].Y
			if nextHorizontal != horizontal {
				break
			}
			i++
		}
		i++
		end := i
		segments = append(segments, segment{
			start: points[start], end: points[end], horizontal: horizontal,
			length: Abs(points[end].X-points[start].X) + Abs(points[end].Y-points[start].Y) + 1,
		})
	}
	best := segments[0]
	for _, candidate := range segments[1:] {
		if candidate.length > best.length || (candidate.length == best.length && candidate.horizontal && !best.horizontal) {
			best = candidate
		}
	}
	width := len([]rune(label))
	if width > best.length {
		width = best.length
	}
	if width < 1 {
		width = 1
	}
	if best.horizontal {
		labelY := best.start.Y - 1
		if labelY < 1 {
			labelY = best.start.Y + 1
		}
		return LabelPlacement{
			X:     best.start.X + (best.end.X-best.start.X-width+1)/2,
			Y:     labelY,
			Width: width, Horizontal: true,
		}
	}
	return LabelPlacement{
		X:     best.start.X + 1,
		Y:     best.start.Y + (best.end.Y-best.start.Y)/2,
		Width: width,
	}
}
