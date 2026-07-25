package editor

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"circinus/internal/screen"

	"github.com/gdamore/tcell/v2"
)

func (e *Editor) resolveReferenceInput(input string, allowDirectories bool) (string, bool) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "@") {
		return filepath.ToSlash(input), true
	}
	query := strings.TrimSpace(strings.TrimPrefix(input, "@"))
	candidates, err := referenceCandidates(e.root, query, allowDirectories)
	if err != nil {
		e.status = err.Error()
		return "", false
	}
	if len(candidates) == 0 {
		e.status = "no reference matches"
		return "", false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	return promptChoice(e.screen, "reference", candidates)
}

func referenceCandidates(root, query string, allowDirectories bool) ([]string, error) {
	query = strings.ToLower(filepath.ToSlash(query))
	var candidates []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root && entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if path == root || (!allowDirectories && entry.IsDir()) {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if query == "" || strings.Contains(strings.ToLower(relative), query) {
			candidates = append(candidates, relative)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(candidates)
	return candidates, nil
}

func promptChoice(s screen.Screen, label string, choices []string) (string, bool) {
	selected := 0
	for {
		_, height := s.Size()
		start := height - len(choices) - 2
		if start < 0 {
			start = 0
		}
		s.PutStrStyled(0, start, fmt.Sprintf("%s (Enter confirms; Esc cancels)", label), tcell.StyleDefault.Foreground(tcell.ColorYellow))
		for i, choice := range choices {
			style := tcell.StyleDefault.Foreground(tcell.ColorWhite)
			if i == selected {
				style = style.Foreground(tcell.ColorYellow).Bold(true)
			}
			s.PutStrStyled(0, start+1+i, choice, style)
		}
		s.Show()
		event, ok := s.PollEvent().(*tcell.EventKey)
		if !ok {
			continue
		}
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyCtrlC:
			return "", false
		case tcell.KeyUp:
			if selected > 0 {
				selected--
			}
		case tcell.KeyDown, tcell.KeyTab:
			selected = (selected + 1) % len(choices)
		case tcell.KeyEnter:
			return choices[selected], true
		}
	}
}
