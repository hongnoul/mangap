package mangadex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/hongnoul/mangap/constant"
	"github.com/hongnoul/mangap/network"
	"github.com/hongnoul/mangap/source"
)

// The at-home endpoint allows 40 requests a minute per IP. Thumbnails
// stay well under that so reading a chapter right after never gets 429.
const (
	atHomeBudget = 30
	atHomeWindow = time.Minute
)

var atHome struct {
	mu   sync.Mutex
	sent []time.Time
}

// waitAtHome blocks until another at-home request fits the budget.
func waitAtHome() {
	for {
		atHome.mu.Lock()
		now := time.Now()
		kept := atHome.sent[:0]
		for _, t := range atHome.sent {
			if now.Sub(t) < atHomeWindow {
				kept = append(kept, t)
			}
		}
		atHome.sent = kept
		if len(kept) < atHomeBudget {
			atHome.sent = append(atHome.sent, now)
			atHome.mu.Unlock()
			return
		}
		wait := atHomeWindow - now.Sub(kept[0])
		atHome.mu.Unlock()
		time.Sleep(wait)
	}
}

// FirstPageURL returns the data-saver image URL of a chapter's first page
// (small, good enough for a thumbnail) with one at-home lookup, instead
// of downloading the whole chapter like PagesOf.
func (m *Mangadex) FirstPageURL(chapter *source.Chapter) (string, error) {
	waitAtHome()
	u := "https://api.mangadex.org/at-home/server/" + chapter.ID
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", constant.HonestUserAgent)
	resp, err := network.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", u, resp.Status)
	}
	var r struct {
		BaseURL string `json:"baseUrl"`
		Chapter struct {
			Hash      string   `json:"hash"`
			Data      []string `json:"data"`
			DataSaver []string `json:"dataSaver"`
		} `json:"chapter"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	switch {
	case len(r.Chapter.DataSaver) > 0:
		return fmt.Sprintf("%s/data-saver/%s/%s", r.BaseURL, r.Chapter.Hash, r.Chapter.DataSaver[0]), nil
	case len(r.Chapter.Data) > 0:
		return fmt.Sprintf("%s/data/%s/%s", r.BaseURL, r.Chapter.Hash, r.Chapter.Data[0]), nil
	}
	return "", fmt.Errorf("%s: no pages", u)
}
