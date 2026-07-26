package editor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"circinus/internal/document"
	"circinus/internal/geometry"

	"github.com/gdamore/tcell/v2"
)

type arrowFocus int

const (
	focusOut arrowFocus = iota // arrow FROM selected node
	focusIn                    // arrow TO selected node
	focusDim                   // arrow not incident to selection
)

func (e *Editor) draw() {
	e.screen.Clear()
	w, h := e.screen.Size()
	titleStyle := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorLightGreen).Bold(true)
	statusStyle := tcell.StyleDefault.Foreground(tcell.ColorLightGreen).Background(tcell.ColorBlack)
	e.screen.Fill(' ', tcell.StyleDefault.Background(tcell.ColorBlack))

	sc := e.ensureScene()
	routes := sc.routes
	if e.dragging && e.dragKind == "element" && e.dragElement >= 0 {
		routes = e.provisionalRoutes(sc, e.dragElement)
	}
	// build focus map: per-arrow classification relative to selection
	focus := e.arrowFocusMap(sc)
	// draw dim arrows first (background), then incoming, then outgoing (on top)
	for _, pass := range []arrowFocus{focusDim, focusIn, focusOut} {
		for i, element := range e.doc.Elements {
			if element.Kind != "arrow" {
				continue
			}
			if focus[i] != pass {
				continue
			}
			isSel := i == e.selected || i == e.hovered
			e.drawArrowPath(element, isSel, routes[i], focus[i])
		}
	}
	// labels follow same pass order but all draws happen after all paths
	for _, pass := range []arrowFocus{focusDim, focusIn, focusOut} {
		for i, element := range e.doc.Elements {
			if element.Kind != "arrow" {
				continue
			}
			if focus[i] != pass {
				continue
			}
			isSel := i == e.selected || i == e.hovered
			e.drawArrowLabel(element, isSel, routes[i], focus[i])
		}
	}
	for i, element := range e.doc.Elements {
		switch element.Kind {
		case "box":
			e.drawBox(element, i == e.selected || i == e.hovered)
		case "annotation":
			e.drawAnnotation(element, i == e.selected || i == e.hovered)
		}
	}
	e.drawConnectionPreview()

	e.screen.PutStrStyled(0, 0, fmt.Sprintf(" Circinus  %s ", filepath.Base(e.path)), titleStyle)
	e.drawDetails(w, h)
	e.screen.PutStrStyled(0, h-2, fit(" "+e.status, w), statusStyle)
	modeBar := ""
	if e.connectMode {
		tgtID := "?"
		if e.selected >= 0 && e.selected < len(e.doc.Elements) && e.selected != e.connectFrom && e.doc.Elements[e.selected].Kind == "box" {
			tgtID = e.doc.Elements[e.selected].ID
		}
		srcID := "?"
		if e.connectFrom >= 0 && e.connectFrom < len(e.doc.Elements) {
			srcID = e.doc.Elements[e.connectFrom].ID
		}
		modeBar = fmt.Sprintf(" CONNECT: %s → %s | arrows/Tab: target  Enter: confirm  Esc: cancel ", srcID, tgtID)
	} else if e.routeMode {
		modeBar = " ROUTE: Tab: point  arrows/hjkl: move  Enter: add  Delete: remove  r: reset  Esc: done "
	} else {
		modeBar = " b node  n annotation  a relation  m route  x delete  Tab select  e edit  r ref  h/j/k/l move  +/- zoom  0 fit  u undo  Ctrl-r redo  Ctrl-s save  q quit "
	}
	e.screen.PutStrStyled(0, h-1, fit(modeBar, w), statusStyle)
	e.screen.Show()
}

