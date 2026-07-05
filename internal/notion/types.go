package notion

import "context"

// Client is the Notion boundary used by the site generator.
type Client interface {
	FindPageByTitle(ctx context.Context, title string) (PageRef, error)
	Children(ctx context.Context, blockID string) ([]Block, error)
	QueryDatabase(ctx context.Context, databaseID string) ([]DatabaseRow, error)
}

type PageRef struct {
	ID    string
	Title string
}

type DatabaseRow struct {
	ID         string
	Title      string
	Properties map[string]string
}

type Block struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	HasChildren bool   `json:"has_children"`

	Paragraph        *RichTextBlock  `json:"paragraph,omitempty"`
	Heading1         *RichTextBlock  `json:"heading_1,omitempty"`
	Heading2         *RichTextBlock  `json:"heading_2,omitempty"`
	Heading3         *RichTextBlock  `json:"heading_3,omitempty"`
	BulletedListItem *RichTextBlock  `json:"bulleted_list_item,omitempty"`
	NumberedListItem *RichTextBlock  `json:"numbered_list_item,omitempty"`
	Quote            *RichTextBlock  `json:"quote,omitempty"`
	Callout          *RichTextBlock  `json:"callout,omitempty"`
	ToDo             *ToDoBlock      `json:"to_do,omitempty"`
	Code             *CodeBlock      `json:"code,omitempty"`
	Image            *FileBlock      `json:"image,omitempty"`
	File             *FileBlock      `json:"file,omitempty"`
	Bookmark         *BookmarkBlock  `json:"bookmark,omitempty"`
	ChildPage        *ChildPageBlock `json:"child_page,omitempty"`
	ChildDatabase    *ChildPageBlock `json:"child_database,omitempty"`
	Children         []Block         `json:"-"`
	DatabaseRows     []DatabaseRow   `json:"-"`
}

type RichTextBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color"`
}

type ToDoBlock struct {
	RichText []RichText `json:"rich_text"`
	Checked  bool       `json:"checked"`
}

type CodeBlock struct {
	RichText []RichText `json:"rich_text"`
	Language string     `json:"language"`
}

type FileBlock struct {
	Type     string     `json:"type"`
	Caption  []RichText `json:"caption"`
	External *URLObject `json:"external,omitempty"`
	File     *URLObject `json:"file,omitempty"`
}

type URLObject struct {
	URL string `json:"url"`
}

type BookmarkBlock struct {
	URL     string     `json:"url"`
	Caption []RichText `json:"caption"`
}

type ChildPageBlock struct {
	Title string `json:"title"`
}

type RichText struct {
	PlainText   string      `json:"plain_text"`
	Href        string      `json:"href"`
	Annotations Annotations `json:"annotations"`
}

type Annotations struct {
	Bold          bool `json:"bold"`
	Italic        bool `json:"italic"`
	Strikethrough bool `json:"strikethrough"`
	Underline     bool `json:"underline"`
	Code          bool `json:"code"`
}
