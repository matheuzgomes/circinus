package editor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"circinus/internal/document"
	"circinus/internal/screen"

	"github.com/gdamore/tcell/v2"
)

type Editor struct {
	screen      screen.Screen
	root        string
	path        string
	doc         document.Document
	selected    int
	viewportX   int
	viewportY   int
	zoom        int
	routeMode   bool
	routePoint  int
	dirty       bool
	status      string
	quitArmed   bool
	connectMode bool
	connectFrom int
	history     []document.Document
	future      []document.Document
	nextElement int
	dragging    bool
	dragKind    string
	dragElement int
	dragControl int
	hovered     int
	lastMouseX  int
	lastMouseY  int
}

func EditDocument(root, input string) error {
	path, err := repoPath(root, input)
	if err != nil {
		return err
	}
	doc, err := document.Load(path)
	if err != nil {
		return err
	}
	if err := doc.ValidateStructure(); err != nil {
		return err
	}
	s, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := s.Init(); err != nil {
		return err
	}
	s.EnableMouse()
	defer s.Fini()

	e := &Editor{screen: s, root: root, path: path, doc: doc, selected: -1, connectFrom: -1, routePoint: -1, hovered: -1, zoom: 100, status: "ready"}
	e.nextElement = e.doc.NextID()
	return e.loop()
}

func (e *Editor) loop() error {
	for {
		e.draw()
		event := e.screen.PollEvent()
		switch event := event.(type) {
		case *tcell.EventResize:
			e.screen.Sync()
		case *tcell.EventMouse:
			e.handleMouse(event)
		case *tcell.EventKey:
			if done, err := e.handleKey(event); err != nil {
				return err
			} else if done {
				return nil
			}
		}
	}
}

func (e *Editor) handleKey(event *tcell.EventKey) (bool, error) {
	if e.routeMode {
		return e.handleRouteKey(event), nil
	}
	isQuitKey := event.Key() == tcell.KeyCtrlC || (event.Key() == tcell.KeyRune && event.Rune() == 'q')
	if !isQuitKey {
		e.quitArmed = false
	}
	switch event.Key() {
	case tcell.KeyCtrlB:
		e.addSpecializedNode()
		return false, nil
	case tcell.KeyCtrlC:
		return e.quit()
	case tcell.KeyCtrlS:
		if err := e.save(); err != nil {
			e.status = err.Error()
		} else {
			e.status = "saved"
		}
		return false, nil
	case tcell.KeyCtrlR:
		e.redo()
		return false, nil
	case tcell.KeyDelete:
		e.deleteSelected()
		return false, nil
	case tcell.KeyEscape:
		if e.connectMode {
			e.connectMode = false
			e.connectFrom = -1
			e.status = "connection cancelled"
		} else {
			e.status = "selection cleared"
			e.selected = -1
		}
	case tcell.KeyTab:
		if e.connectMode {
			e.selectNextBox()
		} else {
			e.selectNext()
		}
	case tcell.KeyUp:
		e.moveSelected(0, -1)
	case tcell.KeyDown:
		e.moveSelected(0, 1)
	case tcell.KeyLeft:
		e.moveSelected(-1, 0)
	case tcell.KeyRight:
		e.moveSelected(1, 0)
	case tcell.KeyEnter:
		if e.connectMode {
			e.confirmConnection()
		} else {
			e.openReference()
		}
	case tcell.KeyRune:
		switch event.Rune() {
		case 'q':
			return e.quit()
		case 'b':
			e.addBox()
		case 'a':
			e.beginConnection()
		case 'e':
			if e.selected >= 0 && e.selected < len(e.doc.Elements) && e.doc.Elements[e.selected].Kind == "annotation" {
				e.editAnnotation()
			} else {
				e.editLabel()
			}
		case 'r':
			e.editReference()
		case 'm':
			e.beginRouteEdit()
		case 'n':
			e.addAnnotation()
		case 's':
			e.status = "selection mode"
		case 'u':
			e.undo()
		case 'h':
			e.moveSelected(-1, 0)
		case 'j':
			e.moveSelected(0, 1)
		case 'k':
			e.moveSelected(0, -1)
		case 'l':
			e.moveSelected(1, 0)
		case 'x':
			e.deleteSelected()
		case '+', '=':
			e.changeZoom(1)
		case '-':
			e.changeZoom(-1)
		case '0':
			e.fitToScreen()
		case '/':
			e.status = "search is pending"
		}
	}
	return false, nil
}