func (e *Editor) drawDetails(width, height int) {
	style := tcell.StyleDefault.Foreground(tcell.ColorLightCyan).Background(tcell.ColorBlack)
	start := height - 6
	if start < 1 {
		return
	}
	for y := start; y < height-2; y++ {
		e.screen.PutStrStyled(0, y, strings.Repeat(" ", width), style)
	}
	if e.selected < 0 || e.selected >= len(e.doc.Elements) {
		return
	}
	element := e.doc.Elements[e.selected]
	lines := []string{}
	if element.Kind == "box" {
		out, in := e.incidentCounts(element.ID)
		lines = []string{
			fmt.Sprintf(" %s  ▲%d  ▼%d", element.ID, out, in),
			" " + e.relationList(element.ID, true, width-1),
			" " + e.relationList(element.ID, false, width-1),
			fmt.Sprintf(" label: %s  ref: %s", displayValue(element.Label), formatReference(element.Ref)),
		}
	} else if element.Kind == "annotation" {
		lines = []string{
			fmt.Sprintf(" annotation %s", element.ID),
			fmt.Sprintf(" text: %s", displayValue(strings.ReplaceAll(element.Text, "\n", " "))),
			fmt.Sprintf(" size: %dx%d", element.W, element.H),
		}
	} else {
		lines = []string{
			fmt.Sprintf(" relation %s", element.ID),
			fmt.Sprintf(" %s → %s  label: %s", element.From, element.To, displayValue(element.Label)),
			fmt.Sprintf(" ref: %s", formatReference(element.Ref)),
		}
	}
	for i, line := range lines {
		if start+i < height-2 {
			e.screen.PutStrStyled(0, start+i, fit(line, width), style)
		}
	}
}

func displayValue(value string) string {
	if value == "" {
		return "\u2014"
	}
	return value
}

func formatReference(ref *document.Reference) string {
	if ref == nil {
		return "\u2014"
	}
	value := ref.Path
	if ref.Line != nil {
		value += ":" + strconv.Itoa(*ref.Line)
	}
	if ref.Symbol != "" {
		value += " (" + ref.Symbol + ")"
	}
	return value
}

func fit(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	return string(runes[:geometry.Max(0, width-1)]) + "\u2026"
}

type nodeShapeRenderer func(*Editor, document.Element, int, int, tcell.Style)

var nodeShapeRenderers = map[document.Shape]nodeShapeRenderer{
	document.ShapeBox:      drawRectNode,
	document.ShapeDatabase: drawDatabaseNode,
	document.ShapeGateway:  drawGatewayNode,
	document.ShapeQueue:    drawQueueNode,
}

func (e *Editor) drawBox(element document.Element, selected bool) {
	x, y := e.canvasToScreen(element.X, element.Y)
	if e.zoom < 50 {
		style := tcell.StyleDefault.Foreground(tcell.ColorLightGreen)
		if selected {
			style = style.Foreground(tcell.ColorYellow).Bold(true)
		}
		e.screen.SetContent(x, y, '□', nil, style)
		if element.Label != "" {
			e.screen.SetContent(x+1, y, []rune(element.Label)[0], nil, style)
		}
		return
	}
	element.W = e.scaledSize(element.W)
	element.H = e.scaledSize(element.H)
	style := tcell.StyleDefault.Foreground(tcell.ColorLightGreen)
	if selected {
		style = style.Foreground(tcell.ColorYellow).Bold(true)
	}
	shape := document.EffectiveShape(element)
	definition, ok := document.ShapeDefinitionFor(shape)
	if !ok {
		definition, _ = document.ShapeDefinitionFor(document.ShapeBox)
		shape = document.ShapeBox
	}
	renderer, ok := nodeShapeRenderers[shape]
	if !ok {
		renderer = drawRectNode
	}
	renderer(e, element, x, y, style)
	if element.Label != "" {
		inset := definition.LabelInset
		labelWidth := element.W - inset*2
		if inset < 1 {
			inset = 1
			labelWidth = element.W - 2
		}
		labelRow := element.H / 2
		if definition.LabelRow >= 0 && definition.LabelRow < element.H {
			labelRow = definition.LabelRow
		}
		if labelWidth > 0 {
			e.screen.PutStrStyled(x+inset, y+labelRow, truncate(element.Label, labelWidth), style)
		}
	}
	if element.W > 2 {
		marker := rune(0)
		if e.nodeOverlaps(element) {
			marker = '\u00D7'
		}
		if e.referenceBroken(element) {
			marker = '!'
		}
		if marker != 0 {
			e.screen.SetContent(x+element.W-2, y, marker, nil, tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true))
		}
	}
}

