package slack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"

	"github.com/akawula/DoraMatic/store"
)

func templatePullRequest(pr store.SecurityPR) []map[string]interface{} {
	m := fmt.Sprintf("%s\n*%s* [+%d -%d] Author: %s\nState: %s, Created At: %s", pr.Title, pr.RepositoryName, pr.Additions, pr.Deletions, pr.Author, pr.State, pr.CreatedAt)
	if pr.State == "MERGED" {
		m = fmt.Sprintf("%s\n*%s* [+%d -%d] Author: %s\nState: %s, Merged At: %s", pr.Title, pr.RepositoryName, pr.Additions, pr.Deletions, pr.Author, pr.State, pr.MergedAt.String)
	}
	return []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": m,
			},
			"accessory": map[string]interface{}{
				"type": "button",
				"text": map[string]interface{}{
					"type":  "plain_text",
					"emoji": true,
					"text":  "Open",
				},
				"value":     pr.Url,
				"url":       pr.Url,
				"action_id": pr.Id,
			},
		},
		{
			"type": "divider",
		},
	}
}

func SendMessage(prs []store.SecurityPR) {
	initialBlock := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type":  "plain_text",
				"emoji": true,
				"text":  "Looks like there were new Pull Requests yesterday",
			},
		},
		{
			"type": "divider",
		},
	}

	for _, pr := range prs {
		initialBlock = append(initialBlock, templatePullRequest(pr)...)
	}

	for c := range slices.Chunk(initialBlock, 50) {
		sendMesasge(c, "UE9M08BLP")
	}

	sendMesasge([]map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type":  "plain_text",
				"emoji": true,
				"text":  "Doramatic success!",
			},
		},
	}, "UJ36ACNUD")
}

func sendMesasge(blocks []map[string]interface{}, channel string) error {
	// Message payload
	payload := map[string]interface{}{
		"channel": channel,
		"blocks":  blocks,
	}

	// Your Slack Bot Token
	token := os.Getenv("SLACK_TOKEN")
	if len(token) == 0 {
		return errors.New("SLACK_TOKEN env is required")
	}
	// Slack API endpoint for sending messages
	url := "https://slack.com/api/chat.postMessage"
	// Convert payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("Slack API returned non-200 status code: %d\n", resp.StatusCode))
	}

	// Parse and print response
	var response map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&response)

	return nil
}
