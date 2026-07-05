package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const notionVersion = "2022-06-28"

type HTTPClient struct {
	token   string
	baseURL string
	client  *http.Client
}

func NewHTTPClient(token string) *HTTPClient {
	return &HTTPClient{
		token:   token,
		baseURL: "https://api.notion.com/v1",
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *HTTPClient) FindPageByTitle(ctx context.Context, title string) (PageRef, error) {
	body := map[string]any{
		"query":     title,
		"filter":    map[string]string{"property": "object", "value": "page"},
		"page_size": 10,
	}
	var out searchResponse
	if err := c.do(ctx, http.MethodPost, "/search", body, &out); err != nil {
		return PageRef{}, err
	}
	for _, result := range out.Results {
		pageTitle := result.Title()
		if strings.EqualFold(pageTitle, title) {
			return PageRef{ID: result.ID, Title: pageTitle}, nil
		}
	}
	if len(out.Results) > 0 {
		pageTitle := out.Results[0].Title()
		return PageRef{ID: out.Results[0].ID, Title: pageTitle}, nil
	}
	return PageRef{}, fmt.Errorf("Notion page %q not found", title)
}

func (c *HTTPClient) Children(ctx context.Context, blockID string) ([]Block, error) {
	var blocks []Block
	cursor := ""
	for {
		path := fmt.Sprintf("/blocks/%s/children?page_size=100", blockID)
		if cursor != "" {
			path += "&start_cursor=" + cursor
		}
		var out childrenResponse
		if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
			return nil, err
		}
		blocks = append(blocks, out.Results...)
		if !out.HasMore || out.NextCursor == "" {
			break
		}
		cursor = out.NextCursor
	}
	return blocks, nil
}

func (c *HTTPClient) QueryDatabase(ctx context.Context, databaseID string) ([]DatabaseRow, error) {
	var rows []DatabaseRow
	cursor := ""
	for {
		body := map[string]any{"page_size": 100}
		if cursor != "" {
			body["start_cursor"] = cursor
		}
		var out databaseResponse
		if err := c.do(ctx, http.MethodPost, fmt.Sprintf("/databases/%s/query", databaseID), body, &out); err != nil {
			return nil, err
		}
		for _, page := range out.Results {
			rows = append(rows, page.Row())
		}
		if !out.HasMore || out.NextCursor == "" {
			break
		}
		cursor = out.NextCursor
	}
	return rows, nil
}

func (c *HTTPClient) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Notion-Version", notionVersion)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Notion API %s %s: %s", method, path, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

type searchResponse struct {
	Results []pageObject `json:"results"`
}

type childrenResponse struct {
	Results    []Block `json:"results"`
	HasMore    bool    `json:"has_more"`
	NextCursor string  `json:"next_cursor"`
}

type databaseResponse struct {
	Results    []pageObject `json:"results"`
	HasMore    bool         `json:"has_more"`
	NextCursor string       `json:"next_cursor"`
}

type pageObject struct {
	ID         string                     `json:"id"`
	Properties map[string]json.RawMessage `json:"properties"`
}

func (p pageObject) Title() string {
	for _, raw := range p.Properties {
		var prop struct {
			Type  string `json:"type"`
			Title []struct {
				PlainText string `json:"plain_text"`
			} `json:"title"`
		}
		if json.Unmarshal(raw, &prop) == nil && prop.Type == "title" {
			parts := make([]string, 0, len(prop.Title))
			for _, t := range prop.Title {
				parts = append(parts, t.PlainText)
			}
			return strings.TrimSpace(strings.Join(parts, ""))
		}
	}
	return "Untitled"
}

func (p pageObject) Row() DatabaseRow {
	row := DatabaseRow{ID: p.ID, Title: p.Title(), Properties: map[string]string{}}
	for name, raw := range p.Properties {
		if propertyType(raw) == "title" {
			continue
		}
		row.Properties[name] = plainProperty(raw)
	}
	return row
}

func propertyType(raw json.RawMessage) string {
	var prop struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &prop) != nil {
		return ""
	}
	return prop.Type
}

func plainProperty(raw json.RawMessage) string {
	var prop struct {
		Type     string     `json:"type"`
		RichText []RichText `json:"rich_text"`
		Title    []RichText `json:"title"`
		URL      string     `json:"url"`
		Email    string     `json:"email"`
		Phone    string     `json:"phone_number"`
		Checkbox bool       `json:"checkbox"`
		Number   *float64   `json:"number"`
		Select   *struct {
			Name string `json:"name"`
		} `json:"select"`
		MultiSelect []struct {
			Name string `json:"name"`
		} `json:"multi_select"`
		Status *struct {
			Name string `json:"name"`
		} `json:"status"`
		Date *struct {
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"date"`
	}
	if json.Unmarshal(raw, &prop) != nil {
		return ""
	}
	switch prop.Type {
	case "title":
		return joinRichText(prop.Title)
	case "rich_text":
		return joinRichText(prop.RichText)
	case "url":
		return prop.URL
	case "email":
		return prop.Email
	case "phone_number":
		return prop.Phone
	case "checkbox":
		if prop.Checkbox {
			return "true"
		}
		return "false"
	case "number":
		if prop.Number != nil {
			return fmt.Sprintf("%g", *prop.Number)
		}
	case "select":
		if prop.Select != nil {
			return prop.Select.Name
		}
	case "multi_select":
		names := make([]string, 0, len(prop.MultiSelect))
		for _, v := range prop.MultiSelect {
			names = append(names, v.Name)
		}
		return strings.Join(names, ", ")
	case "status":
		if prop.Status != nil {
			return prop.Status.Name
		}
	case "date":
		if prop.Date != nil {
			if prop.Date.End != "" {
				return prop.Date.Start + " - " + prop.Date.End
			}
			return prop.Date.Start
		}
	}
	return ""
}

func joinRichText(parts []RichText) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, part.PlainText)
	}
	return strings.Join(out, "")
}