func drawRectNode(e *Editor, element document.Element, x, y int, style tcell.Style) {
	put := func(px, py int, r rune) { e.screen.SetContent(px, py, r, nil, style) }
	put(x, y, '\u256D')
	put(x+element.W-1, y, '\u256E')
	put(x, y+element.H-1, '\u2570')
	put(x+element.W-1, y+element.H-1, '\u256F')
	for i := 1; i < element.W-1; i++ {
		put(x+i, y, '\u2500')
		put(x+i, y+element.H-1, '\u2500')
	}
	for i := 1; i < element.H-1; i++ {
		put(x, y+i, '\u2502')
		put(x+element.W-1, y+i, '\u2502')
	}
}

func drawDatabaseNode(e *Editor, element document.Element, x, y int, style tcell.Style) {
	for row, line := range databaseLines(element.W, element.H) {
		e.screen.PutStrStyled(x, y+row, line, style)
	}
}

func databaseLines(width, height int) []string {
	if width < 6 || height < 4 {
		return nil
	}
	horizontal := strings.Repeat("-", width-3)
	body := strings.Repeat(" ", width-3)
	separator := "|" + strings.Repeat("-", width-2) + "|"
	rows := []string{
		" ." + horizontal + ".",
		"/" + strings.Repeat(" ", width-2) + "\\",
	}
	for i := 0; i < height-4; i++ {
		if i%2 == 0 {
			rows = append(rows, separator)
		} else {
			rows = append(rows, "|"+body+"o|")
		}
	}
	rows = append(rows,
		"\\"+strings.Repeat(" ", width-2)+"/",
		" '"+horizontal+"'",
	)
	return rows
}

func drawGatewayNode(e *Editor, element document.Element, x, y int, style tcell.Style) {
	for row, line := range gatewayLines(element.W, element.H) {
		e.screen.PutStrStyled(x, y+row, line, style)
	}
}

func gatewayLines(width, height int) []string {
	if width < 6 || height < 3 {
		return nil
	}
	horizontal := strings.Repeat("─", width-3)
	rows := []string{" ╱" + horizontal + "╲"}
	for i := 0; i < height-2; i++ {
		rows = append(rows, "│"+strings.Repeat(" ", width-2)+"│")
	}
	return append(rows, " ╲"+horizontal+"╱")
}

func drawQueueNode(e *Editor, element document.Element, x, y int, style tcell.Style) {
	if element.W < 2 || element.H < 1 {
		return
	}
	rows := []string{
		"╭" + strings.Repeat("─", element.W-2) + "╮",
		"│" + strings.Repeat(" ", element.W-2) + "│",
		"├" + strings.Repeat("─", element.W-2) + "┤",
		"│" + strings.Repeat("·", element.W-2) + "│",
		"╰" + strings.Repeat("─", element.W-2) + "╯",
	}
	for row, line := range rows {
		if row < element.H {
			e.screen.PutStrStyled(x, y+row, line, style)
		}
	}
}

type relationRoute struct {
	points  []geometry.Point
	warning bool
}

type relationPort struct {
	nodeID     string
	horizontal bool
	side       int
}

func relationPortFor(node, other document.Element) relationPort {
	nodeCenterX := node.X + node.W/2
	nodeCenterY := node.Y + node.H/2
	otherCenterX := other.X + other.W/2
	otherCenterY := other.Y + other.H/2
	if geometry.Abs(otherCenterX-nodeCenterX) >= geometry.Abs(otherCenterY-nodeCenterY) {
		side := -1
		if otherCenterX >= nodeCenterX {
			side = 1
		}
		return relationPort{nodeID: node.ID, horizontal: true, side: side}
	}
	side := -1
	if otherCenterY >= nodeCenterY {
		side = 1
	}
	return relationPort{nodeID: node.ID, horizontal: false, side: side}
}

