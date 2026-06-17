package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	desktopEntrySection       = "Desktop Entry"
	desktopActionSectionStart = "Desktop Action "
)

type desktopKey struct {
	name      string
	value     string
	lineIndex int
}

type desktopSection struct {
	name      string
	lineIndex int
	keys      []desktopKey
}

func (s *desktopSection) get(name string) (string, bool) {
	for _, key := range s.keys {
		if key.name == name {
			return key.value, true
		}
	}
	return "", false
}

type desktopFile struct {
	lines           []string
	sections        []desktopSection
	trailingNewline bool
}

func findDesktopFile(extracted string) (*desktopFile, error) {
	candidates, err := filepath.Glob(filepath.Join(extracted, "*.desktop"))
	if err != nil {
		return nil, err
	}
	sort.Strings(candidates)

	for _, candidate := range candidates {
		if filepath.Base(candidate) == "default.desktop" {
			continue
		}
		desktop, err := readDesktopFile(candidate)
		if err != nil {
			return nil, err
		}
		return desktop, nil
	}

	return nil, nil
}

func readDesktopFile(path string) (*desktopFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	text := string(data)
	body := strings.TrimSuffix(text, "\n")
	lines := []string{}
	if body != "" {
		lines = strings.Split(body, "\n")
	}

	desktop := &desktopFile{
		lines:           lines,
		trailingNewline: strings.HasSuffix(text, "\n"),
	}

	var current *desktopSection
	for index, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			desktop.sections = append(desktop.sections, desktopSection{
				name:      strings.TrimSpace(line[1 : len(line)-1]),
				lineIndex: index,
			})
			current = &desktop.sections[len(desktop.sections)-1]
			continue
		}

		if current == nil || !strings.Contains(line, "=") {
			continue
		}

		name, value, _ := strings.Cut(line, "=")
		current.keys = append(current.keys, desktopKey{
			name:      strings.TrimSpace(name),
			value:     strings.TrimSpace(value),
			lineIndex: index,
		})
	}

	return desktop, nil
}

func (d *desktopFile) section(name string) *desktopSection {
	if section := d.findSection(name); section != nil {
		return section
	}

	if len(d.lines) > 0 && strings.TrimSpace(d.lines[len(d.lines)-1]) != "" {
		d.lines = append(d.lines, "")
	}

	lineIndex := len(d.lines)
	d.lines = append(d.lines, "["+name+"]")
	d.sections = append(d.sections, desktopSection{name: name, lineIndex: lineIndex})
	return &d.sections[len(d.sections)-1]
}

func (d *desktopFile) findSection(name string) *desktopSection {
	for i := range d.sections {
		if d.sections[i].name == name {
			return &d.sections[i]
		}
	}
	return nil
}

func (d *desktopFile) get(name string, sectionName string) (string, bool) {
	section := d.findSection(sectionName)
	if section == nil {
		return "", false
	}
	return section.get(name)
}

func (d *desktopFile) set(name string, value string, sectionName string) {
	section := d.section(sectionName)

	for i := range section.keys {
		if section.keys[i].name == name {
			section.keys[i].value = value
			d.lines[section.keys[i].lineIndex] = section.keys[i].name + "=" + value
			return
		}
	}

	insertAt := len(d.lines)
	for _, other := range d.sections {
		if other.lineIndex > section.lineIndex && other.lineIndex < insertAt {
			insertAt = other.lineIndex
		}
	}

	d.insertLine(insertAt, name+"="+value)
	section.keys = append(section.keys, desktopKey{
		name:      name,
		value:     value,
		lineIndex: insertAt,
	})
}

func (d *desktopFile) insertLine(index int, line string) {
	d.lines = append(d.lines, "")
	copy(d.lines[index+1:], d.lines[index:])
	d.lines[index] = line

	for i := range d.sections {
		if d.sections[i].lineIndex >= index {
			d.sections[i].lineIndex++
		}
		for j := range d.sections[i].keys {
			if d.sections[i].keys[j].lineIndex >= index {
				d.sections[i].keys[j].lineIndex++
			}
		}
	}
}

func (d *desktopFile) write(path string) error {
	text := strings.Join(d.lines, "\n")
	if d.trailingNewline {
		text += "\n"
	}
	return writeAtomic(path, 0o644, func(w io.Writer) error {
		_, err := w.Write([]byte(text))
		return err
	})
}

func (a app) patchDesktopFile(desktop *desktopFile, appName string, hasIcon bool) error {
	home, err := a.homeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	appimages := filepath.Join(home, "AppImages")
	appimage := filepath.Join(appimages, appName+".appimage")

	if hasIcon {
		desktop.set("Icon", filepath.Join(appimages, ".icons", appName), desktopEntrySection)
	}
	desktop.set("TryExec", appimage, desktopEntrySection)

	if execCmd, ok := desktop.get("Exec", desktopEntrySection); ok {
		desktop.set("Exec", patchExecCommand(execCmd, appimage), desktopEntrySection)
	}

	for _, section := range desktop.sections {
		if !strings.HasPrefix(section.name, desktopActionSectionStart) {
			continue
		}
		if execCmd, ok := section.get("Exec"); ok {
			desktop.set("Exec", patchExecCommand(execCmd, appimage), section.name)
		}
	}

	return nil
}

func (a app) installDesktopFile(desktop *desktopFile, appName string) (string, error) {
	home, err := a.homeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	dest := filepath.Join(home, ".local", "share", "applications", appName+".desktop")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("create desktop file directory %s: %w", filepath.Dir(dest), err)
	}
	if err := desktop.write(dest); err != nil {
		return "", fmt.Errorf("write desktop file %s: %w", dest, err)
	}
	return dest, nil
}
