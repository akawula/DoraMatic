package slack

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/akawula/DoraMatic/store"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestTemplatePullRequest(t *testing.T) {
	createdAt := time.Now().Add(-24 * time.Hour)
	mergedAt := time.Now()

	testCases := []struct {
		name     string
		pr       store.SecurityPR
		expected []map[string]interface{}
	}{
		{
			name: "Open PR",
			pr: store.SecurityPR{
				Id:             "pr123",
				Title:          "feat: New feature",
				RepositoryName: "test-repo",
				Additions:      100,
				Deletions:      10,
				Author:         "testuser",
				State:          "OPEN",
				Url:       "http://example.com/pr/123",
				CreatedAt: createdAt.Format(time.RFC3339), // Use format consistent with potential DB storage
				MergedAt:  pgtype.Timestamptz{Valid: false}, // Not merged
			},
			expected: []map[string]interface{}{
				{
					"type": "section",
					"text": map[string]interface{}{
						"type": "mrkdwn",
						"text": fmt.Sprintf("feat: New feature\n*test-repo* [+100 -10] Author: testuser\nState: OPEN, Created At: %s", createdAt.Format(time.RFC3339)),
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
						"action_id": "pr123",
					},
				},
				{
					"type": "divider",
				},
			},
		},
		{
			name: "Merged PR",
			pr: store.SecurityPR{
				Id:             "pr456",
				Title:          "fix: Bug fix",
				RepositoryName: "another-repo",
				Additions:      5,
				Deletions:      5,
				Author:         "anotheruser",
				State:          "MERGED",
				Url:       "http://example.com/pr/456",
				CreatedAt: createdAt.Format(time.RFC3339),
				MergedAt:  pgtype.Timestamptz{Time: mergedAt, Valid: true}, // Merged
			},
			expected: []map[string]interface{}{
				{
					"type": "section",
					"text": map[string]interface{}{
						"type": "mrkdwn",
						"text": fmt.Sprintf("fix: Bug fix\n*another-repo* [+5 -5] Author: anotheruser\nState: MERGED, Merged At: %s", mergedAt.Format(time.RFC1123)),
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
						"action_id": "pr456",
					},
				},
				{
					"type": "divider",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := templatePullRequest(tc.pr)
			if !reflect.DeepEqual(result, tc.expected) {
				// Marshal both to JSON for easier comparison in logs
				resultJSON, _ := json.MarshalIndent(result, "", "  ")
				expectedJSON, _ := json.MarshalIndent(tc.expected, "", "  ")
				t.Errorf("templatePullRequest() failed for %s.\nExpected:\n%s\nGot:\n%s", tc.name, string(expectedJSON), string(resultJSON))
			}
		})
	}
}