func relationPairLane(elements []document.Element, index int) int {
	if index < 0 || index >= len(elements) || elements[index].Kind != "arrow" {
		return 0
	}
	current := elements[index]
	left, right := current.From, current.To
	if right < left {
		left, right = right, left
	}
	ordinal, count := 0, 0
	for i, element := range elements {
		if element.Kind != "arrow" {
			continue
		}
		from, to := element.From, element.To
		if to < from {
			from, to = to, from
		}
		if from == left && to == right {
			if i < index {
				ordinal++
			}
			count++
		}
	}
	return ordinal*2 - (count - 1)
}

func relationPorts(from, to document.Element) (relationPort, relationPort) {
	return relationPortFor(from, to), relationPortFor(to, from)
}

func (e *Editor) nodePortOffset(targetIndex int, target relationPort) int {
	ordinal, count := 0, 0
	for i, element := range e.doc.Elements {
		if element.Kind != "arrow" {
			continue
		}
		from, to, err := e.doc.RelationNodes(element)
		if err != nil {
			continue
		}
		ports := [...]relationPort{relationPortFor(from, to), relationPortFor(to, from)}
		for _, port := range ports {
			if port.nodeID != target.nodeID || port.horizontal != target.horizontal || port.side != target.side {
				continue
			}
			if i < targetIndex {
				ordinal++
			}
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return ordinal*2 - (count - 1)
}

func (e *Editor) relationEdges(index int, from, to document.Element) (geometry.Point, geometry.Point) {
	fromBox := geometry.Box{X: from.X, Y: from.Y, W: from.W, H: from.H}
	toBox := geometry.Box{X: to.X, Y: to.Y, W: to.W, H: to.H}
	fromCenter := geometry.Point{X: from.X + from.W/2, Y: from.Y + from.H/2}
	toCenter := geometry.Point{X: to.X + to.W/2, Y: to.Y + to.H/2}
	fromOffset, toOffset := 0, 0
	if e.scene != nil && e.scene.portOffsets != nil {
		fromOffset = e.scene.portOffsets[index]
		toOffset = e.scene.portOffsets[index]
	} else {
		fromPort, toPort := relationPorts(from, to)
		pairOffset := relationPairLane(e.doc.Elements, index)
		fromOffset = pairOffset + e.nodePortOffset(index, fromPort)
		toOffset = pairOffset + e.nodePortOffset(index, toPort)
	}
	start, end := geometry.ArrowEdgesWithOffsets(fromBox, toBox, fromCenter, toCenter, fromOffset, toOffset)
	return start, end
}

func (e *Editor) drawAnnotation(element document.Element, selected bool) {
	x, y := e.canvasToScreen(element.X, element.Y)
	if e.zoom < 50 {
		style := tcell.StyleDefault.Foreground(tcell.ColorLightCyan)
		if selected {
			style = style.Foreground(tcell.ColorYellow).Bold(true)
		}
		e.screen.SetContent(x, y, '▱', nil, style)
		return
	}
	width, height := e.scaledSize(element.W), e.scaledSize(element.H)
	style := tcell.StyleDefault.Foreground(tcell.ColorLightCyan)
	if selected {
		style = style.Foreground(tcell.ColorYellow).Bold(true)
	}
	for row := 0; row < height; row++ {
		e.screen.PutStrStyled(x, y+row, strings.Repeat(" ", width), style)
	}
	lines := strings.Split(element.Text, "\n")
	for row, line := range lines {
		if row >= height {
			break
		}
		e.screen.PutStrStyled(x, y+row, truncate(line, width), style)
	}
}

func (e *Editor) planRelationRoutes() map[int]relationRoute {
	routes := make(map[int]relationRoute)
	reserved := []geometry.Point{}
	for i, element := range e.doc.Elements {
		if element.Kind != "arrow" {
			continue
		}
		from, to, err := e.doc.RelationNodes(element)
		if err != nil {
			continue
		}
		start, end := e.relationEdges(i, from, to)
		obstacles := routeBoxes(element, e.doc.Elements)
		points := routeThroughControlPoints(start, end, element.ControlPoints, obstacles, reserved)
		warning := geometry.PathHitsObstacle(points, obstacles) || geometry.PathHitsPoints(points, reserved)
		routes[i] = relationRoute{points: points, warning: warning}
		reserved = append(reserved, points...)
	}
	return routes
}

// ponytail: cheap orthogonal routes for incident arrows during drag;
// full BFS obstacle avoidance deferred to mouse-up.
func (e *Editor) provisionalRoutes(sc *sceneCache, dragIdx int) map[int]relationRoute {
	dragID := e.doc.Elements[dragIdx].ID
	routes := make(map[int]relationRoute, len(sc.routes))
	for i, cached := range sc.routes {
		el := e.doc.Elements[i]
		if el.Kind != "arrow" {
			continue
		}
		if el.From == dragID || el.To == dragID {
			from, to, err := e.doc.RelationNodes(el)
			if err != nil {
				routes[i] = cached
				continue
			}
			start, end := e.relationEdges(i, from, to)
			routes[i] = relationRoute{points: geometry.RoutePoints(start, end)}
			continue
		}
		routes[i] = cached
	}
	return routes
}

func routeBoxes(relation document.Element, elements []document.Element) []geometry.Box {
	boxes := make([]geometry.Box, 0)
	for _, element := range elements {
		if (element.Kind == "box" || element.Kind == "annotation") && element.ID != relation.From && element.ID != relation.To {
			boxes = append(boxes, geometry.Box{X: element.X, Y: element.Y, W: element.W, H: element.H})
		}
	}
	return boxes
}

func (e *Editor) drawArrowPath(element document.Element, selected bool, route relationRoute, focus arrowFocus) {
	if len(route.points) == 0 {
		return
	}
	points := e.screenRoutePoints(route.points)
	style := e.arrowStyle(element, selected, route.warning, focus)
	for i, current := range points {
		glyph := '\u00B7'
		if i > 0 && points[i-1].X == current.X {
			glyph = '\u2502'
		}
		if i > 0 && points[i-1].Y == current.Y {
			glyph = '\u2500'
		}
		e.screen.SetContent(current.X, current.Y, glyph, nil, style)
	}
	// ponytail: ● at source for outgoing arrows — direction readable without relying on color alone
	if focus == focusOut && len(points) >= 2 {
		e.screen.SetContent(points[0].X, points[0].Y, '\u25CF', nil, style)
	}
	if len(points) > 1 {
		previous, current := points[len(points)-2], points[len(points)-1]
		arrow := '\u25B6'
		if current.X < previous.X {
			arrow = '\u25C0'
		}
		if current.Y > previous.Y {
			arrow = '\u25BC'
		}
		if current.Y < previous.Y {
			arrow = '\u25B2'
		}
		e.screen.SetContent(current.X, current.Y, arrow, nil, style)
	}
	if selected && len(element.ControlPoints) > 0 {
		for pointIndex, point := range element.ControlPoints {
			control := e.screenRoutePoints([]geometry.Point{{X: point.X, Y: point.Y}})[0]
			glyph := '◇'
			if pointIndex == e.routePoint && e.routeMode {
				glyph = '◆'
			}
			e.screen.SetContent(control.X, control.Y, glyph, nil, tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true))
		}
	}
	if route.warning && len(points) > 0 {
		alert := points[len(points)/2]
		e.screen.SetContent(alert.X, alert.Y, '!', nil, tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true))
	}
}

