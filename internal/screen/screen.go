package screen

import "github.com/gdamore/tcell/v2"

type Screen interface {
	Clear()
	Fill(r rune, style tcell.Style)
	SetContent(x, y int, r rune, comb []rune, style tcell.Style)
	PutStrStyled(x, y int, s string, style tcell.Style)
	Size() (int, int)
	Show()
	Sync()
	Suspend() error
	Resume() error
	Fini()
	PollEvent() tcell.Event
}
