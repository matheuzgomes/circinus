package editor

import (
	"strconv"

	"circinus/internal/geometry"
)

var zoomLevels = []int{25, 50, 75, 100, 125, 150, 200}

func (e *Editor) canvasToScreen(x, y int) (int, int) {
	if e.zoom == 0 {
		e.zoom = 100
	}
	return (x - e.viewportX) * e.zoom / 100, (y-e.viewportY)*e.zoom/100 + 1
}

func (e *Editor) scaledSize(size int) int {
	if e.zoom == 0 {
		e.zoom = 100
	}
	if e.zoom < 50 {
		return 1
	}
	value := size * e.zoom / 100
	if value < 1 {
		return 1
	}
	return value
}

func (e *Editor) changeZoom(direction int) {
	if e.zoom == 0 {
		e.zoom = 100
	}
	focusX, focusY := e.viewportX, e.viewportY
	if e.selected >= 0 && e.selected < len(e.doc.Elements) && e.doc.Elements[e.selected].Kind != "arrow" {
		element := e.doc.Elements[e.selected]
		focusX = element.X + element.W/2
		focusY = element.Y + element.H/2
	}
	oldZoom := e.zoom
	if direction > 0 {
		for _, level := range zoomLevels {
			if level > e.zoom {
				e.zoom = level
				break
			}
		}
	} else if direction < 0 {
		for i := len(zoomLevels) - 1; i >= 0; i-- {
			if zoomLevels[i] < e.zoom {
				e.zoom = zoomLevels[i]
				break
			}
		}
	}
	if e.zoom == oldZoom {
		return
	}
	oldScreenX := (focusX - e.viewportX) * oldZoom / 100
	oldScreenY := (focusY - e.viewportY) * oldZoom / 100
	e.viewportX = focusX - oldScreenX*100/e.zoom
	e.viewportY = focusY - oldScreenY*100/e.zoom
	e.invalidateSceneScreen()
	e.status = "zoom " + zoomText(e.zoom)
}

func (e *Editor) fitToScreen() {
	if e.screen == nil || len(e.doc.Elements) == 0 {
		e.zoom = 100
		e.status = "zoom 100%"
		return
	}
	minX, minY := 0, 0
	maxX, maxY := 1, 1
	first := true
	for _, element := range e.doc.Elements {
		if element.Kind == "arrow" {
			continue
		}
		if first || element.X < minX {
			minX = element.X
		}
		if first || element.Y < minY {
			minY = element.Y
		}
		if first || element.X+element.W > maxX {
			maxX = element.X + element.W
		}
		if first || element.Y+element.H > maxY {
			maxY = element.Y + element.H
		}
		first = false
	}
	if first {
		return
	}
	width, height := e.screen.Size()
	availableWidth, availableHeight := geometry.Max(1, width-2), geometry.Max(1, height-7)
	e.zoom = 25
	for _, level := range zoomLevels {
		if (maxX-minX)*level/100 <= availableWidth && (maxY-minY)*level/100 <= availableHeight {
			e.zoom = level
		}
	}
	e.viewportX = minX - 1
	e.viewportY = minY - 1
	e.status = "fit " + zoomText(e.zoom)
}

func zoomText(value int) string {
	return strconv.Itoa(value) + "%"
}
