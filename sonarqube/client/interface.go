package client

import "context"

// Client is an interface for interacting with the SonarQube API
type Client interface {
	// GetAllProjects fetches all projects from SonarQube
	GetAllProjects(ctx context.Context) ([]Project, error)

	// GetProjectMetrics fetches metrics for a specific project
	GetProjectMetrics(ctx context.Context, projectKey string, metricKeys []string) (*ProjectMetrics, error)

	// GetAllProjectsMetrics fetches metrics for all projects
	GetAllProjectsMetrics(ctx context.Context, metricKeys []string) ([]ProjectMetrics, error)
}

// Project represents a SonarQube project
type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// ProjectMetrics represents metrics for a project
type ProjectMetrics struct {
	ProjectKey  string            `json:"projectKey"`
	ProjectName string            `json:"projectName"`
	Metrics     map[string]Metric `json:"metrics"`
}

// Metric represents a single metric value
type Metric struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
}

// ComponentsResponse represents the response from /api/components/search
type ComponentsResponse struct {
	Paging     Paging      `json:"paging"`
	Components []Component `json:"components"`
}

// Component represents a component in the search response
type Component struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Paging represents pagination information
type Paging struct {
	PageIndex int `json:"pageIndex"`
	PageSize  int `json:"pageSize"`
	Total     int `json:"total"`
}

// MeasuresSearchResponse represents the response from /api/measures/search
type MeasuresSearchResponse struct {
	Paging   Paging    `json:"paging"`
	Measures []Measure `json:"measures"`
}

// Measure represents a single measure in the response
type Measure struct {
	Component string `json:"component"`
	Metric    string `json:"metric"`
	Value     string `json:"value"`
}
