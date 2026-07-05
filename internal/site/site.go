package site

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/chasemduffin/notion-ssg/internal/convert"
	"github.com/chasemduffin/notion-ssg/internal/notion"
	"github.com/chasemduffin/notion-ssg/internal/theme"
)

type Generator struct {
	Client notion.Client
	Theme  theme.Theme
}

type Site struct {
	Theme    theme.Theme
	Pages    []Page
	NavItems []NavItem
}

type Page struct {
	Title     string
	Path      string
	Href      string
	HTML      string
	HideTitle bool
}

type NavItem struct {
	Title string
	Href  string
	HTML  string
}

type Module string

const (
	ModuleDefault Module = "default"
	ModuleBlog    Module = "blog"
	ModuleGallery Module = "gallery"
)

func (g Generator) Build(ctx context.Context, navRoot string) (Site, error) {
	root, err := g.Client.FindPageByTitle(ctx, navRoot)
	if err != nil {
		return Site{}, err
	}
	blocks, err := g.hydrate(ctx, root.ID)
	if err != nil {
		return Site{}, err
	}

	pages := []Page{{Title: root.Title, Path: "index.html", Href: "/", HideTitle: true}}
	var rootBlocks []notion.Block
	navItems := []NavItem{}

	for i := 0; i < len(blocks); i++ {
		block := blocks[i]
		if block.Type == "child_page" && block.ChildPage != nil {
			segment := slugSegment(block.ChildPage.Title)
			mod := moduleForTitle(block.ChildPage.Title)
			pageHTML, rowPages := g.renderBlocks(ctx, block.Children, segment, mod)
			if mod == ModuleGallery {
				pageHTML = ""
				rowPages = nil
			}
			page := Page{Title: block.ChildPage.Title, Path: filepath.Join(segment, "index.html"), Href: "/" + segment + "/", HTML: pageHTML}
			pages = append(pages, page)
			pages = append(pages, rowPages...)
			navItems = append(navItems, NavItem{Title: page.Title, Href: page.Href})
			continue
		}
		if block.Type == "child_database" && block.ChildDatabase != nil {
			segment := slugSegment(block.ChildDatabase.Title)
			page, rowPages := g.databasePages(ctx, block, segment, ModuleDefault)
			pages = append(pages, page)
			pages = append(pages, rowPages...)
			navItems = append(navItems, NavItem{Title: page.Title, Href: page.Href})
			continue
		}
		if marker, ok := imageNavMarker(block); ok {
			item := NavItem{Title: marker.Title, Href: marker.Href}
			if i+1 < len(blocks) && blocks[i+1].Type == "image" {
				item.HTML = imageNavHTML(marker.Title, marker.Href, blocks[i+1])
				i++
			}
			navItems = append(navItems, item)
			continue
		}
		rootBlocks = append(rootBlocks, block)
	}

	rootHTML, rootRowPages := g.renderBlocks(ctx, rootBlocks, "", ModuleDefault)
	pages[0].HTML = rootHTML
	pages = append(pages, rootRowPages...)

	return Site{Theme: g.Theme, Pages: pages, NavItems: navItems}, nil
}

func moduleForTitle(title string) Module {
	switch strings.ToLower(strings.TrimSpace(title)) {
	case "blog":
		return ModuleBlog
	case "gallery":
		return ModuleGallery
	default:
		return ModuleDefault
	}
}

func (g Generator) renderBlocks(ctx context.Context, blocks []notion.Block, parentPath string, mod Module) (string, []Page) {
	var b bytes.Buffer
	var rowPages []Page
	for _, block := range blocks {
		if block.Type == "child_database" && block.ChildDatabase != nil {
			page, rows := g.databasePages(ctx, block, databaseBasePath(parentPath, block.ChildDatabase.Title), mod)
			b.WriteString(page.HTML)
			rowPages = append(rowPages, rows...)
			continue
		}
		b.WriteString(convert.BlocksToHTML([]notion.Block{block}, slugForTitle))
	}
	return b.String(), rowPages
}

