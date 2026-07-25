package editor

import (
	"strconv"
	"strings"

	"circinus/internal/document"

	"github.com/gdamore/tcell/v2"
)

func (e *Editor) addAnnotation() {
	text, ok := promptMultiline(e.screen, "annotation", "")
	if !ok || strings.TrimSpace(text) == "" {
		e.status = "annotation cancelled"
		return
	}
	e.checkpoint()
	n := e.nextElement
	e.nextElement++
	e.doc.Elements = append(e.doc.Elements, document.Element{
		ID: fmtElementID("annotation", n), Kind: "annotation",
		X: e.viewportX + 4, Y: e.viewportY + 3, W: 32, H: annotationHeight(text, 32), Text: text,
	})
	e.selected = len(e.doc.Elements) - 1
	e.dirty = true
	e.status = "annotation added"
}

func (e *Editor) resizeAnnotation(index, dx int) {
	if index < 0 || index >= len(e.doc.Elements) || e.doc.Elements[index].Kind != "annotation" {
		return
	}
	zoom := e.zoom
	if zoom < 1 {
		zoom = 1
	}
	width := e.doc.Elements[index].W + dx*100/zoom
	if width < 8 {
		width = 8
	}
	e.doc.Elements[index].W = width
	e.doc.Elements[index].H = annotationHeight(e.doc.Elements[index].Text, width)
	e.dirty = true
}

func annotationHeight(text string, width int) int {
	if width < 1 {
		width = 1
	}
	height := 0
	for _, line := range strings.Split(text, "\n") {
		rows := (len([]rune(line)) + width - 1) / width
		if rows < 1 {
			rows = 1
		}
		height += rows
	}
	if height < 1 {
		return 1
	}
	return height
}

func (e *Editor) editAnnotation() {
	if e.selected < 0 || e.selected >= len(e.doc.Elements) || e.doc.Elements[e.selected].Kind != "annotation" {
		e.status = "select an annotation first"
		return
	}
	value, ok := promptMultiline(e.screen, "annotation", e.doc.Elements[e.selected].Text)
	if !ok {
		return
	}
	e.checkpoint()
	e.doc.Elements[e.selected].Text = value
	e.doc.Elements[e.selected].H = annotationHeight(value, e.doc.Elements[e.selected].W)
	e.dirty = true
	e.status = "annotation updated"
}

func fmtElementID(prefix string, n int) string {
	return prefix + "-" + strconv.Itoa(n)
}

func promptMultiline(s interface {
	Size() (int, int)
	PutStrStyled(int, int, string, tcell.Style)
	Show()
	PollEvent() tcell.Event
}, label, initial string) (string, bool) {
	value := []rune(initial)
	for {
		_, height := s.Size()
		lines := strings.Split(string(value), "\n")
		start := height - len(lines) - 2
		if start < 0 {
			start = 0
		}
		s.PutStrStyled(0, start, label+" (Ctrl-Enter confirms; Esc cancels)", tcell.StyleDefault.Foreground(tcell.ColorYellow))
		for i, line := range lines {
			s.PutStrStyled(0, start+1+i, line, tcell.StyleDefault.Foreground(tcell.ColorYellow))
		}
		s.Show()
		event, ok := s.PollEvent().(*tcell.EventKey)
		if !ok {
			continue
		}
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyCtrlC {
			return "", false
		}
		if event.Key() == tcell.KeyEnter {
			if event.Modifiers()&tcell.ModCtrl != 0 {
				return string(value), true
			}
			value = append(value, '\n')
			continue
		}
		switch event.Key() {
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if len(value) > 0 {
				value = value[:len(value)-1]
			}
		case tcell.KeyRune:
			value = append(value, event.Rune())
		}
	}
}
