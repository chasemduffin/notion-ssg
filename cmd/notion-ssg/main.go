package main

import (
	"context"
	"fmt"
	"os"

	"github.com/chasemduffin/notion-ssg/internal/config"
	"github.com/chasemduffin/notion-ssg/internal/notion"
	"github.com/chasemduffin/notion-ssg/internal/site"
	"github.com/chasemduffin/notion-ssg/internal/theme"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "notion-ssg: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Parse(args, os.Getenv)
	if err != nil {
		return err
	}

	th, err := theme.Load(cfg.Theme)
	if err != nil {
		return err
	}

	client := notion.NewHTTPClient(cfg.NotionPAT)
	generator := site.Generator{Client: client, Theme: th}
	generated, err := generator.Build(context.Background(), cfg.NavRoot)
	if err != nil {
		return err
	}

	return generated.Write(cfg.Output)
}
