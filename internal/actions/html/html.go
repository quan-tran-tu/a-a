package html

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	htmldom "golang.org/x/net/html"

	"a-a/internal/utils"
)

func parseDoc(html string) (*goquery.Document, error) {
	return goquery.NewDocumentFromReader(strings.NewReader(html))
}
func absolute(base, href string) string {
	u, err := url.Parse(href)
	if err != nil || href == "" {
		return href
	}
	if u.IsAbs() {
		return u.String()
	}
	if base == "" {
		return href
	}
	bu, err := url.Parse(base)
	if err != nil {
		return href
	}
	return bu.ResolveReference(u).String()
}
func outerHTML(sel *goquery.Selection) string {
	var buf bytes.Buffer
	for _, n := range sel.Nodes {
		_ = htmldom.Render(&buf, n)
	}
	return buf.String()
}

func handleLinks(_ context.Context, payload map[string]any) (map[string]any, error) {
	html, err := utils.GetStringPayload(payload, "html")
	if err != nil {
		return nil, err
	}
	baseURL, _ := payload["base_url"].(string)

	doc, err := parseDoc(html)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	type link struct {
		Text string `json:"text"`
		URL  string `json:"url"`
	}
	var out []link
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		t := strings.TrimSpace(s.Text())
		out = append(out, link{Text: t, URL: absolute(baseURL, href)})
	})
	b, _ := json.Marshal(out)
	return map[string]any{"links_json": string(b)}, nil
}

func handleLinksBulk(_ context.Context, payload map[string]any) (map[string]any, error) {
	// pages_json: array of objects with {url, status_code, content}
	pagesJSON, err := utils.GetStringPayload(payload, "pages_json")
	if err != nil {
		return nil, err
	}
	baseURL, _ := payload["base_url"].(string)

	type page struct {
		URL        string `json:"url"`
		StatusCode int    `json:"status_code"`
		Content    string `json:"content"`
	}
	var pages []page
	if err := json.Unmarshal([]byte(pagesJSON), &pages); err != nil {
		return nil, fmt.Errorf("pages_json must be array of {url,status_code,content}: %w", err)
	}

	type link struct {
		Text string `json:"text"`
		URL  string `json:"url"`
	}
	out := make([]link, 0, 256)
	for _, p := range pages {
		if strings.HasPrefix(p.Content, "ERROR:") || strings.TrimSpace(p.Content) == "" {
			continue
		}
		doc, err := parseDoc(p.Content)
		if err != nil {
			continue
		}
		doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			t := strings.TrimSpace(s.Text())
			base := p.URL
			if baseURL != "" {
				base = baseURL
			}
			out = append(out, link{Text: t, URL: absolute(base, href)})
		})
	}
	b, _ := json.Marshal(out)
	return map[string]any{"links_json": string(b)}, nil
}

func handleSelectAll(_ context.Context, payload map[string]any) (map[string]any, error) {
	html, err := utils.GetStringPayload(payload, "html")
	if err != nil {
		return nil, err
	}
	selector, err := utils.GetStringPayload(payload, "selector")
	if err != nil {
		return nil, err
	}
	doc, err := parseDoc(html)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	var items []string
	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		items = append(items, outerHTML(s))
	})
	b, _ := json.Marshal(items)
	return map[string]any{"items_json": string(b)}, nil
}

func handleInnerText(_ context.Context, payload map[string]any) (map[string]any, error) {
	html, err := utils.GetStringPayload(payload, "html")
	if err != nil {
		return nil, err
	}
	doc, err := parseDoc(html)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	txt := strings.TrimSpace(doc.Text())
	return map[string]any{"text": txt}, nil
}

func HandleHtmlAction(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "links":
		return handleLinks(ctx, payload)
	case "links_bulk":
		return handleLinksBulk(ctx, payload)
	case "select_all":
		return handleSelectAll(ctx, payload)
	case "inner_text":
		return handleInnerText(ctx, payload)
	default:
		return nil, fmt.Errorf("unknown html operation: %s", operation)
	}
}