func TestSendMesasge(t *testing.T) {
	var lastRequest *http.Request
	var lastRequestBody map[string]interface{}

	// Mock Slack server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastRequest = r // Capture the request

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}
		defer r.Body.Close()

		err = json.Unmarshal(bodyBytes, &lastRequestBody) // Capture the body
		if err != nil {
			t.Logf("Failed to unmarshal request body: %s", string(bodyBytes)) // Log raw body on error
			t.Fatalf("Failed to unmarshal request body: %v", err)
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintln(w, `{"ok": false, "error": "invalid_auth"}`)
			return
		}
		// Simulate different Slack responses based on a header or payload detail if needed
		// For now, just simulate success or a generic error
		if val, ok := lastRequestBody["channel"]; ok {
			if channelStr, ok := val.(string); ok && channelStr == "fail-channel" {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, `{"ok": false, "error": "internal_error"}`)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"ok": true}`)
	}))
	defer server.Close()

	// Override the API URL to point to the mock server
	originalURL := slackAPIURL
	slackAPIURL = server.URL
	defer func() { slackAPIURL = originalURL }() // Restore original URL after test

	// --- Test Cases ---

	t.Run("Success", func(t *testing.T) {
		t.Setenv("SLACK_TOKEN", "test-token") // Set dummy token
		lastRequest = nil                     // Reset captures
		lastRequestBody = nil

		blocks := []map[string]interface{}{{"type": "section", "text": map[string]interface{}{"type": "plain_text", "text": "hello"}}}
		channel := "test-channel"

		err := sendMesasge(blocks, channel)

		if err != nil {
			t.Errorf("sendMesasge() returned unexpected error: %v", err)
		}
		if lastRequest == nil {
			t.Fatal("No request was sent to the mock server")
		}
		if lastRequest.Method != "POST" {
			t.Errorf("Expected POST request, got %s", lastRequest.Method)
		}
		if auth := lastRequest.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Errorf("Expected Authorization header 'Bearer test-token', got '%s'", auth)
		}
		if ctype := lastRequest.Header.Get("Content-Type"); ctype != "application/json" {
			t.Errorf("Expected Content-Type header 'application/json', got '%s'", ctype)
		}
		if lastRequestBody == nil {
			t.Fatal("Request body was not captured")
		}
		if reqChan, ok := lastRequestBody["channel"].(string); !ok || reqChan != channel {
			t.Errorf("Expected channel '%s' in request body, got '%v'", channel, lastRequestBody["channel"])
		}
		if reqBlocks, ok := lastRequestBody["blocks"].([]interface{}); !ok || len(reqBlocks) != len(blocks) {
			// Basic check for block presence and count. DeepEqual might be too complex with interface{} types.
			t.Errorf("Expected %d blocks in request body, got %d", len(blocks), len(reqBlocks))
		} else {
			// Optional: More detailed block comparison if necessary
			// marshaledBlocks, _ := json.Marshal(blocks)
			// marshaledReqBlocks, _ := json.Marshal(reqBlocks)
			// if string(marshaledBlocks) != string(marshaledReqBlocks) {
			//     t.Errorf("Request blocks mismatch.\nExpected:\n%s\nGot:\n%s", marshaledBlocks, marshaledReqBlocks)
			// }
		}
	})

	t.Run("Missing Token", func(t *testing.T) {
		// Ensure SLACK_TOKEN is unset
		os.Unsetenv("SLACK_TOKEN")

		err := sendMesasge([]map[string]interface{}{}, "any-channel")
		if err == nil {
			t.Error("sendMesasge() should have returned an error when SLACK_TOKEN is missing, but got nil")
		} else if !strings.Contains(err.Error(), "SLACK_TOKEN env is required") {
			t.Errorf("sendMesasge() returned wrong error for missing token: %v", err)
		}
	})

	t.Run("Slack API Error", func(t *testing.T) {
		t.Setenv("SLACK_TOKEN", "test-token")
		lastRequest = nil
		lastRequestBody = nil

		err := sendMesasge([]map[string]interface{}{}, "fail-channel") // Use the channel that triggers error in mock server

		if err == nil {
			t.Error("sendMesasge() should have returned an error for non-200 response, but got nil")
		} else if !strings.Contains(err.Error(), "Slack API returned non-200 status code") {
			t.Errorf("sendMesasge() returned wrong error for API failure: %v", err)
		}
	})
}

func TestSendMessage(t *testing.T) {
	requests := []map[string]interface{}{} // Slice to store request bodies

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		var reqBody map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &reqBody); err == nil {
			requests = append(requests, reqBody) // Capture the body
		} else {
			t.Logf("Mock server failed to unmarshal request body: %s", string(bodyBytes))
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"ok": true}`)
	}))
	defer server.Close()

	originalURL := slackAPIURL
	slackAPIURL = server.URL
	defer func() { slackAPIURL = originalURL }()

	t.Setenv("SLACK_TOKEN", "test-token-sendmessage")

	t.Run("Single Batch", func(t *testing.T) {
		requests = nil // Reset captures
		now := time.Now()
		prs := []store.SecurityPR{
			{Id: "pr1", Title: "PR 1", RepositoryName: "repo1", State: "OPEN", Url: "url1", CreatedAt: now.Add(-time.Hour).Format(time.RFC3339)},
			{Id: "pr2", Title: "PR 2", RepositoryName: "repo2", State: "MERGED", Url: "url2", CreatedAt: now.Add(-time.Hour).Format(time.RFC3339), MergedAt: pgtype.Timestamptz{Time: now, Valid: true}},
		}

		SendMessage(prs)

		if len(requests) != 2 { // 1 batch of PRs + 1 final success message
			t.Fatalf("Expected 2 requests to Slack, got %d", len(requests))
		}

		// Check first request (PRs)
		req1 := requests[0]
		if chan1, ok := req1["channel"].(string); !ok || chan1 != "UE9M08BLP" {
			t.Errorf("Expected PR message channel 'UE9M08BLP', got '%v'", req1["channel"])
		}
		if blocks1, ok := req1["blocks"].([]interface{}); !ok {
			t.Errorf("Expected blocks in first request, got none")
		} else {
			// Initial block + divider + (2 blocks per PR * 2 PRs) = 2 + 4 = 6 blocks
			expectedBlocks := 2 + (len(prs) * 2)
			if len(blocks1) != expectedBlocks {
				t.Errorf("Expected %d blocks in first request, got %d", expectedBlocks, len(blocks1))
			}
			// Check if first block is the introductory text
			firstBlock, _ := blocks1[0].(map[string]interface{})
			textMap, _ := firstBlock["text"].(map[string]interface{})
			text, _ := textMap["text"].(string)
			if !strings.Contains(text, "new Pull Requests yesterday") {
				t.Errorf("First block text mismatch: %s", text)
			}
		}

		// Check second request (Success message)
		req2 := requests[1]
		if chan2, ok := req2["channel"].(string); !ok || chan2 != "UJ36ACNUD" {
			t.Errorf("Expected success message channel 'UJ36ACNUD', got '%v'", req2["channel"])
		}
		if blocks2, ok := req2["blocks"].([]interface{}); !ok || len(blocks2) != 1 {
			t.Errorf("Expected 1 block in success message, got %d", len(blocks2))
		} else {
			block, _ := blocks2[0].(map[string]interface{})
			textMap, _ := block["text"].(map[string]interface{})
			text, _ := textMap["text"].(string)
			if !strings.Contains(text, "Doramatic success!") {
				t.Errorf("Success block text mismatch: %s", text)
			}
		}
	})

	t.Run("Multiple Batches (Chunking)", func(t *testing.T) {
		requests = nil // Reset captures
		// Create enough PRs to exceed the 50-block limit (2 blocks per PR + 2 initial blocks)
		// Need (50 - 2) / 2 = 24 PRs for the first chunk. Let's use 26 PRs for 2 chunks.
		var prs []store.SecurityPR
		for i := 0; i < 26; i++ {
			prs = append(prs, store.SecurityPR{
				Id:             fmt.Sprintf("pr%d", i),
				Title:          fmt.Sprintf("PR %d", i),
				RepositoryName: "repo",
				State:          "OPEN",
				Url:            fmt.Sprintf("url%d", i),
				CreatedAt:      time.Now().Format(time.RFC3339),
			})
		}

		SendMessage(prs)

		// Expected requests:
		// 1st chunk: Initial 2 blocks + 24 * 2 PR blocks = 50 blocks
		// 2nd chunk: 2 * 2 PR blocks = 4 blocks
		// Final success message: 1 block
		// Total = 3 requests
		if len(requests) != 3 {
			t.Fatalf("Expected 3 requests (2 chunks + 1 final), got %d", len(requests))
		}

		// Basic check on block counts and channels
		req1 := requests[0]
		blocks1, _ := req1["blocks"].([]interface{})
		if len(blocks1) != 50 {
			t.Errorf("Expected 50 blocks in first chunk, got %d", len(blocks1))
		}
		if chan1, ok := req1["channel"].(string); !ok || chan1 != "UE9M08BLP" {
			t.Errorf("Expected PR message channel 'UE9M08BLP' for chunk 1, got '%v'", req1["channel"])
		}

		req2 := requests[1]
		blocks2, _ := req2["blocks"].([]interface{})
		// Remaining PRs: 26 total - 24 in first chunk = 2. Blocks: 2 * 2 = 4
		if len(blocks2) != 4 {
			t.Errorf("Expected 4 blocks in second chunk, got %d", len(blocks2))
		}
		if chan2, ok := req2["channel"].(string); !ok || chan2 != "UE9M08BLP" {
			t.Errorf("Expected PR message channel 'UE9M08BLP' for chunk 2, got '%v'", req2["channel"])
		}

		req3 := requests[2]
		blocks3, _ := req3["blocks"].([]interface{})
		if len(blocks3) != 1 {
			t.Errorf("Expected 1 block in final success message, got %d", len(blocks3))
		}
		if chan3, ok := req3["channel"].(string); !ok || chan3 != "UJ36ACNUD" {
			t.Errorf("Expected success message channel 'UJ36ACNUD', got '%v'", req3["channel"])
		}
	})
}