func (e *Editor) quit() (bool, error) {
	if e.dirty && !e.quitArmed {
		e.quitArmed = true
		e.status = "unsaved changes; press q again to discard"
		return false, nil
	}
	return true, nil
}

func (e *Editor) checkpoint() {
	e.history = append(e.history, cloneDocument(e.doc))
	e.future = nil
}

func cloneDocument(source document.Document) document.Document {
	clone := source
	clone.Elements = make([]document.Element, len(source.Elements))
	copy(clone.Elements, source.Elements)
	for i := range clone.Elements {
		clone.Elements[i].Ref = cloneReference(source.Elements[i].Ref)
		clone.Elements[i].ControlPoints = append([]document.Point(nil), source.Elements[i].ControlPoints...)
	}
	return clone
}

func cloneReference(source *document.Reference) *document.Reference {
	if source == nil {
		return nil
	}
	clone := *source
	if source.Line != nil {
		line := *source.Line
		clone.Line = &line
	}
	return &clone
}

func (e *Editor) undo() {
	if len(e.history) == 0 {
		e.status = "nothing to undo"
		return
	}
	e.future = append(e.future, cloneDocument(e.doc))
	e.doc = cloneDocument(e.history[len(e.history)-1])
	e.history = e.history[:len(e.history)-1]
	e.selected = -1
	e.connectMode = false
	e.connectFrom = -1
	e.routeMode = false
	e.routePoint = -1
	e.dirty = true
	e.status = "undone"
}

func (e *Editor) redo() {
	if len(e.future) == 0 {
		e.status = "nothing to redo"
		return
	}
	e.history = append(e.history, cloneDocument(e.doc))
	e.doc = cloneDocument(e.future[len(e.future)-1])
	e.future = e.future[:len(e.future)-1]
	e.selected = -1
	e.connectMode = false
	e.connectFrom = -1
	e.routeMode = false
	e.routePoint = -1
	e.dirty = true
	e.status = "redone"
}

func (e *Editor) save() error {
	if err := e.doc.Validate(e.root); err != nil {
		return err
	}
	if err := e.doc.Save(e.path); err != nil {
		return err
	}
	e.dirty = false
	return nil
}

func (e *Editor) addBox() {
	e.addNode(document.ShapeBox)
}

func (e *Editor) addSpecializedNode() {
	shape, ok := promptNodeShape(e.screen)
	if !ok {
		e.status = "node creation cancelled"
		return
	}
	e.addNode(shape)
}

func (e *Editor) addNode(shape document.Shape) {
	definition, ok := document.ShapeDefinitionFor(shape)
	if !ok {
		e.status = fmt.Sprintf("unsupported node shape %q", shape)
		return
	}
	e.checkpoint()
	n := e.nextElement
	e.nextElement++
	shapeValue := ""
	if shape != document.ShapeBox {
		shapeValue = string(shape)
	}
	e.doc.Elements = append(e.doc.Elements, document.Element{
		ID: fmt.Sprintf("box-%d", n), Kind: "box",
		X: e.viewportX + 4 + (n%4)*3, Y: e.viewportY + 3 + (n%3)*2,
		W: definition.DefaultWidth, H: definition.DefaultHeight,
		Shape: shapeValue,
	})
	e.selected = len(e.doc.Elements) - 1
	e.dirty = true
	if e.screen != nil {
		if label, ok := prompt(e.screen, "node label", ""); ok {
			e.doc.Elements[e.selected].Label = label
		}
	}
	e.status = "node added; r edits reference"
}

func (e *Editor) beginConnection() {
	if e.selected < 0 || e.selected >= len(e.doc.Elements) || e.doc.Elements[e.selected].Kind != "box" {
		e.status = "select a source box before pressing a"
		return
	}
	if countBoxes(e.doc.Elements) < 2 {
		e.status = "add a second box first"
		return
	}
	e.connectMode = true
	e.connectFrom = e.selected
	e.status = fmt.Sprintf("connect from %s; Tab chooses target, Enter confirms", e.doc.Elements[e.selected].ID)
}

