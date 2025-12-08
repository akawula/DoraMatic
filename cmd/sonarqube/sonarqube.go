package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/akawula/DoraMatic/sonarqube/aggregator"
	"github.com/akawula/DoraMatic/sonarqube/client"
)

var (
	sonarURL     = flag.String("url", getEnvOrDefault("SONAR_URL", "https://sonarcloud.io"), "SonarQube/SonarCloud URL")
	sonarToken   = flag.String("token", os.Getenv("SONAR_TOKEN"), "SonarQube/SonarCloud authentication token")
	metricsFlag  = flag.String("metrics", "bugs,vulnerabilities,code_smells,coverage,duplicated_lines_density,ncloc,sqale_index,sqale_debt_ratio,reliability_rating,security_rating,sqale_rating", "Comma-separated list of metrics to fetch")
	outputFormat = flag.String("format", "summary", "Output format: summary, json, json-summary, or detailed")
	listProjects = flag.Bool("list-projects", false, "List all projects and exit")
)

func main() {
	flag.Parse()

	if *sonarToken == "" {
		log.Fatal("SONAR_TOKEN environment variable or -token flag is required")
	}

	ctx := context.Background()
	sonarClient := client.NewClient(*sonarURL, *sonarToken)

	// If list-projects flag is set, just list projects and exit
	if *listProjects {
		if err := listAllProjects(ctx, sonarClient); err != nil {
			log.Fatalf("Failed to list projects: %v", err)
		}
		return
	}

	// Parse metrics
	metricKeys := parseMetrics(*metricsFlag)

	fmt.Printf("Fetching metrics for all projects from %s...\n", *sonarURL)
	fmt.Printf("Metrics: %s\n\n", strings.Join(metricKeys, ", "))

	// Fetch metrics for all projects
	projectMetrics, err := sonarClient.GetAllProjectsMetrics(ctx, metricKeys)
	if err != nil {
		log.Fatalf("Failed to fetch project metrics: %v", err)
	}

	if len(projectMetrics) == 0 {
		fmt.Println("No projects found or no metrics available.")
		return
	}

	// Aggregate metrics
	aggregated := aggregator.AggregateMetrics(projectMetrics)

	// Output based on format
	switch *outputFormat {
	case "json":
		outputJSON(aggregated)
	case "json-summary":
		outputJSONSummary(aggregated)
	case "detailed":
		outputDetailed(aggregated)
	default:
		outputSummary(aggregated)
	}
}

func listAllProjects(ctx context.Context, sonarClient *client.SonarQubeClient) error {
	projects, err := sonarClient.GetAllProjects(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Found %d projects:\n\n", len(projects))
	for i, project := range projects {
		fmt.Printf("%3d. %-50s (key: %s)\n", i+1, project.Name, project.Key)
	}

	return nil
}

func parseMetrics(metricsStr string) []string {
	metrics := strings.Split(metricsStr, ",")
	for i := range metrics {
		metrics[i] = strings.TrimSpace(metrics[i])
	}
	return metrics
}

func outputSummary(aggregated *aggregator.AggregatedMetrics) {
	fmt.Println(aggregated.FormatSummary())
}

func outputJSON(aggregated *aggregator.AggregatedMetrics) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(aggregated); err != nil {
		log.Fatalf("Failed to encode JSON: %v", err)
	}
}

func outputJSONSummary(aggregated *aggregator.AggregatedMetrics) {
	averages := aggregated.CalculateAverages()

	// Add status assessments for each average metric
	metricAssessments := make(map[string]map[string]interface{})
	for metric, avg := range averages {
		status := aggregator.GetMetricStatus(metric, avg)
		metricAssessments[metric] = map[string]interface{}{
			"value":  avg,
			"status": status,
		}
	}

	summary := map[string]interface{}{
		"totalProjects":      aggregated.TotalProjects,
		"metricAggregations": aggregated.MetricAggregations,
		"metricAverages":     averages,
		"metricAssessments":  metricAssessments,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		log.Fatalf("Failed to encode JSON: %v", err)
	}
}

func outputDetailed(aggregated *aggregator.AggregatedMetrics) {
	fmt.Println(aggregated.FormatSummary())
	fmt.Println("\nProject Breakdown:")
	fmt.Println("==================")

	for _, project := range aggregated.ProjectBreakdown {
		fmt.Printf("\nProject: %s (%s)\n", project.ProjectName, project.ProjectKey)
		for metric, value := range project.Metrics {
			fmt.Printf("  %-30s: %.2f\n", metric, value)
		}
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
