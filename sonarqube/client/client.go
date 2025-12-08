package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// SonarQubeClient implements the Client interface
type SonarQubeClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a new SonarQube client
func NewClient(baseURL, token string) *SonarQubeClient {
	return &SonarQubeClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{},
	}
}

// doRequest performs an HTTP request with authentication
func (c *SonarQubeClient) doRequest(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// SonarQube uses token as username with empty password
	req.SetBasicAuth(c.token, "")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

// GetAllProjects fetches all projects from SonarQube
func (c *SonarQubeClient) GetAllProjects(ctx context.Context) ([]Project, error) {
	var allProjects []Project
	pageIndex := 1
	pageSize := 500

	for {
		endpoint := fmt.Sprintf("/api/components/search?qualifiers=TRK&ps=%d&p=%d", pageSize, pageIndex)
		body, err := c.doRequest(ctx, endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch projects (page %d): %w", pageIndex, err)
		}

		var response ComponentsResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		for _, comp := range response.Components {
			allProjects = append(allProjects, Project{
				Key:  comp.Key,
				Name: comp.Name,
			})
		}

		// Check if we've fetched all pages
		if len(response.Components) < pageSize || pageIndex*pageSize >= response.Paging.Total {
			break
		}
		pageIndex++
	}

	return allProjects, nil
}

// GetProjectMetrics fetches metrics for a specific project
func (c *SonarQubeClient) GetProjectMetrics(ctx context.Context, projectKey string, metricKeys []string) (*ProjectMetrics, error) {
	metricsParam := url.QueryEscape(strings.Join(metricKeys, ","))
	endpoint := fmt.Sprintf("/api/measures/component?component=%s&metricKeys=%s", url.QueryEscape(projectKey), metricsParam)

	body, err := c.doRequest(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metrics for project %s: %w", projectKey, err)
	}

	var response struct {
		Component struct {
			Key      string `json:"key"`
			Name     string `json:"name"`
			Measures []struct {
				Metric string `json:"metric"`
				Value  string `json:"value"`
			} `json:"measures"`
		} `json:"component"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	metrics := make(map[string]Metric)
	for _, m := range response.Component.Measures {
		value, _ := strconv.ParseFloat(m.Value, 64)
		metrics[m.Metric] = Metric{
			Key:   m.Metric,
			Value: value,
		}
	}

	return &ProjectMetrics{
		ProjectKey:  response.Component.Key,
		ProjectName: response.Component.Name,
		Metrics:     metrics,
	}, nil
}

// GetAllProjectsMetrics fetches metrics for all projects
func (c *SonarQubeClient) GetAllProjectsMetrics(ctx context.Context, metricKeys []string) ([]ProjectMetrics, error) {
	// First, get all projects
	projects, err := c.GetAllProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch projects: %w", err)
	}

	var allMetrics []ProjectMetrics

	// Fetch metrics for each project
	for _, project := range projects {
		metrics, err := c.GetProjectMetrics(ctx, project.Key, metricKeys)
		if err != nil {
			// Log the error but continue with other projects
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch metrics for project %s: %v\n", project.Key, err)
			continue
		}
		allMetrics = append(allMetrics, *metrics)
	}

	return allMetrics, nil
}
