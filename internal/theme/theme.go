package theme

import (
	"fmt"
	"os"
	"strings"
)

// Theme controls generated HTML, CSS, and navigation behavior.
type Theme struct {
	Name       string `yaml:"name"`
	Mode       string `yaml:"mode"`
	FontFamily string `yaml:"font_family"`
	Accent     string `yaml:"accent"`
	MaxWidth   string `yaml:"max_width"`
	Flair      string `yaml:"flair"`
}

const (
	ModeStatic = "static"
	ModeSPA    = "spa"
)

var builtins = map[string]string{
	"minimal": `name: minimal
mode: static
font_family: system
accent: "#000000"
max_width: 72rem
flair: none
`,
	"terminal": `name: terminal
mode: spa
font_family: mono
accent: "#000000"
max_width: 74rem
flair: terminal-rule
`,
	"editorial": `name: editorial
mode: static
font_family: serif
accent: "#000000"
max_width: 68rem
flair: none
`,
	"cmd-bio": `name: cmd-bio
mode: spa
font_family: system
accent: "#000000"
max_width: 76rem
flair: terminal-rule
`,
}

// Load reads a built-in theme name or a theme YAML file path.
func Load(nameOrPath string) (Theme, error) {
	raw, ok := builtins[nameOrPath]
	if !ok {
		b, err := os.ReadFile(nameOrPath)
		if err != nil {
			return Theme{}, fmt.Errorf("load theme %q: %w", nameOrPath, err)
		}
		raw = string(b)
	}

	th := parseYAML(raw)
	if th.Name == "" {
		return Theme{}, fmt.Errorf("theme %q missing name", nameOrPath)
	}
	if th.Mode == "" {
		th.Mode = ModeStatic
	}
	if th.Mode != ModeStatic && th.Mode != ModeSPA {
		return Theme{}, fmt.Errorf("theme %q has unsupported mode %q", nameOrPath, th.Mode)
	}
	if th.FontFamily == "" {
		th.FontFamily = "system"
	}
	if th.Accent == "" {
		th.Accent = "#000000"
	}
	if th.MaxWidth == "" {
		th.MaxWidth = "72rem"
	}
	return th, nil
}

func parseYAML(raw string) Theme {
	var th Theme
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch strings.TrimSpace(key) {
		case "name":
			th.Name = value
		case "mode":
			th.Mode = value
		case "font_family":
			th.FontFamily = value
		case "accent":
			th.Accent = value
		case "max_width":
			th.MaxWidth = value
		case "flair":
			th.Flair = value
		}
	}
	return th
}
