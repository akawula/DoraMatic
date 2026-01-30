package pullrequests

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/akawula/DoraMatic/github/client"
)

const (
	// DefaultFilesThreshold is the default maximum number of files to fetch per PR.
	// PRs with more files than this will have files_complete=false.
	DefaultFilesThreshold = 500
)

// FileFetchResult contains the result of fetching files for a PR.
type FileFetchResult struct {
	Files              []FileChange
	TotalFiles         int  // Total number of files in the PR (from changed_files count)
	FilesComplete      bool // Whether all files were fetched
	GeneratedAdditions int  // Sum of additions in generated files
	GeneratedDeletions int  // Sum of deletions in generated files
	HumanAdditions     int  // Sum of additions in human-written files
	HumanDeletions     int  // Sum of deletions in human-written files
	Error              error
}

// FileFetcher handles fetching and classifying PR files.
type FileFetcher struct {
	restClient *client.RESTClient
	classifier *FileClassifier
	threshold  int
	logger     *slog.Logger
}

// NewFileFetcher creates a new FileFetcher with default settings.
func NewFileFetcher(logger *slog.Logger) *FileFetcher {
	threshold := DefaultFilesThreshold
	if envThreshold := os.Getenv("PR_FILES_THRESHOLD"); envThreshold != "" {
		if t, err := strconv.Atoi(envThreshold); err == nil && t > 0 {
			threshold = t
		}
	}

	return &FileFetcher{
		restClient: client.NewRESTClient(),
		classifier: NewFileClassifier(),
		threshold:  threshold,
		logger:     logger,
	}
}

// NewFileFetcherWithThreshold creates a FileFetcher with a custom threshold.
func NewFileFetcherWithThreshold(threshold int, logger *slog.Logger) *FileFetcher {
	return &FileFetcher{
		restClient: client.NewRESTClient(),
		classifier: NewFileClassifier(),
		threshold:  threshold,
		logger:     logger,
	}
}

// FetchAndClassifyFiles fetches files for a PR and classifies them.
// If changedFilesCount exceeds the threshold, it skips fetching and marks as incomplete.
func (f *FileFetcher) FetchAndClassifyFiles(ctx context.Context, owner, repo string, prNumber int, changedFilesCount int) FileFetchResult {
	result := FileFetchResult{
		TotalFiles:    changedFilesCount,
		FilesComplete: true,
	}

	// Check threshold before fetching
	if changedFilesCount > f.threshold {
		f.logger.Info("PR exceeds file threshold, skipping file-level fetch",
			"owner", owner,
			"repo", repo,
			"pr", prNumber,
			"changed_files", changedFilesCount,
			"threshold", f.threshold)
		result.FilesComplete = false
		return result
	}

	// Fetch files from GitHub REST API
	files, err := f.restClient.GetPullRequestFiles(ctx, owner, repo, prNumber, f.logger)
	if err != nil {
		f.logger.Error("Failed to fetch PR files",
			"owner", owner,
			"repo", repo,
			"pr", prNumber,
			"error", err)
		result.Error = err
		result.FilesComplete = false
		return result
	}

	// Convert and classify files
	for _, file := range files {
		fc := FileChange{
			Path:      file.Filename,
			Additions: file.Additions,
			Deletions: file.Deletions,
			Status:    file.Status,
		}
		fc.IsGenerated = f.classifier.IsGenerated(file.Filename)

		if fc.IsGenerated {
			result.GeneratedAdditions += file.Additions
			result.GeneratedDeletions += file.Deletions
		} else {
			result.HumanAdditions += file.Additions
			result.HumanDeletions += file.Deletions
		}

		result.Files = append(result.Files, fc)
	}

	// Check if we hit the API limit
	if len(files) >= 3000 && changedFilesCount > 3000 {
		f.logger.Warn("PR has more files than API limit allows",
			"owner", owner,
			"repo", repo,
			"pr", prNumber,
			"fetched", len(files),
			"total", changedFilesCount)
		result.FilesComplete = false
	}

	f.logger.Debug("Classified PR files",
		"owner", owner,
		"repo", repo,
		"pr", prNumber,
		"total_files", len(result.Files),
		"generated_files", countGenerated(result.Files),
		"generated_additions", result.GeneratedAdditions,
		"human_additions", result.HumanAdditions)

	return result
}

// ExtractPRNumber extracts the PR number from a GitHub PR URL.
// URL format: https://github.com/owner/repo/pull/123
func ExtractPRNumber(prURL string) int {
	parts := strings.Split(prURL, "/")
	if len(parts) < 2 {
		return 0
	}
	// The PR number is the last part of the URL
	prNumStr := parts[len(parts)-1]
	prNum, err := strconv.Atoi(prNumStr)
	if err != nil {
		return 0
	}
	return prNum
}

// countGenerated counts the number of generated files in a slice.
func countGenerated(files []FileChange) int {
	count := 0
	for _, f := range files {
		if f.IsGenerated {
			count++
		}
	}
	return count
}

// GetThreshold returns the current file threshold.
func (f *FileFetcher) GetThreshold() int {
	return f.threshold
}
