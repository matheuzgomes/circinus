package editor

import (
	"strings"

	"circinus/internal/document"
	"circinus/internal/screen"

	"github.com/gdamore/tcell/v2"
)

const shapePromptWidth = 38

func promptNodeShape(s screen.Screen) (document.Shape, bool) {
	choices := document.SpecializedShapeDefinitions()
	if s == nil || len(choices) == 0 {
		return "", false
	}
	selected := 0
	for {
		drawShapePrompt(s, choices, selected)
		event := s.PollEvent()
		key, ok := event.(*tcell.EventKey)
		if !ok {
			continue
		}
		switch key.Key() {
		case tcell.KeyUp:
			selected = (selected + len(choices) - 1) % len(choices)
		case tcell.KeyDown, tcell.KeyTab:
			selected = (selected + 1) % len(choices)
		case tcell.KeyEnter:
			return choices[selected].Shape, true
		case tcell.KeyEscape, tcell.KeyCtrlC:
			return "", false
		case tcell.KeyRune:
			switch key.Rune() {
			case 'k':
				selected = (selected + len(choices) - 1) % len(choices)
			case 'j':
				selected = (selected + 1) % len(choices)
			}
		}
	}
}

func drawShapePrompt(s screen.Screen, choices []document.ShapeDefinition, selected int) {
	width, height := s.Size()
	panelHeight := len(choices) + 6
	x := (width - shapePromptWidth) / 2
	y := (height - panelHeight) / 2
	if x < 0 {
		x = 0
	}
	if y < 1 {
		y = 1
	}
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorDarkBlue)
	selectedStyle := style.Foreground(tcell.ColorYellow).Bold(true)
	for row := 0; row < panelHeight; row++ {
		s.PutStrStyled(x, y+row, strings.Repeat(" ", shapePromptWidth), style)
	}
	s.PutStrStyled(x, y, "╭"+strings.Repeat("─", shapePromptWidth-2)+"╮", style)
	s.PutStrStyled(x, y+1, popupLine("Create specialized node", shapePromptWidth), selectedStyle)
	s.PutStrStyled(x, y+2, popupLine("", shapePromptWidth), style)
	for i, choice := range choices {
		marker := "  "
		lineStyle := style
		if i == selected {
			marker = "> "
			lineStyle = selectedStyle
		}
		s.PutStrStyled(x, y+3+i, popupLine(marker+choice.Label, shapePromptWidth), lineStyle)
	}
	footerY := y + 3 + len(choices)
	s.PutStrStyled(x, footerY, popupLine("", shapePromptWidth), style)
	s.PutStrStyled(x, footerY+1, popupLine("↑/↓ choose  Enter select  Esc cancel", shapePromptWidth), style)
	s.PutStrStyled(x, footerY+2, "╰"+strings.Repeat("─", shapePromptWidth-2)+"╯", style)
	s.Show()
}

func popupLine(value string, width int) string {
	innerWidth := width - 2
	value = fit(value, innerWidth)
	padding := innerWidth - len([]rune(value))
	return "│" + value + strings.Repeat(" ", padding) + "│"
}
