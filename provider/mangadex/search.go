package mangadex

import (
	"encoding/json"
	"fmt"
	"github.com/darylhjd/mangodex"
	"github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/source"
	"github.com/spf13/viper"
	"log"
	"net/url"
	"strconv"
)

func (m *Mangadex) Search(query string) ([]*source.Manga, error) {
	if cached, ok := m.cache.mangas.Get(query).Get(); ok {
		for _, manga := range cached {
			manga.Source = m
		}

		return cached, nil
	}

	params := url.Values{}
	params.Set("limit", strconv.Itoa(100))

	ratings := []string{mangodex.Safe, mangodex.Suggestive}

	for _, rating := range ratings {
		params.Add("contentRating[]", rating)
	}

	if viper.GetBool(key.MangadexNSFW) {
		params.Add("contentRating[]", mangodex.Porn)
		params.Add("contentRating[]", mangodex.Erotica)
	}

	params.Set("order[followedCount]", "desc")
	params.Set("title", query)
	// Embed cover_art relationships so cover file names come back inline.
	params.Add("includes[]", mangodex.CoverArtRel)

	mangaList, err := m.client.Manga.GetMangaList(params)
	if err != nil {
		log.Fatalln(err)
		return nil, err
	}

	var mangas []*source.Manga

	for i, manga := range mangaList.Data {
		m := source.Manga{
			Name:   manga.GetTitle(viper.GetString(key.MangadexLanguage)),
			URL:    fmt.Sprintf("https://mangadex.org/title/%s", manga.ID),
			Index:  uint16(i),
			ID:     manga.ID,
			Source: m,
		}

		if cover := coverURL(&manga); cover != "" {
			m.Metadata.Cover.ExtraLarge = cover
			m.Metadata.Cover.Large = cover + ".512.jpg"
			m.Metadata.Cover.Medium = cover + ".256.jpg"
		}

		mangas = append(mangas, &m)
	}

	_ = m.cache.mangas.Set(query, mangas)
	return mangas, nil
}

// coverURL builds the full-size cover URL from an embedded cover_art
// relationship, or "" if the response carried none.
func coverURL(manga *mangodex.Manga) string {
	for _, rel := range manga.Relationships {
		if rel.Type != mangodex.CoverArtRel {
			continue
		}
		raw, ok := rel.Attributes.(*json.RawMessage)
		if !ok || raw == nil {
			continue
		}
		var attrs struct {
			FileName string `json:"fileName"`
		}
		if json.Unmarshal(*raw, &attrs) != nil || attrs.FileName == "" {
			continue
		}
		return fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s", manga.ID, attrs.FileName)
	}
	return ""
}
