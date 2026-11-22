package generic

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/hongnoul/mangaz/constant"
	"github.com/hongnoul/mangaz/network"
	"github.com/hongnoul/mangaz/source"
)

// FirstPageURL returns the image URL of a chapter's first page with one
// plain HTTP request. Unlike PagesOf it does not go through the shared
// colly collector, so many chapters can be looked up in parallel (the
// collector's Wait() blocks on every in-flight request).
func (s *Scraper) FirstPageURL(chapter *source.Chapter) (string, error) {
	req, err := http.NewRequest(http.MethodGet, chapter.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", constant.UserAgent)
	req.Header.Set("Referer", chapter.URL)
	req.Header.Set("Accept", "text/html")
	resp, err := network.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", chapter.URL, resp.Status)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}
	sel := doc.Find(s.config.PageExtractor.Selector).First()
	if sel.Length() == 0 {
		return "", fmt.Errorf("%s: no pages", chapter.URL)
	}
	link := s.config.PageExtractor.URL(sel)
	if link == "" {
		return "", fmt.Errorf("%s: empty page url", chapter.URL)
	}
	if u, err := url.Parse(link); err == nil && !u.IsAbs() {
		if base, err := url.Parse(chapter.URL); err == nil {
			link = base.ResolveReference(u).String()
		}
	}
	return link, nil
}