func (g Generator) databasePages(ctx context.Context, block notion.Block, basePath string, mod Module) (Page, []Page) {
	title := block.ChildDatabase.Title
	page := Page{Title: title, Path: filepath.Join(basePath, "index.html"), Href: "/" + basePath + "/"}
	rows := rowsForModule(block.DatabaseRows, mod)
	page.HTML = databaseIndexHTML(title, basePath, rows)

	rowPages := make([]Page, 0, len(rows))
	for _, row := range rows {
		rowPath := pathJoinURL(basePath, slugSegment(row.Title))
		rowHTML := ""
		if row.ID != "" {
			if children, err := g.hydrate(ctx, row.ID); err == nil {
				rowHTML, _ = g.renderBlocks(ctx, children, rowPath, mod)
			}
		}
		if rowHTML == "" {
			rowHTML = rowPropertiesHTML(row)
		}
		rowPages = append(rowPages, Page{Title: row.Title, Path: filepath.Join(rowPath, "index.html"), Href: "/" + rowPath + "/", HTML: rowHTML})
	}
	return page, rowPages
}

func rowsForModule(rows []notion.DatabaseRow, mod Module) []notion.DatabaseRow {
	if mod != ModuleBlog {
		return rows
	}
	filtered := make([]notion.DatabaseRow, 0, len(rows))
	for _, row := range rows {
		if strings.EqualFold(strings.TrimSpace(row.Properties["Status"]), "Published") {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func (g Generator) hydrate(ctx context.Context, blockID string) ([]notion.Block, error) {
	blocks, err := g.Client.Children(ctx, blockID)
	if err != nil {
		return nil, err
	}
	for i := range blocks {
		if blocks[i].HasChildren {
			children, err := g.hydrate(ctx, blocks[i].ID)
			if err != nil {
				return nil, err
			}
			blocks[i].Children = children
		}
		if blocks[i].Type == "child_database" {
			rows, err := g.Client.QueryDatabase(ctx, blocks[i].ID)
			if err != nil {
				return nil, err
			}
			blocks[i].DatabaseRows = rows
		}
	}
	return blocks, nil
}

func (s Site) Write(output string) error {
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	if err := cleanOutput(output); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "styles.css"), []byte(s.CSS()), 0o644); err != nil {
		return err
	}
	if s.Theme.Mode == theme.ModeSPA {
		if err := os.WriteFile(filepath.Join(output, "app.js"), []byte(appJS), 0o644); err != nil {
			return err
		}
	}
	assetCache := map[string]string{}
	for _, page := range s.Pages {
		rendered, err := localizeAssets(output, s.RenderPage(page), assetCache)
		if err != nil {
			return err
		}
		outputPath := filepath.Join(output, page.Path)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, []byte(rendered), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func cleanOutput(output string) error {
	entries, err := os.ReadDir(output)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(output, name)
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
			continue
		}
		switch {
		case strings.HasSuffix(name, ".html"), name == "styles.css", name == "app.js":
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func localizeAssets(output, rendered string, cache map[string]string) (string, error) {
	re := regexp.MustCompile(`src="(https?://[^"]+)"`)
	matches := re.FindAllStringSubmatch(rendered, -1)
	for _, match := range matches {
		encodedURL := match[1]
		rawURL := html.UnescapeString(encodedURL)
		local, ok := cache[rawURL]
		if !ok {
			var err error
			local, err = downloadAsset(output, rawURL)
			if err != nil {
				return "", err
			}
			cache[rawURL] = local
		}
		rendered = strings.ReplaceAll(rendered, encodedURL, local)
	}
	return rendered, nil
}

func downloadAsset(output, rawURL string) (string, error) {
	resp, err := http.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download asset %s: %s", rawURL, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(output, "assets"), 0o755); err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte(assetHashInput(rawURL)))
	ext := assetExt(rawURL, resp.Header.Get("Content-Type"))
	name := hex.EncodeToString(sum[:])[:16] + ext
	localPath := filepath.Join("assets", name)
	if err := os.WriteFile(filepath.Join(output, localPath), body, 0o644); err != nil {
		return "", err
	}
	return "/" + filepath.ToSlash(localPath), nil
}

func assetHashInput(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return parsed.Host + parsed.Path
}

func assetExt(rawURL, contentType string) string {
	if contentType != "" {
		if exts, err := mime.ExtensionsByType(strings.Split(contentType, ";")[0]); err == nil && len(exts) > 0 {
			return exts[0]
		}
	}
	parsed, err := url.Parse(rawURL)
	if err == nil {
		if ext := filepath.Ext(parsed.Path); ext != "" {
			return ext
		}
	}
	return ".bin"
}

func (s Site) RenderPage(page Page) string {
	var b bytes.Buffer
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<title>" + html.EscapeString(page.Title) + "</title>\n")
	b.WriteString("<link rel=\"stylesheet\" href=\"/styles.css\">\n")
	if s.Theme.Mode == theme.ModeSPA {
		b.WriteString("<script defer src=\"/app.js\"></script>\n")
	}
	b.WriteString("</head>\n<body>\n<a class=\"skip-link\" href=\"#content\">Skip to content</a>\n")
	b.WriteString("<header class=\"site-header\"><a class=\"brand\" href=\"/\" data-internal>" + html.EscapeString(s.Pages[0].Title) + "</a><nav aria-label=\"Primary\">")
	for _, nav := range s.NavItems {
		current := ""
		if nav.Href == page.Href || (nav.Href != "/" && strings.HasPrefix(page.Href, nav.Href)) {
			current = " aria-current=\"page\""
		}
		internal := ""
		if isInternalHref(nav.Href) {
			internal = " data-internal"
		}
		label := html.EscapeString(nav.Title)
		if nav.HTML != "" {
			label = nav.HTML
		}
		b.WriteString("<a href=\"" + html.EscapeString(nav.Href) + "\"" + internal + current + ">" + label + "</a>")
	}
	b.WriteString("</nav></header>\n")
	b.WriteString("<main id=\"content\" class=\"content\" tabindex=\"-1\">\n")
	if !page.HideTitle && !page.hasHeading() {
		b.WriteString("<h1>" + html.EscapeString(page.Title) + "</h1>\n")
	}
	b.WriteString(page.HTML)
	b.WriteString("</main>\n<footer><span>made with ☕ 💌 🤝 by <a href=\"https://github.com/chasemduffin/notion-ssg\">Notion Static Site Generator</a></span></footer>\n</body>\n</html>\n")
	return b.String()
}

func (p Page) hasHeading() bool {
	return strings.HasPrefix(strings.TrimSpace(p.HTML), "<h1")
}

type imageMarker struct {
	Title string
	Href  string
}

func imageNavMarker(block notion.Block) (imageMarker, bool) {
	if block.Type != "paragraph" || block.Paragraph == nil {
		return imageMarker{}, false
	}
	text := strings.TrimSpace(plainText(block.Paragraph.RichText))
	re := regexp.MustCompile(`^(?:<img>)?\[([^\]]+)\]\(([^)]+)\)$`)
	match := re.FindStringSubmatch(text)
	if len(match) != 3 {
		return imageMarker{}, false
	}
	return imageMarker{Title: strings.TrimSpace(match[1]), Href: strings.TrimSpace(match[2])}, true
}

func imageNavHTML(title, href string, block notion.Block) string {
	if block.Image == nil {
		return html.EscapeString(title)
	}
	src := fileURL(block.Image)
	if src == "" {
		return html.EscapeString(title)
	}
	return "<img class=\"nav-icon\" src=\"" + html.EscapeString(src) + "\" alt=\"" + html.EscapeString(title) + "\">"
}

func fileURL(file *notion.FileBlock) string {
	if file == nil {
		return ""
	}
	if file.External != nil && file.External.URL != "" {
		return file.External.URL
	}
	if file.File != nil {
		return file.File.URL
	}
	return ""
}

func plainText(parts []notion.RichText) string {
	var b strings.Builder
	for _, part := range parts {
		b.WriteString(part.PlainText)
	}
	return b.String()
}

func databaseIndexHTML(title, basePath string, rows []notion.DatabaseRow) string {
	var b bytes.Buffer
	b.WriteString("<section class=\"database\"><h2>" + html.EscapeString(title) + "</h2>")
	if len(rows) == 0 {
		b.WriteString("<p>No rows.</p></section>\n")
		return b.String()
	}
	keys := databaseColumns(rows)
	tags := databaseTags(rows)
	if len(tags) > 0 {
		b.WriteString("<div class=\"tag-filter\" data-filter-bar aria-label=\"Active tag filters\"></div>")
		b.WriteString("<div class=\"tag-cloud\" aria-label=\"Tag filters\">")
		for _, tag := range tags {
			b.WriteString(tagButton(tag))
		}
		b.WriteString("</div>")
	}
	b.WriteString("<div class=\"table-wrap\"><table class=\"database-table\"><thead><tr><th><button type=\"button\" data-sort=\"text\">Name</button></th>")
	for _, col := range keys {
		sortType := "text"
		if isDateColumn(col, rows) {
			sortType = "date"
		}
		b.WriteString("<th><button type=\"button\" data-sort=\"" + sortType + "\">" + html.EscapeString(col) + "</button></th>")
	}
	b.WriteString("</tr></thead><tbody>")
	for _, row := range rows {
		rowHref := "/" + pathJoinURL(basePath, slugSegment(row.Title)) + "/"
		b.WriteString("<tr data-tags=\"" + html.EscapeString(strings.Join(rowTagIDs(row), " ")) + "\"><td data-sort-value=\"" + html.EscapeString(row.Title) + "\"><a href=\"" + html.EscapeString(rowHref) + "\" data-internal>" + html.EscapeString(row.Title) + "</a></td>")
		for _, col := range keys {
			b.WriteString(renderDatabaseCell(col, row.Properties[col]))
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table></div></section>\n")
	return b.String()
}

func renderDatabaseCell(column, value string) string {
	if isTagColumn(column) {
		var b bytes.Buffer
		b.WriteString("<td data-sort-value=\"" + html.EscapeString(value) + "\">")
		for _, tag := range splitTags(value) {
			b.WriteString(tagButton(tag))
		}
		b.WriteString("</td>")
		return b.String()
	}
	return "<td data-sort-value=\"" + html.EscapeString(value) + "\">" + html.EscapeString(value) + "</td>"
}

func tagButton(tag string) string {
	return "<button type=\"button\" class=\"tag\" data-tag=\"" + html.EscapeString(slugSegment(tag)) + "\" style=\"--tag-hue:" + tagHue(tag) + "\">" + html.EscapeString(tag) + "</button>"
}

func databaseTags(rows []notion.DatabaseRow) []string {
	seen := map[string]bool{}
	var tags []string
	for _, row := range rows {
		for _, tag := range rowTags(row) {
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	sort.Strings(tags)
	return tags
}

func rowTags(row notion.DatabaseRow) []string {
	for key, value := range row.Properties {
		if isTagColumn(key) {
			return splitTags(value)
		}
	}
	return nil
}

func rowTagIDs(row notion.DatabaseRow) []string {
	tags := rowTags(row)
	ids := make([]string, 0, len(tags))
	for _, tag := range tags {
		ids = append(ids, slugSegment(tag))
	}
	return ids
}

func splitTags(value string) []string {
	parts := strings.Split(value, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			tags = append(tags, part)
		}
	}
	return tags
}

func isTagColumn(column string) bool {
	normalized := strings.ToLower(strings.TrimSpace(column))
	return normalized == "tags" || normalized == "tag"
}

func isDateColumn(column string, rows []notion.DatabaseRow) bool {
	name := strings.ToLower(column)
	if strings.Contains(name, "date") {
		return true
	}
	for _, row := range rows {
		value := strings.TrimSpace(row.Properties[column])
		if value == "" {
			continue
		}
		return regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`).MatchString(value)
	}
	return false
}

func tagHue(tag string) string {
	sum := sha1.Sum([]byte(strings.ToLower(tag)))
	hue := int(sum[0])<<8 | int(sum[1])
	return fmt.Sprintf("%d", hue%360)
}

func rowPropertiesHTML(row notion.DatabaseRow) string {
	keys := databaseColumns([]notion.DatabaseRow{row})
	if len(keys) == 0 {
		return ""
	}
	var b bytes.Buffer
	b.WriteString("<dl class=\"properties\">")
	for _, key := range keys {
		b.WriteString("<dt>" + html.EscapeString(key) + "</dt><dd>" + html.EscapeString(row.Properties[key]) + "</dd>")
	}
	b.WriteString("</dl>\n")
	return b.String()
}

func databaseColumns(rows []notion.DatabaseRow) []string {
	keys := map[string]bool{}
	for _, row := range rows {
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
	return columns
}

func isInternalHref(href string) bool {
	return strings.HasPrefix(href, "/")
}

func pathJoinURL(parts ...string) string {
	joined := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part != "" {
			joined = append(joined, part)
		}
	}
	return strings.Join(joined, "/")
}

func databaseBasePath(parentPath, title string) string {
	segment := slugSegment(title)
	parentPath = strings.Trim(parentPath, "/")
	if parentPath == "" {
		return segment
	}
	if lastPathSegment(parentPath) == segment {
		return parentPath
	}
	return pathJoinURL(parentPath, segment)
}

func lastPathSegment(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return parts[len(parts)-1]
}

func (s Site) CSS() string {
	font := `ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`
	switch s.Theme.FontFamily {
	case "mono":
		font = `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace`
	case "serif":
		font = `Iowan Old Style, Apple Garamond, Baskerville, Georgia, serif`
	}
	flair := ""
	if s.Theme.Flair == "terminal-rule" {
		flair = `.brand::before{content:"> ";}.site-header{border-bottom:1px solid var(--fg);}`
	}
	css := `:root{--bg:#fff;--fg:#000;--muted:#555;--line:#d8d8d8;--accent:__ACCENT__;--max:__MAX__;--font:__FONT__;color-scheme:light;}*{box-sizing:border-box;}html{font-size:clamp(16px,1.2vw,19px);}body{margin:0;background:var(--bg);color:var(--fg);font-family:var(--font);line-height:1.55;text-rendering:optimizeLegibility;}a{color:inherit;text-decoration-thickness:.08em;text-underline-offset:.18em;}a:hover{text-decoration-thickness:.14em;}.skip-link{position:absolute;left:-999px;top:1rem;background:var(--fg);color:var(--bg);padding:.5rem .75rem;z-index:10}.skip-link:focus{left:1rem}.site-header{max-width:var(--max);margin:0 auto;padding:clamp(1rem,3vw,2rem);display:flex;align-items:center;justify-content:space-between;gap:1rem}.brand{font-weight:700;text-decoration:none;letter-spacing:-.03em}nav{display:flex;align-items:center;justify-content:flex-end;gap:.45rem;flex-wrap:wrap}nav a{padding:.4rem .55rem;text-decoration:none;border:1px solid transparent;border-radius:999px}nav a:hover,nav a[aria-current="page"]{border-color:var(--fg)}.nav-icon{display:block;width:1.15em;height:1.15em;aspect-ratio:1;object-fit:contain;border-radius:0;filter:grayscale(1)}.content{max-width:var(--max);margin:0 auto;padding:clamp(1.5rem,5vw,4rem) clamp(1rem,3vw,2rem);transition:opacity .18s ease,transform .18s ease}.content.is-transitioning{opacity:0;transform:translateY(.35rem)}h1,h2,h3{line-height:1.1;letter-spacing:-.04em;margin:2.2rem 0 .8rem}h1{font-size:clamp(2.4rem,9vw,6rem);max-width:10ch}h2{font-size:clamp(1.6rem,4vw,2.6rem)}h3{font-size:clamp(1.25rem,3vw,1.7rem)}p,ul,ol,blockquote,pre,figure,.callout,.database,.properties{max-width:68ch}p{margin:0 0 1rem}ul,ol{padding-left:1.4rem}blockquote{margin:1.5rem 0;padding-left:1rem;border-left:2px solid var(--fg);font-size:1.15rem}pre{overflow:auto;padding:1rem;border:1px solid var(--line);border-radius:.75rem;background:#f7f7f7}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:.9em}.todo{display:flex;gap:.5rem;align-items:flex-start;margin:.5rem 0}.callout{border:1px solid var(--fg);border-radius:1rem;padding:1rem;margin:1.25rem 0}hr{border:0;border-top:1px solid var(--line);margin:2rem 0}figure{margin:1.5rem 0}figure>img{width:100%}img{max-width:100%;height:auto;display:block;border-radius:.75rem}figcaption{color:var(--muted);font-size:.9rem;margin-top:.5rem}.columns{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(18rem,100%),1fr));gap:clamp(1rem,3vw,2rem);align-items:start;max-width:100%;margin:1.5rem 0}.column{min-width:0}.column>figure,.column>p{max-width:none;margin-top:0}.column img{width:100%}.tag-cloud,.tag-filter{display:flex;flex-wrap:wrap;gap:.4rem;margin:.75rem 0 1rem}.tag{--tag-hue:0;appearance:none;border:1px solid hsl(var(--tag-hue) 45% 42%);background:hsl(var(--tag-hue) 72% 94%);color:hsl(var(--tag-hue) 45% 24%);border-radius:999px;padding:.18rem .5rem;font:inherit;font-size:.82rem;line-height:1.2;cursor:pointer}.tag:hover,.tag.is-active{background:hsl(var(--tag-hue) 65% 84%)}.tag-clear::after{content:" ×";font-weight:700}.table-wrap{overflow:auto;border:1px solid var(--line);border-radius:.75rem}table{border-collapse:collapse;min-width:100%;font-size:.92rem}th,td{text-align:left;padding:.65rem .75rem;border-bottom:1px solid var(--line);vertical-align:top}th button{appearance:none;border:0;background:transparent;color:inherit;font:inherit;font-weight:700;padding:0;cursor:pointer;text-align:left}th button::after{content:" ↕";color:var(--muted);font-weight:400}.properties{display:grid;grid-template-columns:max-content 1fr;gap:.35rem 1rem}.properties dt{font-weight:700}.properties dd{margin:0}footer{max-width:var(--max);margin:0 auto;padding:2rem;color:var(--muted);font-size:.85rem}.unsupported{color:var(--muted);font-style:italic;}__FLAIR__@media (max-width:720px){.site-header{align-items:flex-start;flex-direction:column}nav{justify-content:flex-start}nav a{padding:.45rem .6rem}.content{padding-top:2rem}h1{max-width:none}}@media (prefers-reduced-motion:reduce){*,*::before,*::after{transition:none!important;scroll-behavior:auto!important}}`
	return strings.NewReplacer(
		"__ACCENT__", s.Theme.Accent,
		"__MAX__", s.Theme.MaxWidth,
		"__FONT__", font,
		"__FLAIR__", flair,
	).Replace(css)
}

func slugForTitle(title string) string {
	s := slugSegment(title)
	if s == "index" {
		return "index.html"
	}
	return s + ".html"
}

func slugSegment(title string) string {
	s := strings.ToLower(strings.TrimSpace(normalizeLatin(title)))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = strings.Trim(re.ReplaceAllString(s, "-"), "-")
	if s == "" {
		return "index"
	}
	return s
}

func normalizeLatin(s string) string {
	return strings.NewReplacer(
		"À", "A", "Á", "A", "Â", "A", "Ã", "A", "Ä", "A", "Å", "A",
		"Æ", "AE", "Ç", "C", "È", "E", "É", "E", "Ê", "E", "Ë", "E",
		"Ì", "I", "Í", "I", "Î", "I", "Ï", "I", "Ð", "D", "Ñ", "N",
		"Ò", "O", "Ó", "O", "Ô", "O", "Õ", "O", "Ö", "O", "Ø", "O",
		"Ù", "U", "Ú", "U", "Û", "U", "Ü", "U", "Ý", "Y", "Þ", "Th",
		"ß", "ss", "à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a",
		"å", "a", "æ", "ae", "ç", "c", "è", "e", "é", "e", "ê", "e",
		"ë", "e", "ì", "i", "í", "i", "î", "i", "ï", "i", "ð", "d",
		"ñ", "n", "ò", "o", "ó", "o", "ô", "o", "õ", "o", "ö", "o",
		"ø", "o", "ù", "u", "ú", "u", "û", "u", "ü", "u", "ý", "y",
		"þ", "th", "ÿ", "y",
	).Replace(s)
}

const appJS = `(() => {
  const content = document.querySelector('#content');
  if (!content) return;

  const sameOrigin = (url) => url.origin === window.location.origin;
  const internal = (link) => link && link.matches('a[data-internal]') && sameOrigin(new URL(link.href));

  async function navigate(url, push = true) {
    content.classList.add('is-transitioning');
    try {
      const response = await fetch(url, { headers: { 'X-Requested-With': 'notion-ssg' } });
      if (!response.ok) throw new Error('Navigation failed');
      const html = await response.text();
      const doc = new DOMParser().parseFromString(html, 'text/html');
      const next = doc.querySelector('#content');
      if (!next) throw new Error('Missing content');
      document.title = doc.title;
      content.innerHTML = next.innerHTML;
      initDatabaseEnhancements(content);
      document.querySelectorAll('nav a').forEach((a) => {
        a.removeAttribute('aria-current');
        if (new URL(a.href).pathname === new URL(url, window.location.href).pathname) {
          a.setAttribute('aria-current', 'page');
        }
      });
      if (push) history.pushState({}, '', url);
      content.focus({ preventScroll: true });
      window.scrollTo({ top: 0, behavior: 'smooth' });
    } catch (_) {
      window.location.href = url;
    } finally {
      requestAnimationFrame(() => content.classList.remove('is-transitioning'));
    }
  }

  document.addEventListener('click', (event) => {
    const link = event.target.closest('a');
    if (!internal(link) || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    navigate(link.href);
  });

  window.addEventListener('popstate', () => navigate(window.location.href, false));
  initDatabaseEnhancements(content);

  function initDatabaseEnhancements(root) {
    root.querySelectorAll('.database').forEach((database) => {
      const active = new Set();
      const filterBar = database.querySelector('[data-filter-bar]');
      const rows = Array.from(database.querySelectorAll('tbody tr'));

      const renderFilters = () => {
        if (!filterBar) return;
        filterBar.innerHTML = '';
        Array.from(active).sort().forEach((tag) => {
          const source = database.querySelector('.tag[data-tag="' + CSS.escape(tag) + '"]');
          const button = document.createElement('button');
          button.type = 'button';
          button.className = 'tag tag-clear is-active';
          button.dataset.tag = tag;
          button.textContent = source ? source.textContent : tag;
          if (source) button.style.cssText = source.style.cssText;
          button.addEventListener('click', () => {
            active.delete(tag);
            applyFilters();
          });
          filterBar.appendChild(button);
        });
      };

      const applyFilters = () => {
        rows.forEach((row) => {
          const rowTags = new Set((row.dataset.tags || '').split(/\s+/).filter(Boolean));
          row.hidden = active.size > 0 && !Array.from(active).every((tag) => rowTags.has(tag));
        });
        database.querySelectorAll('.tag[data-tag]').forEach((tagButton) => {
          tagButton.classList.toggle('is-active', active.has(tagButton.dataset.tag));
        });
        renderFilters();
      };

      database.querySelectorAll('.tag[data-tag]').forEach((tagButton) => {
        tagButton.addEventListener('click', () => {
          const tag = tagButton.dataset.tag;
          if (active.has(tag)) active.delete(tag);
          else active.add(tag);
          applyFilters();
        });
      });

      database.querySelectorAll('th button[data-sort]').forEach((button, index) => {
        button.addEventListener('click', () => {
          const tbody = button.closest('table').querySelector('tbody');
          const direction = button.dataset.direction === 'asc' ? 'desc' : 'asc';
          button.dataset.direction = direction;
          const factor = direction === 'asc' ? 1 : -1;
          const type = button.dataset.sort;
          rows.sort((a, b) => {
            const av = a.children[index]?.dataset.sortValue || a.children[index]?.textContent || '';
            const bv = b.children[index]?.dataset.sortValue || b.children[index]?.textContent || '';
            if (type === 'date') {
              const ad = Number.isNaN(Date.parse(av)) ? 0 : Date.parse(av);
              const bd = Number.isNaN(Date.parse(bv)) ? 0 : Date.parse(bv);
              return (ad - bd) * factor;
            }
            return av.localeCompare(bv, undefined, { numeric: true, sensitivity: 'base' }) * factor;
          });
          rows.forEach((row) => tbody.appendChild(row));
        });
      });
    });
  }
})();
`
