package anilist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/hongnoul/mangap/anilist"
	"github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/log"
	"github.com/hongnoul/mangap/source"
	"github.com/spf13/viper"
	"net/http"
	"strconv"
)

var markReadQuery = `
mutation ($ID: Int, $progress: Int) {
	SaveMediaListEntry (mediaId: $ID, progress: $progress, status: CURRENT) {
		ID
	}
}
`

func (a *Anilist) MarkRead(chapter *source.Chapter) error {
	if a.token == "" {
		a.token = viper.GetString(key.AnilistToken)
	}
	if a.token == "" {
		err := a.login()
		if err != nil {
			log.Error(err)
			return err
		}
	}

	manga, err := anilist.FindClosest(chapter.Manga.Name)
	if err != nil {
		log.Error(err)
		return err
	}

	// prepare body
	body := map[string]interface{}{
		"query": markReadQuery,
		"variables": map[string]interface{}{
			"ID":       manga.ID,
			"progress": chapter.Index,
		},
	}

	// parse body to json
	jsonBody, err := json.Marshal(body)
	if err != nil {
		log.Error(err)
		return err
	}

	// make request
	req, err := http.NewRequest(
		http.MethodPost,
		"https://graphql.anilist.co",
		bytes.NewBuffer(jsonBody),
	)

	if err != nil {
		log.Error(err)
		return err
	}

	// set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Accept", "application/json")

	// send request
	log.Info("Sending request to Anilist: " + string(jsonBody))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error(err)
		return err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("anilist session expired, run `mangap integration anilist` to log in again")
	}

	if resp.StatusCode != http.StatusOK {
		log.Info("Request failed with status code: " + strconv.Itoa(resp.StatusCode))
		return fmt.Errorf("invalid response code %d", resp.StatusCode)
	}

	// decode response
	var response struct {
		Data struct {
			SaveMediaListEntry struct {
				ID int `json:"ID"`
			} `json:"SaveMediaListEntry"`
		} `json:"data"`
	}

	return json.NewDecoder(resp.Body).Decode(&response)
}
