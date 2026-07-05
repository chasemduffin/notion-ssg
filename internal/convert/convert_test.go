package convert

import (
	"strings"
	"testing"

	"github.com/chasemduffin/notion-ssg/internal/notion"
)

func TestBlocksToHTMLConvertsCommonBlocks(t *testing.T) {
	html := BlocksToHTML([]notion.Block{
		{Type: "heading_1", Heading1: &notion.RichTextBlock{RichText: rt("Title")}},
		{Type: "paragraph", Paragraph: &notion.RichTextBlock{RichText: []notion.RichText{{PlainText: "Hello ", Annotations: notion.Annotations{Bold: true}}, {PlainText: "world"}}}},
		{Type: "to_do", ToDo: &notion.ToDoBlock{RichText: rt("Done"), Checked: true}},
		{Type: "code", Code: &notion.CodeBlock{RichText: rt("fmt.Println(1)"), Language: "go"}},
	}, func(title string) string { return "x.html" })

	for _, want := range []string{"<h1>Title</h1>", "<strong>Hello </strong>world", "checked", "language-go"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in %s", want, html)
		}
	}
}

func TestBlocksToHTMLRendersDatabase(t *testing.T) {
	got := BlocksToHTML([]notion.Block{{
		Type:          "child_database",
		ChildDatabase: &notion.ChildPageBlock{Title: "Posts"},
		DatabaseRows:  []notion.DatabaseRow{{Title: "First", Properties: map[string]string{"Status": "Draft"}}},
	}}, func(string) string { return "" })
	if !strings.Contains(got, "<table>") || !strings.Contains(got, "First") || !strings.Contains(got, "Draft") {
		t.Fatalf("unexpected database HTML: %s", got)
	}
}

func rt(text string) []notion.RichText { return []notion.RichText{{PlainText: text}} }
