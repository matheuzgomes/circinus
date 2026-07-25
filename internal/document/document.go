package document

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const SchemaVersion = 1

type Document struct {
	Schema   int       `json:"schema"`
	Canvas   Canvas    `json:"canvas"`
	Elements []Element `json:"elements"`
}

type Canvas struct {
	Background string `json:"background"`
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Element struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	X             int        `json:"x,omitempty"`
	Y             int        `json:"y,omitempty"`
	W             int        `json:"w,omitempty"`
	H             int        `json:"h,omitempty"`
	Label         string     `json:"label,omitempty"`
	Text          string     `json:"text,omitempty"`
	Shape         string     `json:"shape,omitempty"`
	From          string     `json:"from,omitempty"`
	To            string     `json:"to,omitempty"`
	ControlPoints []Point    `json:"points,omitempty"`
	Ref           *Reference `json:"ref,omitempty"`
}

type Reference struct {
	Path   string `json:"path"`
	Symbol string `json:"symbol,omitempty"`
	Line   *int   `json:"line,omitempty"`
}

func New() Document {
	return Document{Schema: SchemaVersion, Canvas: Canvas{Background: "default"}, Elements: []Element{}}
}

func Load(path string) (Document, error) {
	file, err := os.Open(path)
	if err != nil {
		return Document{}, err
	}
	defer file.Close()

	var doc Document
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("invalid JSON: %w", err)
	}
	return doc, nil
}

func (d *Document) Save(path string) error {
	for i := range d.Elements {
		if d.Elements[i].Kind == "arrow" {
			d.Elements[i].X = 0
			d.Elements[i].Y = 0
			d.Elements[i].W = 0
			d.Elements[i].H = 0
		}
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".circinus-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (d *Document) ValidateStructure() error {
	if d.Schema != SchemaVersion {
		return fmt.Errorf("unsupported schema %d", d.Schema)
	}
	ids := make(map[string]bool, len(d.Elements))
	for _, element := range d.Elements {
		if element.ID == "" {
			return errors.New("element ID cannot be empty")
		}
		if ids[element.ID] {
			return fmt.Errorf("duplicate element ID %q", element.ID)
		}
		ids[element.ID] = true
		switch element.Kind {
		case "box":
			if element.W <= 0 || element.H <= 0 {
				return fmt.Errorf("box %q must have positive width and height", element.ID)
			}
			shape := EffectiveShape(element)
			definition, ok := ShapeDefinitionFor(shape)
			if !ok {
				return fmt.Errorf("box %q has unsupported shape %q", element.ID, shape)
			}
			if shape != ShapeBox && (element.W < definition.DefaultWidth || element.H < definition.DefaultHeight) {
				return fmt.Errorf("box %q is too small for shape %q", element.ID, shape)
			}
		case "arrow":
			if element.Shape != "" || element.Text != "" {
				return fmt.Errorf("arrow %q cannot have shape or annotation text", element.ID)
			}
			if _, _, err := d.RelationNodes(element); err != nil {
				return err
			}
		case "annotation":
			if element.Ref != nil || element.Shape != "" || element.From != "" || element.To != "" {
				return fmt.Errorf("annotation %q cannot have relation or reference fields", element.ID)
			}
			if element.W <= 0 || element.H <= 0 {
				return fmt.Errorf("annotation %q must have positive width and height", element.ID)
			}
		default:
			return fmt.Errorf("element %q has unsupported kind %q", element.ID, element.Kind)
		}
	}
	return nil
}

func (d *Document) Validate(root string) error {
	if err := d.ValidateStructure(); err != nil {
		return err
	}
	for _, element := range d.Elements {
		if element.Ref != nil {
			if err := validateReference(root, *element.Ref); err != nil {
				return fmt.Errorf("element %q: %w", element.ID, err)
			}
		}
	}
	return nil
}

func (d *Document) RelationNodes(relation Element) (Element, Element, error) {
	if relation.Kind != "arrow" {
		return Element{}, Element{}, fmt.Errorf("element %q is not an arrow", relation.ID)
	}
	if relation.From == "" || relation.To == "" {
		return Element{}, Element{}, fmt.Errorf("arrow %q needs from and to", relation.ID)
	}
	if relation.From == relation.To {
		return Element{}, Element{}, fmt.Errorf("arrow %q cannot connect a node to itself", relation.ID)
	}
	from, fromOK := findElement(d.Elements, relation.From)
	to, toOK := findElement(d.Elements, relation.To)
	if !fromOK || !toOK {
		return Element{}, Element{}, fmt.Errorf("arrow %q points to a missing element", relation.ID)
	}
	if from.Kind != "box" || to.Kind != "box" {
		return Element{}, Element{}, fmt.Errorf("arrow %q must connect boxes", relation.ID)
	}
	return from, to, nil
}

func (d *Document) FindByID(id string) (Element, bool) {
	return findElement(d.Elements, id)
}

func (d *Document) HasArrow(from, to string) bool {
	for _, element := range d.Elements {
		if element.Kind == "arrow" && element.From == from && element.To == to {
			return true
		}
	}
	return false
}

func (d *Document) NextID() int {
	max := 0
	for _, element := range d.Elements {
		parts := strings.Split(element.ID, "-")
		if len(parts) != 2 {
			continue
		}
		n, err := strconv.Atoi(parts[1])
		if err == nil && n >= max {
			max = n + 1
		}
	}
	return max
}

func ValidateReference(root string, ref Reference) error {
	return validateReference(root, ref)
}

func validateReference(root string, ref Reference) error {
	if strings.TrimSpace(ref.Path) == "" {
		return errors.New("reference path cannot be empty")
	}
	path, err := repoPath(root, filepath.FromSlash(ref.Path))
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("reference path does not exist: %s", ref.Path)
	}
	if ref.Line == nil {
		return nil
	}
	if *ref.Line < 1 {
		return errors.New("reference line must be 1 or greater")
	}
	if info.IsDir() {
		return errors.New("reference line cannot target a directory")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	lines := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if lines == 0 {
		lines = 1
	}
	if *ref.Line > lines {
		return fmt.Errorf("reference line %d is beyond file (%d lines)", *ref.Line, lines)
	}
	return nil
}

func findElement(elements []Element, id string) (Element, bool) {
	for _, element := range elements {
		if element.ID == id {
			return element, true
		}
	}
	return Element{}, false
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
