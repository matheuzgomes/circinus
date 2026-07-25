package editor

import (
	"fmt"

	"circinus/internal/document"
	"circinus/internal/geometry"

	"github.com/gdamore/tcell/v2"
)

func (e *Editor) beginRouteEdit() {
	if e.selected < 0 || e.selected >= len(e.doc.Elements) || e.doc.Elements[e.selected].Kind != "arrow" {
		e.status = "select a relation first"
		return
	}
	e.routeMode = true
	e.routePoint = -1
	if len(e.doc.Elements[e.selected].ControlPoints) > 0 {
		e.routePoint = 0
	}
	e.status = "route mode: Tab points, arrows move, Enter adds, Delete removes, r resets"
}

func (e *Editor) handleRouteKey(event *tcell.EventKey) bool {
	if event.Key() == tcell.KeyEscape {
		e.routeMode = false
		e.routePoint = -1
		e.status = "route mode closed"
		return false
	}
	if event.Key() == tcell.KeyDelete {
		e.deleteRoutePoint()
		return false
	}
	if event.Key() == tcell.KeyTab {
		e.nextRoutePoint()
		return false
	}
	if event.Key() == tcell.KeyEnter {
		e.insertRoutePoint()
		return false
	}
	if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown || event.Key() == tcell.KeyLeft || event.Key() == tcell.KeyRight || event.Key() == tcell.KeyRune {
		dx, dy := 0, 0
		switch event.Key() {
		case tcell.KeyUp:
			dy = -1
		case tcell.KeyDown:
			dy = 1
		case tcell.KeyLeft:
			dx = -1
		case tcell.KeyRight:
			dx = 1
		case tcell.KeyRune:
			switch event.Rune() {
			case 'h':
				dx = -1
			case 'j':
				dy = 1
			case 'k':
				dy = -1
			case 'l':
				dx = 1
			case 'r':
				e.resetRoute()
				return false
			default:
				return false
			}
		}
		e.moveRoutePoint(dx, dy)
	}
	return false
}

func (e *Editor) nextRoutePoint() {
	if e.selected < 0 || e.selected >= len(e.doc.Elements) {
		return
	}
	points := e.doc.Elements[e.selected].ControlPoints
	if len(points) == 0 {
		e.routePoint = -1
		e.status = "no control points; press Enter or drag a segment to add one"
		return
	}
	e.routePoint = (e.routePoint + 1) % len(points)
	e.status = fmt.Sprintf("control point %d/%d", e.routePoint+1, len(points))
}

func (e *Editor) insertRoutePoint() {
	if e.selected < 0 || e.selected >= len(e.doc.Elements) || e.doc.Elements[e.selected].Kind != "arrow" {
		return
	}
	route := e.planRelationRoutes()[e.selected]
	if len(route.points) < 2 {
		return
	}
	middle := route.points[len(route.points)/2]
	e.checkpoint()
	e.doc.Elements[e.selected].ControlPoints = append(e.doc.Elements[e.selected].ControlPoints, document.Point{X: middle.X, Y: middle.Y})
	e.routePoint = len(e.doc.Elements[e.selected].ControlPoints) - 1
	e.dirty = true
	e.status = fmt.Sprintf("control point %d added", e.routePoint+1)
}

func (e *Editor) moveRoutePoint(dx, dy int) {
	if e.selected < 0 || e.selected >= len(e.doc.Elements) || e.routePoint < 0 || e.routePoint >= len(e.doc.Elements[e.selected].ControlPoints) || (dx == 0 && dy == 0) {
		return
	}
	e.checkpoint()
	point := &e.doc.Elements[e.selected].ControlPoints[e.routePoint]
	point.X += dx
	point.Y += dy
	e.dirty = true
}

func (e *Editor) deleteRoutePoint() {
	if e.selected < 0 || e.selected >= len(e.doc.Elements) || e.routePoint < 0 || e.routePoint >= len(e.doc.Elements[e.selected].ControlPoints) {
		return
	}
	e.checkpoint()
	points := e.doc.Elements[e.selected].ControlPoints
	e.doc.Elements[e.selected].ControlPoints = append(points[:e.routePoint], points[e.routePoint+1:]...)
	if e.routePoint >= len(e.doc.Elements[e.selected].ControlPoints) {
		e.routePoint = len(e.doc.Elements[e.selected].ControlPoints) - 1
	}
	e.dirty = true
	e.status = "control point removed"
}

func (e *Editor) resetRoute() {
	if e.selected < 0 || e.selected >= len(e.doc.Elements) || e.doc.Elements[e.selected].Kind != "arrow" || len(e.doc.Elements[e.selected].ControlPoints) == 0 {
		return
	}
	e.checkpoint()
	e.doc.Elements[e.selected].ControlPoints = nil
	e.routePoint = -1
	e.dirty = true
	e.status = "automatic route restored"
}

func routeThroughControlPoints(start, end geometry.Point, controls []document.Point, obstacles []geometry.Box, reserved []geometry.Point) []geometry.Point {
	if len(controls) == 0 {
		return geometry.RouteAroundBoxesAndPoints(start, end, obstacles, reserved)
	}
	result := []geometry.Point{start}
	current := start
	for _, control := range controls {
		next := geometry.Point{X: control.X, Y: control.Y}
		segment := geometry.RouteAroundBoxesAndPoints(current, next, obstacles, reserved)
		result = appendRoute(result, segment)
		current = next
	}
	result = appendRoute(result, geometry.RouteAroundBoxesAndPoints(current, end, obstacles, reserved))
	return result
}

func appendRoute(result, segment []geometry.Point) []geometry.Point {
	if len(segment) == 0 {
		return result
	}
	if len(result) > 0 && result[len(result)-1] == segment[0] {
		segment = segment[1:]
	}
	return append(result, segment...)
}
