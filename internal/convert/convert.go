package convert

import (
	"bytes"
	"html"
	"sort"
	"strings"

	"github.com/chasemduffin/notion-ssg/internal/notion"
)

// BlocksToHTML converts supported Notion blocks into static HTML.
func BlocksToHTML(blocks []notion.Block, slugForTitle func(string) string) string {
	var b bytes.Buffer
	for _, block := range blocks {
		writeBlock(&b, block, slugForTitle)
	}
	return b.String()
}

func writeBlock(b *bytes.Buffer, block notion.Block, slugForTitle func(string) string) {
	switch block.Type {
	case "paragraph":
		if block.Paragraph != nil {
			text := plain(block.Paragraph.RichText)
			if strings.TrimSpace(text) == "" || isMarkdownImageMarker(text) {
				break
			}
			if text := richText(block.Paragraph.RichText); text != "" {
				b.WriteString("<p>" + text + "</p>\n")
			}
		}
	case "column_list":
		b.WriteString("<div class=\"columns\">\n")
		for _, child := range block.Children {
			writeBlock(b, child, slugForTitle)
		}
		b.WriteString("</div>\n")
		return
	case "column":
		b.WriteString("<div class=\"column\">\n")
		for _, child := range block.Children {
			writeBlock(b, child, slugForTitle)
		}
		b.WriteString("</div>\n")
		return
	case "table_of_contents":
		return
	case "heading_1":
		b.WriteString("<h1>" + richText(block.Heading1.RichText) + "</h1>\n")
	case "heading_2":
		b.WriteString("<h2>" + richText(block.Heading2.RichText) + "</h2>\n")
	case "heading_3":
		b.WriteString("<h3>" + richText(block.Heading3.RichText) + "</h3>\n")
	case "bulleted_list_item":
		b.WriteString("<ul><li>" + richText(block.BulletedListItem.RichText) + "</li></ul>\n")
	case "numbered_list_item":
		b.WriteString("<ol><li>" + richText(block.NumberedListItem.RichText) + "</li></ol>\n")
	case "to_do":
		checked := ""
		if block.ToDo.Checked {
			checked = " checked"
		}
		b.WriteString("<label class=\"todo\"><input type=\"checkbox\" disabled" + checked + "> <span>" + richText(block.ToDo.RichText) + "</span></label>\n")
	case "quote":
		b.WriteString("<blockquote>" + richText(block.Quote.RichText) + "</blockquote>\n")
	case "callout":
		b.WriteString("<aside class=\"callout\">" + richText(block.Callout.RichText) + "</aside>\n")
	case "code":
		lang := html.EscapeString(block.Code.Language)
		b.WriteString("<pre><code class=\"language-" + lang + "\">" + html.EscapeString(plain(block.Code.RichText)) + "</code></pre>\n")
	case "divider":
		b.WriteString("<hr>\n")
	case "image":
		writeFileBlock(b, "img", block.Image)
	case "file":
		writeFileBlock(b, "file", block.File)
	case "bookmark":
		url := html.EscapeString(block.Bookmark.URL)
		label := richText(block.Bookmark.Caption)
		if label == "" {
			label = url
		}
		b.WriteString("<p><a class=\"bookmark\" href=\"" + url + "\">" + label + "</a></p>\n")
	case "child_page":
		title := block.ChildPage.Title
		b.WriteString("<p><a href=\"" + html.EscapeString(slugForTitle(title)) + "\" data-internal>" + html.EscapeString(title) + "</a></p>\n")
	case "child_database":
		writeDatabase(b, block)
	default:
		b.WriteString("<p class=\"unsupported\">Unsupported Notion block: " + html.EscapeString(block.Type) + "</p>\n")
	}

	if len(block.Children) > 0 {
		b.WriteString(BlocksToHTML(block.Children, slugForTitle))
	}
}

func writeFileBlock(b *bytes.Buffer, kind string, file *notion.FileBlock) {
	if file == nil {
		return
	}
	url := ""
	if file.External != nil {
		url = file.External.URL
	}
	if url == "" && file.File != nil {
		url = file.File.URL
	}
	if url == "" {
		return
	}
	safeURL := html.EscapeString(url)
	caption := richText(file.Caption)
	if kind == "img" {
		alt := stripTags(caption)
		b.WriteString("<figure><img src=\"" + safeURL + "\" alt=\"" + html.EscapeString(alt) + "\">")
		if caption != "" {
			b.WriteString("<figcaption>" + caption + "</figcaption>")
		}
		b.WriteString("</figure>\n")
		return
	}
	label := caption
	if label == "" {
		label = safeURL
	}
	b.WriteString("<p><a href=\"" + safeURL + "\">" + label + "</a></p>\n")
}

func writeDatabase(b *bytes.Buffer, block notion.Block) {
	title := "Database"
	if block.ChildDatabase != nil && block.ChildDatabase.Title != "" {
		title = block.ChildDatabase.Title
	}
	b.WriteString("<section class=\"database\"><h2>" + html.EscapeString(title) + "</h2>")
	if len(block.DatabaseRows) == 0 {
		b.WriteString("<p>No rows.</p></section>\n")
		return
	}
	keys := map[string]bool{}
	for _, row := range block.DatabaseRows {
		for key, value := range row.Properties {
			if value != "" {
				keys[key] = true
			}
		}
	}
	columns := make([]string, 0, len(keys))
	for key := range keys {
		columns = append(columns, key)
	}
	sort.Strings(columns)
	b.WriteString("<div class=\"table-wrap\"><table><thead><tr><th>Name</th>")
	for _, col := range columns {
		b.WriteString("<th>" + html.EscapeString(col) + "</th>")
	}
	b.WriteString("</tr></thead><tbody>")
	for _, row := range block.DatabaseRows {
		b.WriteString("<tr><td>" + html.EscapeString(row.Title) + "</td>")
		for _, col := range columns {
			b.WriteString("<td>" + html.EscapeString(row.Properties[col]) + "</td>")
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table></div></section>\n")
}

func richText(parts []notion.RichText) string {
	var b strings.Builder
	for _, part := range parts {
		text := html.EscapeString(part.PlainText)
		if part.Annotations.Code {
			text = "<code>" + text + "</code>"
		}
		if part.Annotations.Bold {
			text = "<strong>" + text + "</strong>"
		}
		if part.Annotations.Italic {
			text = "<em>" + text + "</em>"
		}
		if part.Annotations.Underline {
			text = "<u>" + text + "</u>"
		}
		if part.Annotations.Strikethrough {
			text = "<s>" + text + "</s>"
		}
		if part.Href != "" {
			text = "<a href=\"" + html.EscapeString(part.Href) + "\">" + text + "</a>"
		}
		b.WriteString(text)
	}
	return b.String()
}

func plain(parts []notion.RichText) string {
	var b strings.Builder
	for _, part := range parts {
		b.WriteString(part.PlainText)
	}
	return b.String()
}

func stripTags(s string) string {
	replacer := strings.NewReplacer("<", "", ">", "")
	return replacer.Replace(s)
}

func isMarkdownImageMarker(text string) bool {
	text = strings.TrimSpace(text)
	return strings.HasPrefix(text, "[image](") && strings.HasSuffix(text, ")")
}
