package site

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasemduffin/notion-ssg/internal/notion"
	"github.com/chasemduffin/notion-ssg/internal/theme"
)

type fakeClient struct{}

func (fakeClient) FindPageByTitle(context.Context, string) (notion.PageRef, error) {
	return notion.PageRef{ID: "root", Title: "cmd.bio"}, nil
}
func (fakeClient) Children(_ context.Context, id string) ([]notion.Block, error) {
	switch id {
	case "root":
		return []notion.Block{
			{ID: "about", Type: "child_page", ChildPage: &notion.ChildPageBlock{Title: "About"}},
			{ID: "db", Type: "child_database", ChildDatabase: &notion.ChildPageBlock{Title: "Posts"}},
			{Type: "paragraph", Paragraph: &notion.RichTextBlock{RichText: []notion.RichText{{PlainText: "[github](https://github.com/chasemduffin)"}}}},
		}, nil
	case "about":
		return []notion.Block{{Type: "paragraph", Paragraph: &notion.RichTextBlock{RichText: []notion.RichText{{PlainText: "Hello"}}}}}, nil
	case "row-1":
		return []notion.Block{{Type: "paragraph", Paragraph: &notion.RichTextBlock{RichText: []notion.RichText{{PlainText: "Post body"}}}}}, nil
	default:
		return nil, nil
	}
}
func (fakeClient) QueryDatabase(context.Context, string) ([]notion.DatabaseRow, error) {
	return []notion.DatabaseRow{{ID: "row-1", Title: "Post 1", Properties: map[string]string{"Status": "Published"}}}, nil
}

func TestBuildAndWriteSPASite(t *testing.T) {
	generator := Generator{Client: fakeClient{}, Theme: theme.Theme{Name: "cmd-bio", Mode: theme.ModeSPA, FontFamily: "system", Accent: "#000", MaxWidth: "76rem"}}
	built, err := generator.Build(context.Background(), "cmd.bio")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(built.Pages) != 4 {
		t.Fatalf("pages = %d", len(built.Pages))
	}
	if len(built.NavItems) != 3 {
		t.Fatalf("nav items = %d", len(built.NavItems))
	}
	if strings.Contains(built.Pages[0].HTML, "Post body") {
		t.Fatalf("homepage should not flatten child content: %s", built.Pages[0].HTML)
	}
	if built.NavItems[2].Href != "https://github.com/chasemduffin" {
		t.Fatalf("external nav href = %q", built.NavItems[2].Href)
	}

	dir := t.TempDir()
	if err := built.Write(dir); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	for _, name := range []string{"index.html", "about/index.html", "posts/index.html", "posts/post-1/index.html", "styles.css", "app.js"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "data-internal") || strings.Contains(string(raw), "<h1>cmd.bio</h1>") {
		t.Fatalf("unexpected index output: %s", string(raw))
	}
	row, err := os.ReadFile(filepath.Join(dir, "posts/post-1/index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(row), "Post body") {
		t.Fatalf("database row page missing content: %s", string(row))
	}
}
