package config

import (
	"errors"
	"flag"
	"fmt"
)

// Config contains the CLI inputs needed to generate a site.
type Config struct {
	NotionPAT string
	Theme     string
	NavRoot   string
	Output    string
}

// Parse reads command-line flags and environment variables.
func Parse(args []string, getenv func(string) string) (Config, error) {
	fs := flag.NewFlagSet("notion-ssg", flag.ContinueOnError)
	fs.SetOutput(nil)

	var cfg Config
	fs.StringVar(&cfg.NotionPAT, "notion-pat", "", "Notion Personal Access Token; defaults to $NOTION_PAT")
	fs.StringVar(&cfg.Theme, "theme", "minimal", "Built-in theme name or path to theme.yaml")
	fs.StringVar(&cfg.NavRoot, "nav-root", "", "Notion page title to use as the site root")
	fs.StringVar(&cfg.Output, "output", "", "Output directory for generated static files")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if cfg.NotionPAT == "" && getenv != nil {
		cfg.NotionPAT = getenv("NOTION_PAT")
	}

	if cfg.NotionPAT == "" {
		return Config{}, errors.New("missing Notion token: pass --notion-pat or set NOTION_PAT")
	}
	if cfg.NavRoot == "" {
		return Config{}, errors.New("missing --nav-root")
	}
	if cfg.Output == "" {
		return Config{}, errors.New("missing --output")
	}
	if cfg.Theme == "" {
		return Config{}, fmt.Errorf("missing --theme")
	}

	return cfg, nil
}