func (e *Editor) drawArrowLabel(element document.Element, selected bool, route relationRoute, focus arrowFocus) {
	if len(route.points) == 0 || element.Label == "" || (e.zoom < 50 && !selected) {
		return
	}
	points := e.screenRoutePoints(route.points)
	placement := geometry.PlaceLabel(points, element.Label)
	if placement.Width > 0 {
		e.screen.PutStrStyled(placement.X, placement.Y, truncate(element.Label, placement.Width), e.arrowStyle(element, selected, route.warning, focus))
	}
}

func (e *Editor) screenRoutePoints(points []geometry.Point) []geometry.Point {
	if len(points) == 0 {
		return nil
	}
	result := make([]geometry.Point, 0, len(points))
	appendPoint := func(point geometry.Point) {
		if len(result) == 0 || result[len(result)-1] != point {
			result = append(result, point)
		}
	}
	for i, point := range points {
		x, y := e.canvasToScreen(point.X, point.Y)
		if i == 0 {
			appendPoint(geometry.Point{X: x, Y: y})
			continue
		}
		previous := result[len(result)-1]
		steps := geometry.Max(geometry.Abs(x-previous.X), geometry.Abs(y-previous.Y))
		for step := 1; step <= steps; step++ {
			appendPoint(geometry.Point{X: previous.X + (x-previous.X)*step/steps, Y: previous.Y + (y-previous.Y)*step/steps})
		}
	}
	return result
}

