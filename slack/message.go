package slack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"

	"time" // Add time import for formatting

	"github.com/akawula/DoraMatic/store"
)

// slackAPIURL is the endpoint for posting messages. Can be overridden for tests.
var slackAPIURL = "https://slack.com/api/chat.postMessage"

func templatePullRequest(pr store.SecurityPR) []map[string]interface{} {
	mergedAtStr := "N/A"
	if pr.MergedAt.Valid {
		mergedAtStr = pr.MergedAt.Time.Format(time.RFC1123) // Format the time if valid
	}

	m := fmt.Sprintf("%s\n*%s* [+%d -%d] Author: %s\nState: %s, Created At: %s", pr.Title, pr.RepositoryName, pr.Additions, pr.Deletions, pr.Author, pr.State, pr.CreatedAt)
	if pr.State == "MERGED" {
		m = fmt.Sprintf("%s\n*%s* [+%d -%d] Author: %s\nState: %s, Merged At: %s", pr.Title, pr.RepositoryName, pr.Additions, pr.Deletions, pr.Author, pr.State, mergedAtStr) // Use the formatted string
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
		if err := sendMesasge(c, "UE9M08BLP"); err != nil {
			fmt.Printf("Error sending message chunk: %v\n", err)
			// Continue with next chunk even if there's an error
		}
	}

	if err := sendMesasge([]map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type":  "plain_text",
				"emoji": true,
				"text":  "Doramatic success!",
			},
		},
	}, "UJ36ACNUD"); err != nil {
		fmt.Printf("Error sending success message: %v\n", err)
	}
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

	// Convert payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", slackAPIURL, bytes.NewBuffer(payloadBytes))
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
		return fmt.Errorf("Slack API returned non-200 status code: %d", resp.StatusCode)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode Slack API response: %w", err)
	}

	// Check if Slack reported an error in their response
	if ok, _ := response["ok"].(bool); !ok {
		errMsg, _ := response["error"].(string)
		return fmt.Errorf("Slack API reported error: %s", errMsg)
	}

	return nil
}
