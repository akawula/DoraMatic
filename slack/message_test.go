package slack

import (
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/akawula/DoraMatic/store"
)

func TestTemplatePullRequest(t *testing.T) {
	// Test case 1: PR State is not MERGED
	prOpen := store.SecurityPR{
		Id:             "PR123",
		Title:          "Fix critical bug",
		RepositoryName: "my-repo",
		Additions:      10,
		Deletions:      5,
		Author:         "testuser",
		State:          "OPEN",
		CreatedAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Url:            "http://example.com/pr/123",
		MergedAt:       sql.NullString{}, // Not merged
	}
	expectedOpen := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": "Fix critical bug\n*my-repo* [+10 -5] Author: testuser\nState: OPEN, Created At: 2024-01-15T10:00:00Z",
			},
			"accessory": map[string]interface{}{
				"type": "button",
				"text": map[string]interface{}{
					"type":  "plain_text",
					"emoji": true,
					"text":  "Open",
				},
				"value":     "http://example.com/pr/123",
				"url":       "http://example.com/pr/123",
				"action_id": "PR123",
			},
		},
		{
			"type": "divider",
		},
	}
	actualOpen := templatePullRequest(prOpen)
	if !reflect.DeepEqual(actualOpen, expectedOpen) {
		t.Errorf("Test Case 1 Failed: Expected:\n%v\nGot:\n%v", expectedOpen, actualOpen)
	}

	// Test case 2: PR State is MERGED
	mergedTime := time.Date(2024, 1, 16, 12, 30, 0, 0, time.UTC)
	prMerged := store.SecurityPR{
		Id:             "PR456",
		Title:          "Implement new feature",
		RepositoryName: "another-repo",
		Additions:      100,
		Deletions:      20,
		Author:         "anotheruser",
		State:          "MERGED",
		CreatedAt:      time.Date(2024, 1, 10, 9, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Url:            "http://example.com/pr/456",
		MergedAt: sql.NullString{
			String: mergedTime.Format(time.RFC3339), // Use RFC3339 for comparison
			Valid:  true,
		},
	}
	// Note: MergedAt string formatting in the function uses default String(), not RFC3339
	expectedMergedText := "Implement new feature\n*another-repo* [+100 -20] Author: anotheruser\nState: MERGED, Merged At: " + prMerged.MergedAt.String

	expectedMerged := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": expectedMergedText,
			},
			"accessory": map[string]interface{}{
				"type": "button",
				"text": map[string]interface{}{
					"type":  "plain_text",
					"emoji": true,
					"text":  "Open",
				},
				"value":     "http://example.com/pr/456",
				"url":       "http://example.com/pr/456",
				"action_id": "PR456",
			},
		},
		{
			"type": "divider",
		},
	}
	actualMerged := templatePullRequest(prMerged)

	// Manual check because map comparison can be tricky
	if len(actualMerged) != len(expectedMerged) {
		t.Fatalf("Test Case 2 Failed: Length mismatch. Expected %d blocks, got %d", len(expectedMerged), len(actualMerged))
	}
	if !reflect.DeepEqual(actualMerged[0]["accessory"], expectedMerged[0]["accessory"]) {
		t.Errorf("Test Case 2 Failed (Accessory): Expected:\n%v\nGot:\n%v", expectedMerged[0]["accessory"], actualMerged[0]["accessory"])
	}
	actualText := actualMerged[0]["text"].(map[string]interface{})["text"].(string)
	if actualText != expectedMergedText {
		t.Errorf("Test Case 2 Failed (Text): Expected:\n%s\nGot:\n%s", expectedMergedText, actualText)
	}
	if !reflect.DeepEqual(actualMerged[1], expectedMerged[1]) {
		t.Errorf("Test Case 2 Failed (Divider): Expected:\n%v\nGot:\n%v", expectedMerged[1], actualMerged[1])
	}

}