func (e *Editor) drawConnectionPreview() {
	if e.connectFrom < 0 || e.connectFrom >= len(e.doc.Elements) || e.doc.Elements[e.connectFrom].Kind != "box" {
		return
	}
	src := e.doc.Elements[e.connectFrom]
	style := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	srcEdgeX, srcEdgeY := e.canvasToScreen(src.X+src.W, src.Y+src.H/2)
	e.screen.SetContent(srcEdgeX, srcEdgeY, '●', nil, style)

	if e.connectMode && e.selected >= 0 && e.selected < len(e.doc.Elements) && e.doc.Elements[e.selected].Kind == "box" && e.selected != e.connectFrom {
		tgt := e.doc.Elements[e.selected]
		startEdge, endEdge := e.previewEdges(src, tgt)
		pts := geometry.RoutePoints(startEdge, endEdge)
		for _, pt := range pts {
			scrX, scrY := e.canvasToScreen(pt.X, pt.Y)
			e.screen.SetContent(scrX, scrY, '·', nil, style)
		}
		tgtEdgeX, tgtEdgeY := e.canvasToScreen(tgt.X-1, tgt.Y+tgt.H/2)
		if endEdge.X < tgt.X+tgt.W/2 {
			tgtEdgeX, tgtEdgeY = e.canvasToScreen(tgt.X+tgt.W, tgt.Y+tgt.H/2)
		} else if endEdge.Y > tgt.Y+tgt.H/2 {
			tgtEdgeX, tgtEdgeY = e.canvasToScreen(tgt.X+tgt.W/2, tgt.Y-1)
		} else if endEdge.Y < tgt.Y+tgt.H/2 {
			tgtEdgeX, tgtEdgeY = e.canvasToScreen(tgt.X+tgt.W/2, tgt.Y+tgt.H)
		}
		e.screen.SetContent(tgtEdgeX, tgtEdgeY, '●', nil, style)
	}
	if !e.connectMode {
		for i, el := range e.doc.Elements {
			if el.Kind != "box" {
				continue
			}
			if i != e.hovered && i != e.selected {
				continue
			}
			if e.zoom < 50 {
				continue
			}
			nx, ny := e.canvasToScreen(el.X, el.Y)
			w, h := e.scaledSize(el.W), e.scaledSize(el.H)
			handleStyle := tcell.StyleDefault.Foreground(tcell.ColorLightCyan)
			e.screen.SetContent(nx+w/2, ny-1, '╥', nil, handleStyle)
			e.screen.SetContent(nx+w/2, ny+h, '╨', nil, handleStyle)
			e.screen.SetContent(nx-1, ny+h/2, '╡', nil, handleStyle)
			e.screen.SetContent(nx+w, ny+h/2, '╞', nil, handleStyle)
		}
	}
}

func (e *Editor) previewEdges(src, tgt document.Element) (geometry.Point, geometry.Point) {
	srcBox := geometry.Box{X: src.X, Y: src.Y, W: src.W, H: src.H}
	tgtBox := geometry.Box{X: tgt.X, Y: tgt.Y, W: tgt.W, H: tgt.H}
	srcCenter := geometry.Point{X: src.X + src.W/2, Y: src.Y + src.H/2}
	tgtCenter := geometry.Point{X: tgt.X + tgt.W/2, Y: tgt.Y + tgt.H/2}
	return geometry.ArrowEdges(srcBox, tgtBox, srcCenter, tgtCenter)
}

