package editor

import (
	"circinus/internal/document"
	"circinus/internal/geometry"

	"github.com/gdamore/tcell/v2"
)

func (e *Editor) handleMouse(event *tcell.EventMouse) {
	x, y := event.Position()
	buttons := event.Buttons()
	e.hovered = e.hitElement(x, y)
	if e.hovered < 0 {
		e.hovered, _ = e.hitRouteSegment(x, y)
	}
	if buttons&tcell.WheelUp != 0 {
		e.changeZoom(1)
		return
	}
	if buttons&tcell.WheelDown != 0 {
		e.changeZoom(-1)
		return
	}
	if buttons == tcell.ButtonNone {
		e.dragging = false
		e.dragKind = ""
		return
	}
	if buttons&tcell.Button1 == 0 {
		return
	}
	if !e.dragging {
		e.startMouseDrag(x, y)
		return
	}
	e.updateMouseDrag(x, y)
}

func (e *Editor) startMouseDrag(x, y int) {
	e.lastMouseX, e.lastMouseY = x, y
	if index, control := e.hitControlPoint(x, y); index >= 0 {
		e.selected = index
		e.routeMode = true
		e.routePoint = control
		e.checkpoint()
		e.dragging = true
		e.dragKind = "control"
		e.dragElement = index
		e.dragControl = control
		return
	}
	if index := e.hitElement(x, y); index >= 0 {
		e.selected = index
		if e.doc.Elements[index].Kind == "box" || e.doc.Elements[index].Kind == "annotation" {
			e.checkpoint()
			e.dragging = true
			e.dragKind = "element"
			e.dragElement = index
			sx, _ := e.canvasToScreen(e.doc.Elements[index].X, e.doc.Elements[index].Y)
			width := e.scaledSize(e.doc.Elements[index].W)
			if e.doc.Elements[index].Kind == "annotation" && x >= sx+width-1 {
				e.dragKind = "annotation-resize"
			}
		}
		return
	}
	if index, _ := e.hitRouteSegment(x, y); index >= 0 {
		e.selected = index
		e.checkpoint()
		canvas := e.mouseCanvasPoint(x, y)
		e.doc.Elements[index].ControlPoints = append(e.doc.Elements[index].ControlPoints, documentPoint(canvas))
		e.routeMode = true
		e.routePoint = len(e.doc.Elements[index].ControlPoints) - 1
		e.dragging = true
		e.dragKind = "control"
		e.dragElement = index
		e.dragControl = e.routePoint
		e.dirty = true
		return
	}
	e.dragging = true
	e.dragKind = "pan"
	e.dragElement = -1
}

func (e *Editor) updateMouseDrag(x, y int) {
	if e.dragKind != "pan" && (e.dragElement < 0 || e.dragElement >= len(e.doc.Elements)) {
		return
	}
	dx, dy := x-e.lastMouseX, y-e.lastMouseY
	if dx == 0 && dy == 0 {
		return
	}
	switch e.dragKind {
	case "element":
		e.moveElementByScreenDelta(e.dragElement, dx, dy)
	case "annotation-resize":
		e.resizeAnnotation(e.dragElement, dx)
	case "pan":
		step := e.zoom
		if step < 1 {
			step = 1
		}
		e.viewportX -= dx * 100 / step
		e.viewportY -= dy * 100 / step
	case "control":
		point := e.mouseCanvasPoint(x, y)
		if e.dragControl >= 0 && e.dragControl < len(e.doc.Elements[e.dragElement].ControlPoints) {
			e.doc.Elements[e.dragElement].ControlPoints[e.dragControl] = documentPoint(point)
			e.dirty = true
		}
	}
	e.lastMouseX, e.lastMouseY = x, y
}

func (e *Editor) moveElementByScreenDelta(index, dx, dy int) {
	if index < 0 || index >= len(e.doc.Elements) || (e.doc.Elements[index].Kind != "box" && e.doc.Elements[index].Kind != "annotation") {
		return
	}
	step := e.zoom
	if step < 1 {
		step = 1
	}
	e.doc.Elements[index].X += dx * 100 / step
	e.doc.Elements[index].Y += dy * 100 / step
	e.dirty = true
}

func (e *Editor) hitElement(x, y int) int {
	for index := len(e.doc.Elements) - 1; index >= 0; index-- {
		element := e.doc.Elements[index]
		if element.Kind != "box" && element.Kind != "annotation" {
			continue
		}
		sx, sy := e.canvasToScreen(element.X, element.Y)
		width, height := e.scaledSize(element.W), e.scaledSize(element.H)
		if x >= sx && x < sx+width && y >= sy && y < sy+height {
			return index
		}
	}
	return -1
}

func (e *Editor) hitControlPoint(x, y int) (int, int) {
	for index, element := range e.doc.Elements {
		if element.Kind != "arrow" {
			continue
		}
		for control, point := range element.ControlPoints {
			sx, sy := e.canvasToScreen(point.X, point.Y)
			if x == sx && y == sy {
				return index, control
			}
		}
	}
	return -1, -1
}

func (e *Editor) hitRouteSegment(x, y int) (int, geometry.Point) {
	for index, element := range e.doc.Elements {
		if element.Kind != "arrow" {
			continue
		}
		points := e.screenRoutePoints(e.planRelationRoutes()[index].points)
		for pointIndex, point := range points {
			if pointIndex == 0 || pointIndex == len(points)-1 || point.X != x || point.Y != y {
				continue
			}
			return index, geometry.Point{X: point.X, Y: point.Y}
		}
	}
	return -1, geometry.Point{}
}

func (e *Editor) mouseCanvasPoint(x, y int) geometry.Point {
	zoom := e.zoom
	if zoom < 1 {
		zoom = 1
	}
	return geometry.Point{X: x*100/zoom + e.viewportX, Y: (y-1)*100/zoom + e.viewportY}
}

func documentPoint(point geometry.Point) document.Point {
	return document.Point{X: point.X, Y: point.Y}
}
