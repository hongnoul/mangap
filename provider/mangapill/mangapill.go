package mangapill

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/hongnoul/mangap/provider/generic"
	"net/url"
	"strings"
	"time"
)

var Config = &generic.Configuration{
	Name:            "Mangapill",
	Delay:           50 * time.Millisecond,
	Parallelism:     50,
	ReverseChapters: true,
	BaseURL:         "https://mangapill.com",
	GenerateSearchURL: func(query string) string {
		query = strings.TrimSpace(strings.ToLower(query))
		template := "https://mangapill.com/search?q=%s&type=&status="
		return fmt.Sprintf(template, url.QueryEscape(query))
	},
	// The navbar autocomplete: same ranking as /search (top results match)
	// in ~1-2 KB instead of ~9 KB compressed, and often 100-150 ms sooner.
	QuickSearchURL: func(query string) string {
		query = strings.TrimSpace(strings.ToLower(query))
		return "https://mangapill.com/quick-search?q=" + url.QueryEscape(query)
	},
	QuickMangaExtractor: &generic.Extractor{
		Selector: "a[href^='/manga/']",
		Name: func(selection *goquery.Selection) string {
			return strings.TrimSpace(selection.Find("div.font-black").First().Text())
		},
		URL: func(selection *goquery.Selection) string {
			return selection.AttrOr("href", "")
		},
		Volume: func(selection *goquery.Selection) string {
			return ""
		},
		Cover: func(selection *goquery.Selection) string {
			return selection.Find("img").AttrOr("src", "")
		},
		AltNames: func(selection *goquery.Selection) []string {
			return splitAlt(selection.Find("div.text-secondary").First().Text())
		},
	},
	MangaExtractor: &generic.Extractor{
		Selector: "body > div.container.py-3 > div.my-3.grid.justify-end.gap-3.grid-cols-2.md\\:grid-cols-3.lg\\:grid-cols-5 > div",
		Name: func(selection *goquery.Selection) string {
			return strings.TrimSpace(selection.Find("div a div.leading-tight").Text())
		},
		URL: func(selection *goquery.Selection) string {
			return selection.Find("div a:first-child").AttrOr("href", "")
		},
		Volume: func(selection *goquery.Selection) string {
			return ""
		},
		Cover: func(selection *goquery.Selection) string {
			return selection.Find("img").AttrOr("data-src", "")
		},
		AltNames: func(selection *goquery.Selection) []string {
			return splitAlt(selection.Find("div.text-secondary").First().Text())
		},
	},
	ChapterExtractor: &generic.Extractor{
		Selector: "div[data-filter-list] a",
		Name: func(selection *goquery.Selection) string {
			return strings.TrimSpace(selection.Text())
		},
		URL: func(selection *goquery.Selection) string {
			return selection.AttrOr("href", "")
		},
		Volume: func(selection *goquery.Selection) string {
			return ""
		},
	},
	PageExtractor: &generic.Extractor{
		Selector: "picture img",
		URL: func(selection *goquery.Selection) string {
			return selection.AttrOr("data-src", "")
		},
	},
}

// splitAlt splits Mangapill's comma separated alternative titles. Titles
// with commas of their own get split too, which is harmless for matching.
func splitAlt(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