func (e *Editor) arrowStyle(element document.Element, selected, warning bool, focus arrowFocus) tcell.Style {
	style := tcell.StyleDefault
	switch focus {
	case focusOut:
		style = style.Foreground(tcell.ColorLightGreen)
	case focusIn:
		style = style.Foreground(tcell.ColorLightCyan)
	default:
		style = style.Foreground(tcell.ColorDarkGray)
	}
	if e.referenceBroken(element) || warning {
		style = style.Foreground(tcell.ColorRed)
	}
	if selected {
		style = style.Foreground(tcell.ColorYellow).Bold(true)
	}
	return style
}

// arrowFocusMap classifies each arrow relative to the currently selected node.
// When an arrow is selected, its source and target become the focus anchors.
func (e *Editor) arrowFocusMap(sc *sceneCache) map[int]arrowFocus {
	focus := make(map[int]arrowFocus, len(e.doc.Elements))
	focusNode := ""
	if e.selected >= 0 && e.selected < len(e.doc.Elements) {
		el := e.doc.Elements[e.selected]
		if el.Kind == "box" {
			focusNode = el.ID
		} else if el.Kind == "arrow" {
			// when arrow is selected, highlight its endpoints too
			for i, el := range e.doc.Elements {
				if el.Kind != "arrow" {
					continue
				}
				if i == e.selected {
					focus[i] = focusOut
				} else if el.From == focusNode || el.To == focusNode {
					focus[i] = focusIn
				} else {
					focus[i] = focusDim
				}
			}
			return focus
		}
	}
	if focusNode == "" {
		for i, el := range e.doc.Elements {
			if el.Kind == "arrow" {
				focus[i] = focusOut // all look same when nothing selected
			}
		}
		return focus
	}
	for i, el := range e.doc.Elements {
		if el.Kind != "arrow" {
			continue
		}
		if el.From == focusNode {
			focus[i] = focusOut
		} else if el.To == focusNode {
			focus[i] = focusIn
		} else {
			focus[i] = focusDim
		}
	}
	return focus
}

func (e *Editor) referenceBroken(element document.Element) bool {
	return element.Ref != nil && document.ValidateReference(e.root, *element.Ref) != nil
}

func (e *Editor) incidentCounts(nodeID string) (out, in int) {
	for _, el := range e.doc.Elements {
		if el.Kind == "arrow" {
			if el.From == nodeID {
				out++
			} else if el.To == nodeID {
				in++
			}
		}
	}
	return
}

// ponytail: compact relation list in the detail panel shows full labels even when truncated on canvas.
func (e *Editor) relationList(nodeID string, outgoing bool, maxWidth int) string {
	prefix := "← "
	if outgoing {
		prefix = "→ "
	}
	parts := []string{}
	for _, el := range e.doc.Elements {
		if el.Kind != "arrow" {
			continue
		}
		var otherID string
		if outgoing && el.From == nodeID {
			otherID = el.To
		} else if !outgoing && el.To == nodeID {
			otherID = el.From
		} else {
			continue
		}
		label := el.Label
		if label == "" {
			label = "—"
		}
		parts = append(parts, fmt.Sprintf("%s(%s)", otherID, label))
	}
	if len(parts) == 0 {
		return prefix + "—"
	}
	return prefix + strings.Join(parts, " ")
}

func (e *Editor) nodeOverlaps(node document.Element) bool {
	for _, other := range e.doc.Elements {
		if other.Kind == "box" && other.ID != node.ID && boxesOverlap(node, other) {
			return true
		}
	}
	return false
}

func boxesOverlap(left, right document.Element) bool {
	return left.X < right.X+right.W && left.X+left.W > right.X && left.Y < right.Y+right.H && left.Y+left.H > right.Y
}
