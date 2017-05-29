package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/jzelinskie/geddit"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type EmbedData struct {
	Embeds []Embed `json:"embeds"`
}
type Embed struct {
	Title       string       `json:"title,omitempty"`
	URL         string       `json:"url,omitempty"`
	Color       string       `json:"color,omitempty"`
	Description string       `json:"description,omitempty"`
	Fields      []EmbedField `json:"fields,omitempty"`
	Author      EmbedAuthor  `json:"author,omitempty"`
}
type EmbedField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}
type EmbedAuthor struct {
	Name         string `json:"name,omitempty"`
	URL          string `json:"url,omitempty"`
	IconURL      string `json:"icon_url,omitempty"`
	ProxyIconURL string `json:"proxy_icon_url,omitempty"`
}

func main() {

	// Load .env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	redditClient := os.Getenv("REDDIT_CLIENT")
	redditSecret := os.Getenv("REDDIT_SECRET")
	redditUsername := os.Getenv("REDDIT_USERNAME")
	redditPassword := os.Getenv("REDDIT_PASSWORD")
	discordWebhookClient := os.Getenv("DISCORD_WEBHOOK_CLIENT")
	discordWebhookSecret := os.Getenv("DISCORD_WEBHOOK_SECRET")

	// Variables
	var lastID string
	var tickRate time.Duration = 1 * time.Minute

	// Configuration
	webhookBaseURL := "https://discordapp.com/api/v7/webhooks/"
	requestUrl := fmt.Sprintf("%v%v/%v", webhookBaseURL, discordWebhookClient, discordWebhookSecret)
	subreddit := "shitredditsays"
	subredditPretty := "ShitRedditSays"
	iconURL := "https://i.imgur.com/3NtinwD.png"

	// New oAuth session for Reddit API
	o, err := geddit.NewOAuthSession(
		redditClient,
		redditSecret,
		"brdecho v0.02",
		"http://airsi.de",
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create new auth token for confidential clients (personal scripts/apps).
	err = o.LoginAuth(redditUsername, redditPassword)
	if err != nil {
		log.Fatal(err)
	}

	// Set listing options
	optLimit1 := geddit.ListingOptions{Limit: 1}
	// Get latest submissions
	submissions, _ := o.SubredditSubmissions(subreddit, geddit.NewSubmissions, optLimit1)
	// TODO: do this a better way?
	for _, s := range submissions {
		lastID = s.FullID
		fmt.Printf("[DEBUG] FullID: %v is last item.\n", s.FullID)
	}

	timer := time.Tick(tickRate)
	for now := range timer {
		fmt.Printf("[DEBUG] now: %v\n", now)
		optBefore := geddit.ListingOptions{Before: lastID, Limit: 1}
		submissions, _ := o.SubredditSubmissions(subreddit, geddit.NewSubmissions, optBefore)
		for _, s := range submissions {
			lastID = s.FullID
			fmt.Printf("[DEBUG] New FullID is: %v\nThumbnail URL is: %s\n", s.FullID, s.ThumbnailURL)

			// Process submission to send to webhook
			// Prep a HTTP form data object
			embeds := EmbedData{Embeds: []Embed{
				{
					Title: fmt.Sprintf("New post to r/%v", subredditPretty),
					URL:   s.FullPermalink(),
					Color: "16763904",
					//	Description: s.URL,
					Fields: []EmbedField{
						{
							Name:   s.Title,
							Value:  s.URL,
							Inline: false,
						},
					},
					Author: EmbedAuthor{
						Name:    s.Author,
						URL:     fmt.Sprintf("https://www.reddit.com/user/%s/", s.Author),
						IconURL: iconURL,
					},
				},
			}}

			// Create json byte data for body
			jsonEmbeds, err := json.Marshal(embeds)
			fmt.Printf("[DEBUG] json embeds data: %s\n", jsonEmbeds)

			// POST to Discord
			rsp, err := http.Post(requestUrl, "application/json", bytes.NewBuffer(jsonEmbeds))
			if err != nil {
				log.Fatal(err)
			}
			body_byte, err := ioutil.ReadAll(rsp.Body)
			if err != nil {
				log.Fatal(err)
			}

			// Print POST response
			fmt.Printf("[INFO] post response: %s\n", body_byte)

			// Look ahead to see if there's more submissions to process
			optInnerBefore := geddit.ListingOptions{Before: lastID}
			submissions, _ := o.SubredditSubmissions("shitredditsays", geddit.NewSubmissions, optInnerBefore)
			if len(submissions) > 0 {
				fmt.Printf("[WARNING] %v left to process\n", len(submissions))
			}
		}
	} // main loop
}
