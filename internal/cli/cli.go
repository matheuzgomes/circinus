package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"circinus/internal/document"
	"circinus/internal/editor"
)

func Run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printHelp()
		return nil
	}

	root, err := repositoryRoot()
	if err != nil {
		return err
	}

	switch args[0] {
	case "new":
		if len(args) != 2 {
			return errors.New("usage: circinus new <file.diagram.json>")
		}
		return newDocument(root, args[1])
	case "list":
		if len(args) != 1 {
			return errors.New("usage: circinus list")
		}
		return listDocuments(root)
	case "edit":
		if len(args) != 2 {
			return errors.New("usage: circinus edit <file.diagram.json>")
		}
		return editor.EditDocument(root, args[1])
	case "check":
		if len(args) != 2 {
			return errors.New("usage: circinus check <file.diagram.json>")
		}
		return checkDocument(root, args[1])
	default:
		return fmt.Errorf("unknown command %q; use --help", args[0])
	}
}

func printHelp() {
	fmt.Println(`Circinus — terminal architecture diagrams

Usage:
  circinus new architecture.diagram.json
  circinus list
  circinus edit architecture.diagram.json
  circinus check architecture.diagram.json`)
}

func repositoryRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return cwd, nil
		}
	}
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

func newDocument(root, input string) error {
	if !strings.HasSuffix(input, ".diagram.json") {
		return errors.New("native files must end with .diagram.json")
	}
	path, err := repoPath(root, input)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists: %s", input)
	} else if !os.IsNotExist(err) {
		return err
	}
	doc := document.New()
	if err := doc.Save(path); err != nil {
		return err
	}
	fmt.Println("created", input)
	return nil
}

func listDocuments(root string) error {
	paths, err := findDocuments(root)
	if err != nil {
		return err
	}
	for _, path := range paths {
		fmt.Println(path)
	}
	return nil
}

func findDocuments(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".diagram.json") {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func checkDocument(root, input string) error {
	path, err := repoPath(root, input)
	if err != nil {
		return err
	}
	doc, err := document.Load(path)
	if err != nil {
		return err
	}
	if err := doc.Validate(root); err != nil {
		return fmt.Errorf("%s: %w", input, err)
	}
	for _, warning := range documentWarnings(doc.Elements) {
		fmt.Printf("warning: %s\n", warning)
	}
	fmt.Println("ok", input)
	return nil
}

func documentWarnings(elements []document.Element) []string {
	warnings := []string{}
	for i, left := range elements {
		if left.Kind != "box" {
			continue
		}
		for _, right := range elements[i+1:] {
			if right.Kind == "box" && boxesOverlap(left, right) {
				warnings = append(warnings, fmt.Sprintf("nodes %q and %q overlap", left.ID, right.ID))
			}
		}
	}
	return warnings
}

func boxesOverlap(left, right document.Element) bool {
	return left.X < right.X+right.W && left.X+left.W > right.X && left.Y < right.Y+right.H && left.Y+left.H > right.Y
}
