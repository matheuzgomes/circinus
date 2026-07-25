package editor

import (
	"strings"

	"circinus/internal/screen"

	"github.com/gdamore/tcell/v2"
)

func prompt(s screen.Screen, label, initial string) (string, bool) {
	value := []rune(initial)
	for {
		_, h := s.Size()
		s.PutStrStyled(0, h-1, strings.Repeat(" ", 160), tcell.StyleDefault)
		s.PutStrStyled(0, h-1, label+": "+string(value), tcell.StyleDefault.Foreground(tcell.ColorYellow))
		s.Show()
		event := s.PollEvent()
		key, ok := event.(*tcell.EventKey)
		if !ok {
			continue
		}
		switch key.Key() {
		case tcell.KeyEnter:
			return string(value), true
		case tcell.KeyEscape, tcell.KeyCtrlC:
			return "", false
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if len(value) > 0 {
				value = value[:len(value)-1]
			}
		case tcell.KeyRune:
			value = append(value, key.Rune())
		}
	}
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "\u2026"
}
