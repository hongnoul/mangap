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
	language := viper.GetString(key.MangadexLanguage)
	cacheKey := query + "|" + language
	if cached, ok := m.cache.mangas.Get(cacheKey).Get(); ok {
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
	if language != "any" {
		// Only manga that actually have chapters in the preferred language.
		params.Add("availableTranslatedLanguage[]", language)
	}
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
			Name:   mangaTitle(&manga, language),
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

	_ = m.cache.mangas.Set(cacheKey, mangas)
	return mangas, nil
}

// mangaTitle prefers a title in lang, then English, then any title.
func mangaTitle(manga *mangodex.Manga, lang string) string {
	for _, l := range []string{lang, "en"} {
		if t := manga.GetTitle(l); t != "" {
			return t
		}
	}
	for _, t := range manga.Attributes.Title.Values {
		if t != "" {
			return t
		}
	}
	return manga.ID
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