func (e *Editor) selectNextBox() {
	target := nextBoxIndex(e.doc.Elements, e.selected, e.connectFrom)
	if target < 0 {
		e.status = "no other box available"
		return
	}
	e.selected = target
	e.status = fmt.Sprintf("target %s; Enter confirms, Esc cancels", e.doc.Elements[target].ID)
}

func (e *Editor) confirmConnection() {
	if !e.connectMode || e.connectFrom < 0 || e.connectFrom >= len(e.doc.Elements) || e.doc.Elements[e.connectFrom].Kind != "box" || e.selected < 0 || e.selected >= len(e.doc.Elements) || e.doc.Elements[e.selected].Kind != "box" || e.selected == e.connectFrom {
		e.status = "choose a different target box"
		return
	}
	from, to := e.doc.Elements[e.connectFrom].ID, e.doc.Elements[e.selected].ID
	candidate := document.Element{Kind: "arrow", From: from, To: to}
	if _, _, err := e.doc.RelationNodes(candidate); err != nil {
		e.status = err.Error()
		return
	}
	if e.screen != nil {
		if label, ok := prompt(e.screen, "relation label (optional)", ""); ok {
			candidate.Label = label
		}
	}
	if duplicateRelation(e.doc.Elements, candidate) {
		e.status = "that exact relation already exists"
		return
	}
	e.checkpoint()
	candidate.ID = fmt.Sprintf("arrow-%d", e.nextElement)
	e.nextElement++
	e.doc.Elements = append(e.doc.Elements, candidate)
	e.selected = len(e.doc.Elements) - 1
	e.connectMode = false
	e.connectFrom = -1
	e.dirty = true
	e.status = "relation added; r edits reference"
}

func (e *Editor) editLabel() {
	if e.selected < 0 || e.selected >= len(e.doc.Elements) {
		e.status = "select an element first"
		return
	}
	item := &e.doc.Elements[e.selected]
	value, ok := prompt(e.screen, "label", item.Label)
	if ok {
		e.checkpoint()
		item.Label = value
		e.dirty = true
		e.status = "label updated"
	}
}

func (e *Editor) editReference() {
	if e.selected < 0 || e.selected >= len(e.doc.Elements) {
		e.status = "select an element first"
		return
	}
	item := &e.doc.Elements[e.selected]
	initial := ""
	if item.Ref != nil {
		initial = item.Ref.Path
	}
	path, ok := prompt(e.screen, "reference path or @query (blank clears)", initial)
	if !ok {
		return
	}
	if strings.TrimSpace(path) == "" {
		e.checkpoint()
		item.Ref = nil
		e.dirty = true
		e.status = "reference cleared"
		return
	}
	path, ok = e.resolveReferenceInput(path, item.Kind == "box")
	if !ok {
		return
	}
	symbol, ok := prompt(e.screen, "symbol (optional)", referenceSymbol(item.Ref))
	if !ok {
		return
	}
	lineText, ok := prompt(e.screen, "line (optional)", referenceLine(item.Ref))
	if !ok {
		return
	}
	var line *int
	if strings.TrimSpace(lineText) != "" {
		value, err := strconv.Atoi(strings.TrimSpace(lineText))
		if err != nil || value < 1 {
			e.status = "line must be a positive integer"
			return
		}
		line = &value
	}
	e.checkpoint()
	item.Ref = &document.Reference{Path: filepath.ToSlash(strings.TrimSpace(path)), Symbol: strings.TrimSpace(symbol), Line: line}
	e.dirty = true
	e.status = "reference updated"
}

func referenceSymbol(ref *document.Reference) string {
	if ref == nil {
		return ""
	}
	return ref.Symbol
}

func referenceLine(ref *document.Reference) string {
	if ref == nil || ref.Line == nil {
		return ""
	}
	return strconv.Itoa(*ref.Line)
}

