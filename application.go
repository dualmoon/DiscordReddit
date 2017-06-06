package main

import (
	"bytes"
	"encoding/json"
	"errors"
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
type Bookmark struct {
	LastID string
}

func (bookmark *Bookmark) getLastID(session *geddit.OAuthSession, subreddit string) string {
	optLimit1 := geddit.ListingOptions{Limit: 1}
	submissions, _ := session.SubredditSubmissions(subreddit, geddit.NewSubmissions, optLimit1)
	bookmark.LastID = submissions[0].FullID
	return bookmark.LastID
}

// Custom logging function
type Clog struct{}

func (Clog) debug(message string) {
	fmt.Printf("[DEBUG] %v\n", message)
}
func (Clog) info(message string) {
	fmt.Printf("[INFO] %v\n", message)
}
func (Clog) warn(message string) {
	fmt.Printf("[WARNING] %v\n", message)
}
func (Clog) err(error error) {
	log.Fatal(fmt.Sprintf("[ERROR] %v\n", error))
}

func main() {

	// Variables
	var tickRate time.Duration = 1 * time.Minute
	var optBefore geddit.ListingOptions
	bookmark := new(Bookmark)
	clog := new(Clog)

	// Load .env
	err := godotenv.Load()
	if err != nil {
		clog.err(errors.New(fmt.Sprintf("Error loading .env file: %v", err)))
	}

	redditClient := os.Getenv("REDDIT_CLIENT")
	redditSecret := os.Getenv("REDDIT_SECRET")
	redditUsername := os.Getenv("REDDIT_USERNAME")
	redditPassword := os.Getenv("REDDIT_PASSWORD")
	discordWebhookClient := os.Getenv("DISCORD_WEBHOOK_CLIENT")
	discordWebhookSecret := os.Getenv("DISCORD_WEBHOOK_SECRET")

	// Configuration
	webhookBaseURL := "https://discordapp.com/api/v7/webhooks/"
	requestUrl := fmt.Sprintf("%v%v/%v", webhookBaseURL, discordWebhookClient, discordWebhookSecret)
	subreddit := "shitredditsays"
	subredditPretty := "ShitRedditSays"
	iconURL := "https://i.imgur.com/3NtinwD.png"

	// New oAuth session for Reddit API
	session, err := geddit.NewOAuthSession(
		redditClient,
		redditSecret,
		"brdecho v0.02",
		"http://airsi.de",
	)
	if err != nil {
		clog.err(errors.New(fmt.Sprintf("Error in creating Reddit session object: %v", err)))
	}

	// Create new auth token for confidential clients (personal scripts/apps).
	err = session.LoginAuth(redditUsername, redditPassword)
	if err != nil {
		clog.err(errors.New(fmt.Sprintf("Error logging in to Reddit: %v", err)))
	}

	// Get our initial bookmark
	bookmark.getLastID(session, subreddit)

	// Main loop
	timer := time.Tick(tickRate)
	for now := range timer {
		clog.debug(fmt.Sprintf("now: %v", now))

		// Get submissions since our bookmark
		submissions, _ := session.SubredditSubmissions(subreddit, geddit.NewSubmissions, optBefore)

		// If there's no new submissions, move on
		if len(submissions) < 1 {
			clog.debug("No new submissions. Continuing.")
			continue
		}

		// Only work with the next submission
		submission := submissions[0]
		// Update bookmark
		bookmark.LastID = submission.FullID
		optBefore = geddit.ListingOptions{Before: bookmark.LastID, Limit: 1}
		clog.debug(fmt.Sprintf("New FullID is: %v, Thumbnail URL is: %s", submission.FullID, submission.ThumbnailURL))

		/// Process submission to send to webhook
		// Prep a HTTP form data object
		embeds := EmbedData{Embeds: []Embed{
			{
				Title: fmt.Sprintf("New post to r/%v", subredditPretty),
				URL:   submission.FullPermalink(),
				Color: "16763904",
				//	Description: s.URL,
				Fields: []EmbedField{
					{
						Name:   submission.Title,
						Value:  submission.URL,
						Inline: false,
					},
				},
				Author: EmbedAuthor{
					Name:    submission.Author,
					URL:     fmt.Sprintf("https://www.reddit.com/user/%s/", submission.Author),
					IconURL: iconURL,
				},
			},
		}}

		// Create json byte data for body
		jsonEmbeds, err := json.Marshal(embeds)
		if err != nil {
			clog.err(errors.New(fmt.Sprintf("Error marshalling JSON data: %v", err)))
		}
		clog.debug(fmt.Sprintf("json embeds data: %s", jsonEmbeds))

		// POST to Discord
		response, err := http.Post(requestUrl, "application/json", bytes.NewBuffer(jsonEmbeds))
		if err != nil {
			clog.err(errors.New(fmt.Sprintf("Error in HTTP POST to Discord: %v", err)))
		}
		body_byte, err := ioutil.ReadAll(response.Body)
		if err != nil {
			clog.err(errors.New(fmt.Sprintf("Error from Discord after HTTP POST: %v", err)))
		}

		// Print POST response
		clog.info(fmt.Sprintf("post response: %s", body_byte))

		// Look ahead to see if there's more submissions to process
		optInnerBefore := geddit.ListingOptions{Before: bookmark.LastID}
		submissions, _ = session.SubredditSubmissions(subreddit, geddit.NewSubmissions, optInnerBefore)
		if len(submissions) > 0 {
			clog.warn(fmt.Sprintf("%v submissions left to process", len(submissions)))
		}
	} // main loop
}
