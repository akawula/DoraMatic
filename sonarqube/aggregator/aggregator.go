package aggregator

import (
	"fmt"

	"github.com/akawula/DoraMatic/sonarqube/client"
)

// AggregatedMetrics represents aggregated metrics across all projects
type AggregatedMetrics struct {
	TotalProjects      int                `json:"totalProjects"`
	MetricAggregations map[string]float64 `json:"metricAggregations"`
	ProjectBreakdown   []ProjectSummary   `json:"projectBreakdown"`
}

// MetricThreshold defines good/bad thresholds for a metric
type MetricThreshold struct {
	Good      float64
	Excellent float64
	Poor      float64
}

// MetricThresholds defines quality thresholds for various metrics
var MetricThresholds = map[string]MetricThreshold{
	// Average bugs per project
	"bugs": {
		Excellent: 0,
		Good:      5,
		Poor:      20,
	},
	// Average vulnerabilities per project
	"vulnerabilities": {
		Excellent: 0,
		Good:      2,
		Poor:      10,
	},
	// Average code smells per project
	"code_smells": {
		Excellent: 0,
		Good:      100,
		Poor:      500,
	},
	// Coverage percentage (average)
	"coverage": {
		Excellent: 80,
		Good:      60,
		Poor:      40,
	},
	// Duplicated lines density (average)
	"duplicated_lines_density": {
		Excellent: 0,
		Good:      3,
		Poor:      10,
	},
	// Technical debt ratio (average)
	"sqale_debt_ratio": {
		Excellent: 0,
		Good:      5,
		Poor:      20,
	},
}

// GetMetricStatus returns the quality status of a metric value
func GetMetricStatus(metric string, value float64) string {
	threshold, exists := MetricThresholds[metric]
	if !exists {
		return "unknown"
	}

	// For coverage, higher is better
	if metric == "coverage" {
		if value >= threshold.Excellent {
			return "excellent"
		} else if value >= threshold.Good {
			return "good"
		} else if value >= threshold.Poor {
			return "fair"
		}
		return "poor"
	}

	// For other metrics, lower is better
	if value <= threshold.Excellent {
		return "excellent"
	} else if value <= threshold.Good {
		return "good"
	} else if value <= threshold.Poor {
		return "fair"
	}
	return "poor"
}

// ProjectSummary represents a summary of a single project's metrics
type ProjectSummary struct {
	ProjectKey  string             `json:"projectKey"`
	ProjectName string             `json:"projectName"`
	Metrics     map[string]float64 `json:"metrics"`
}

// AggregateMetrics aggregates metrics from multiple projects
func AggregateMetrics(projectMetrics []client.ProjectMetrics) *AggregatedMetrics {
	aggregated := &AggregatedMetrics{
		TotalProjects:      len(projectMetrics),
		MetricAggregations: make(map[string]float64),
		ProjectBreakdown:   make([]ProjectSummary, 0, len(projectMetrics)),
	}

	// Iterate through all projects and aggregate metrics
	for _, pm := range projectMetrics {
		// Create project summary
		summary := ProjectSummary{
			ProjectKey:  pm.ProjectKey,
			ProjectName: pm.ProjectName,
			Metrics:     make(map[string]float64),
		}

		for metricKey, metric := range pm.Metrics {
			// Add to project summary
			summary.Metrics[metricKey] = metric.Value

			// Add to aggregated totals
			aggregated.MetricAggregations[metricKey] += metric.Value
		}

		aggregated.ProjectBreakdown = append(aggregated.ProjectBreakdown, summary)
	}

	return aggregated
}

// CalculateAverages calculates average values for metrics
func (a *AggregatedMetrics) CalculateAverages() map[string]float64 {
	averages := make(map[string]float64)
	if a.TotalProjects == 0 {
		return averages
	}

	for metric, total := range a.MetricAggregations {
		averages[metric] = total / float64(a.TotalProjects)
	}

	return averages
}

// FormatSummary returns a human-readable summary of the aggregated metrics
func (a *AggregatedMetrics) FormatSummary() string {
	summary := fmt.Sprintf("Total Projects: %d\n\n", a.TotalProjects)
	summary += "Aggregated Metrics (Total across all projects):\n"
	summary += "================================================\n"

	// Common metrics with descriptions
	metricDescriptions := map[string]string{
		"bugs":                     "Total Bugs",
		"vulnerabilities":          "Total Vulnerabilities",
		"code_smells":              "Total Code Smells",
		"ncloc":                    "Total Lines of Code",
		"coverage":                 "Coverage (Total)",
		"duplicated_lines_density": "Duplicated Lines Density (Total)",
		"sqale_index":              "Technical Debt (minutes)",
		"sqale_debt_ratio":         "Technical Debt Ratio (Total)",
		"reliability_rating":       "Reliability Rating (Total)",
		"security_rating":          "Security Rating (Total)",
		"sqale_rating":             "Maintainability Rating (Total)",
		"new_bugs":                 "New Bugs",
		"new_vulnerabilities":      "New Vulnerabilities",
		"new_code_smells":          "New Code Smells",
		"new_coverage":             "New Coverage (Total)",
		"new_lines":                "New Lines of Code",
	}

	for metric, total := range a.MetricAggregations {
		description := metricDescriptions[metric]
		if description == "" {
			description = metric
		}
		summary += fmt.Sprintf("  %-35s: %.2f\n", description, total)
	}

	// Calculate and display averages for relevant metrics
	averages := a.CalculateAverages()
	if len(averages) > 0 {
		summary += "\nAverage Metrics (per project):\n"
		summary += "================================\n"

		avgMetrics := []string{"bugs", "vulnerabilities", "code_smells", "coverage", "duplicated_lines_density", "sqale_debt_ratio"}
		for _, metric := range avgMetrics {
			if avg, exists := averages[metric]; exists {
				description := metricDescriptions[metric]
				if description == "" {
					description = metric
				}
				status := GetMetricStatus(metric, avg)
				statusSymbol := getStatusSymbol(status)
				summary += fmt.Sprintf("  %-35s: %7.2f  %s %s\n", description, avg, statusSymbol, status)
			}
		}
	}

	return summary
}

// getStatusSymbol returns a visual symbol for the status
func getStatusSymbol(status string) string {
	switch status {
	case "excellent":
		return "✓✓"
	case "good":
		return "✓ "
	case "fair":
		return "⚠ "
	case "poor":
		return "✗ "
	default:
		return "? "
	}
}