func (e *Editor) deleteSelected() {
	if e.selected < 0 || e.selected >= len(e.doc.Elements) {
		e.status = "select an element first"
		return
	}
	deleted := e.doc.Elements[e.selected].ID
	e.checkpoint()
	e.doc.Elements = append(e.doc.Elements[:e.selected], e.doc.Elements[e.selected+1:]...)
	for i := 0; i < len(e.doc.Elements); i++ {
		if e.doc.Elements[i].Kind == "arrow" && (e.doc.Elements[i].From == deleted || e.doc.Elements[i].To == deleted) {
			e.doc.Elements = append(e.doc.Elements[:i], e.doc.Elements[i+1:]...)
			i--
		}
	}
	e.selected = -1
	e.dirty = true
	e.status = "element deleted"
}

func (e *Editor) moveSelected(dx, dy int) {
	if e.selected < 0 || e.selected >= len(e.doc.Elements) {
		return
	}
	if e.doc.Elements[e.selected].Kind != "box" && e.doc.Elements[e.selected].Kind != "annotation" {
		e.status = "relations and endpoints are not independently movable"
		return
	}
	e.checkpoint()
	e.doc.Elements[e.selected].X += dx
	e.doc.Elements[e.selected].Y += dy
	e.dirty = true
	e.status = fmt.Sprintf("moved to %d,%d", e.doc.Elements[e.selected].X, e.doc.Elements[e.selected].Y)
}

func (e *Editor) selectNext() {
	if len(e.doc.Elements) == 0 {
		e.status = "no elements"
		return
	}
	e.selected = (e.selected + 1) % len(e.doc.Elements)
	e.status = "selected " + e.doc.Elements[e.selected].ID
}

func countBoxes(elements []document.Element) int {
	count := 0
	for _, element := range elements {
		if element.Kind == "box" {
			count++
		}
	}
	return count
}

func nextBoxIndex(elements []document.Element, current, source int) int {
	if len(elements) == 0 {
		return -1
	}
	for offset := 1; offset <= len(elements); offset++ {
		index := (current + offset) % len(elements)
		if index != source && elements[index].Kind == "box" {
			return index
		}
	}
	return -1
}

func duplicateRelation(elements []document.Element, candidate document.Element) bool {
	for _, element := range elements {
		if element.Kind == "arrow" && element.From == candidate.From && element.To == candidate.To && element.Label == candidate.Label && sameReference(element.Ref, candidate.Ref) {
			return true
		}
	}
	return false
}

func sameReference(left, right *document.Reference) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.Path != right.Path || left.Symbol != right.Symbol {
		return false
	}
	if left.Line == nil || right.Line == nil {
		return left.Line == right.Line
	}
	return *left.Line == *right.Line
}

func repoPath(root, input string) (string, error) {
	if filepath.IsAbs(input) {
		return "", errors.New("path must be relative to the repository root")
	}
	clean := filepath.Clean(input)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", errors.New("path must stay inside the repository root")
	}
	full := filepath.Join(root, clean)
	return full, nil
}

func (e *Editor) openReference() {
	if e.selected < 0 || e.selected >= len(e.doc.Elements) || e.doc.Elements[e.selected].Ref == nil {
		e.status = "selected element has no reference"
		return
	}
	ref := e.doc.Elements[e.selected].Ref
	path, err := repoPath(e.root, filepath.FromSlash(ref.Path))
	if err != nil {
		e.status = err.Error()
		return
	}
	name := os.Getenv("VISUAL")
	if name == "" {
		name = os.Getenv("EDITOR")
	}
	if name == "" {
		name = "vi"
	}
	parts := strings.Fields(name)
	if len(parts) == 0 {
		e.status = "EDITOR is empty"
		return
	}
	args := parts[1:]
	if ref.Line != nil {
		switch filepath.Base(parts[0]) {
		case "vi", "vim", "nvim":
			args = append(args, "+"+strconv.Itoa(*ref.Line), path)
		case "code":
			args = append(args, "--goto", fmt.Sprintf("%s:%d", path, *ref.Line))
		default:
			args = append(args, path)
		}
	} else {
		args = append(args, path)
	}
	_ = e.screen.Suspend()
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = cmd.Run()
	_ = e.screen.Resume()
	if err != nil {
		e.status = "editor: " + err.Error()
	} else {
		e.status = "returned from editor"
	}
}
