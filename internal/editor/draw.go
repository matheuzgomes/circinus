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

func (e *Editor) draw() {
	e.screen.Clear()
	w, h := e.screen.Size()
	titleStyle := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorLightGreen).Bold(true)
	statusStyle := tcell.StyleDefault.Foreground(tcell.ColorLightGreen).Background(tcell.ColorBlack)
	e.screen.Fill(' ', tcell.StyleDefault.Background(tcell.ColorBlack))

	routes := e.planRelationRoutes()
	for i, element := range e.doc.Elements {
		if element.Kind == "arrow" {
			e.drawArrowPath(element, i == e.selected || i == e.hovered, routes[i])
		}
	}
	for i, element := range e.doc.Elements {
		if element.Kind == "arrow" {
			e.drawArrowLabel(element, i == e.selected || i == e.hovered, routes[i])
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

	e.screen.PutStrStyled(0, 0, fmt.Sprintf(" Circinus  %s ", filepath.Base(e.path)), titleStyle)
	e.drawDetails(w, h)
	e.screen.PutStrStyled(0, h-2, fit(" "+e.status, w), statusStyle)
	e.screen.PutStrStyled(0, h-1, fit(" b node  n annotation  a relation  m route  x delete  Tab select  e edit  r ref  h/j/k/l move  +/- zoom  0 fit  u undo  Ctrl-r redo  Ctrl-s save  q quit", w), statusStyle)
	e.screen.Show()
}

func (e *Editor) drawDetails(width, height int) {
	style := tcell.StyleDefault.Foreground(tcell.ColorLightCyan).Background(tcell.ColorBlack)
	start := height - 5
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
		lines = []string{
			fmt.Sprintf(" node %s", element.ID),
			fmt.Sprintf(" label: %s", displayValue(element.Label)),
			fmt.Sprintf(" ref: %s", formatReference(element.Ref)),
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
	fromPort, toPort := relationPorts(from, to)
	pairOffset := relationPairLane(e.doc.Elements, index)
	fromOffset := pairOffset + e.nodePortOffset(index, fromPort)
	toOffset := pairOffset + e.nodePortOffset(index, toPort)
	return geometry.ArrowEdgesWithOffsets(fromBox, toBox, fromCenter, toCenter, fromOffset, toOffset)
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

func routeBoxes(relation document.Element, elements []document.Element) []geometry.Box {
	boxes := make([]geometry.Box, 0)
	for _, element := range elements {
		if (element.Kind == "box" || element.Kind == "annotation") && element.ID != relation.From && element.ID != relation.To {
			boxes = append(boxes, geometry.Box{X: element.X, Y: element.Y, W: element.W, H: element.H})
		}
	}
	return boxes
}

func (e *Editor) drawArrowPath(element document.Element, selected bool, route relationRoute) {
	if len(route.points) == 0 {
		return
	}
	points := e.screenRoutePoints(route.points)
	style := e.arrowStyle(element, selected, route.warning)
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

func (e *Editor) drawArrowLabel(element document.Element, selected bool, route relationRoute) {
	if len(route.points) == 0 || element.Label == "" || (e.zoom < 50 && !selected) {
		return
	}
	points := e.screenRoutePoints(route.points)
	placement := geometry.PlaceLabel(points, element.Label)
	if placement.Width > 0 {
		e.screen.PutStrStyled(placement.X, placement.Y, truncate(element.Label, placement.Width), e.arrowStyle(element, selected, route.warning))
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

func (e *Editor) arrowStyle(element document.Element, selected, warning bool) tcell.Style {
	style := tcell.StyleDefault.Foreground(tcell.ColorLightBlue)
	if e.referenceBroken(element) || warning {
		style = style.Foreground(tcell.ColorRed)
	}
	if selected {
		style = style.Foreground(tcell.ColorYellow).Bold(true)
	}
	return style
}

func (e *Editor) referenceBroken(element document.Element) bool {
	return element.Ref != nil && document.ValidateReference(e.root, *element.Ref) != nil
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
