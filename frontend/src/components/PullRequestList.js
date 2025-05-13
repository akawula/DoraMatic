import React, { useState, useMemo } from "react";
import Table from "@mui/joy/Table";
import Input from "@mui/joy/Input";
import Link from "@mui/joy/Link";
import Typography from "@mui/joy/Typography";
import CircularProgress from "@mui/joy/CircularProgress";
import Box from "@mui/joy/Box";
import Chip from "@mui/joy/Chip";

// Helper function to format lead time in seconds to a readable string
const formatLeadTime = (seconds) => {
  if (seconds === null || seconds === undefined || typeof seconds !== 'number') {
    return "N/A";
  }
  if (seconds === 0) {
    return "0 sec";
  }
  if (seconds < 60) {
    return seconds.toFixed(0) + " sec";
  } else if (seconds < 3600) {
    return (seconds / 60).toFixed(1) + " min";
  } else if (seconds < 86400) {
    return (seconds / 3600).toFixed(1) + " hr";
  } else {
    return (seconds / 86400).toFixed(1) + " days";
  }
};


const getStateColor = (state) => {
  switch (state?.toUpperCase()) {
    case "OPEN": return "success";
    case "MERGED": return "primary";
    case "CLOSED": return "danger";
    default: return "neutral";
  }
};

const sizeThresholds = { xs: 10, s: 100, m: 500, l: 1000 };

const getPrSizeLabel = (additions, deletions) => {
  const totalChanges = (additions || 0) + (deletions || 0);
  if (totalChanges <= sizeThresholds.xs) return "XS";
  if (totalChanges <= sizeThresholds.s) return "S";
  if (totalChanges <= sizeThresholds.m) return "M";
  if (totalChanges <= sizeThresholds.l) return "L";
  return "XL";
};

function PullRequestList({
  pullRequests,
  loading,
  error,
  selectedTeam,
  fetchAttempted,
}) {
  const [searchTerm, setSearchTerm] = useState("");
  const [sortConfig, setSortConfig] = useState({ key: "created_at", direction: "descending" });

  const filteredPullRequests = useMemo(() => {
    if (!pullRequests) return [];
    let filtered = [...pullRequests];
    if (searchTerm) {
      const lowerSearchTerm = searchTerm.toLowerCase();
      filtered = filtered.filter(
        (pr) =>
          pr.title?.toLowerCase().includes(lowerSearchTerm) ||
          pr.author?.toLowerCase().includes(lowerSearchTerm) ||
          pr.repo_name?.toLowerCase().includes(lowerSearchTerm) ||
          pr.state?.toLowerCase().includes(lowerSearchTerm) ||
          String(pr.id).includes(lowerSearchTerm) ||
          (pr.jira_references && pr.jira_references.some(ref => ref.toLowerCase().includes(lowerSearchTerm)))
      );
    }
    return filtered;
  }, [pullRequests, searchTerm]);

  const sortedPullRequests = useMemo(() => {
    let sortableItems = [...filteredPullRequests];
    if (sortConfig.key !== null) {
      sortableItems.sort((a, b) => {
        const aValue = a[sortConfig.key];
        const bValue = b[sortConfig.key];
        if (["created_at", "merged_at"].includes(sortConfig.key)) {
          const dateA = aValue ? new Date(aValue.Time || aValue).getTime() : 0;
          const dateB = bValue ? new Date(bValue.Time || bValue).getTime() : 0;
          return sortConfig.direction === "ascending" ? dateA - dateB : dateB - dateA;
        }
        if (["lead_time_to_code_seconds", "lead_time_to_review_seconds", "lead_time_to_merge_seconds", "pr_reviews_requested_count"].includes(sortConfig.key)) {
          // Handle null or undefined values by treating them as very large or very small
          // depending on sort direction, to push them to the end/start.
          // For pr_reviews_requested_count, treat null/undefined as 0 for sorting.
          let valA = aValue;
          let valB = bValue;

          if (sortConfig.key === "pr_reviews_requested_count") {
            valA = aValue === null || aValue === undefined ? 0 : aValue;
            valB = bValue === null || bValue === undefined ? 0 : bValue;
          } else { // For lead time fields
            valA = aValue === null || aValue === undefined ? (sortConfig.direction === "ascending" ? Infinity : -Infinity) : aValue;
            valB = bValue === null || bValue === undefined ? (sortConfig.direction === "ascending" ? Infinity : -Infinity) : bValue;
          }
          return sortConfig.direction === "ascending" ? valA - valB : valB - valA;
        }
        if (sortConfig.key === "jira_references") {
          const refsA = (a.jira_references || []).join(", ").toLowerCase();
          const refsB = (b.jira_references || []).join(", ").toLowerCase();
          if (refsA < refsB) return sortConfig.direction === "ascending" ? -1 : 1;
          if (refsA > refsB) return sortConfig.direction === "ascending" ? 1 : -1;
          return 0;
        }
        if (sortConfig.key === "state") {
          const stateA = aValue?.String?.toLowerCase() || "";
          const stateB = bValue?.String?.toLowerCase() || "";
          if (stateA < stateB) return sortConfig.direction === "ascending" ? -1 : 1;
          if (stateA > stateB) return sortConfig.direction === "ascending" ? 1 : -1;
          return 0;
        }
        if (sortConfig.key === "size") {
          const sizeA = (a.additions || 0) + (a.deletions || 0);
          const sizeB = (b.additions || 0) + (b.deletions || 0);
          return sortConfig.direction === "ascending" ? sizeA - sizeB : sizeB - sizeA;
        }
        // Default sort for other string types or numbers
        const valA = typeof aValue === "string" ? aValue.toLowerCase() : aValue;
        const valB = typeof bValue === "string" ? bValue.toLowerCase() : bValue;

        if (valA === null || valA === undefined) return sortConfig.direction === "ascending" ? 1 : -1; // push nulls/undefined to end
        if (valB === null || valB === undefined) return sortConfig.direction === "ascending" ? -1 : 1; // push nulls/undefined to end


        if (valA < valB) return sortConfig.direction === "ascending" ? -1 : 1;
        if (valA > valB) return sortConfig.direction === "ascending" ? 1 : -1;
        return 0;
      });
    }
    return sortableItems;
  }, [filteredPullRequests, sortConfig]);

  if (!selectedTeam || !fetchAttempted) {
    return null;
  }

  if (loading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", my: 2 }}>
        <CircularProgress size="sm" />
        <Typography sx={{ ml: 1 }}>Loading pull requests...</Typography>
      </Box>
    );
  }

  if (error) {
    // Error is displayed globally in App.js, so no need for specific message here
    return null;
  }

  const requestSort = (key) => {
    let direction = "ascending";
    if (sortConfig.key === key && sortConfig.direction === "ascending") {
      direction = "descending";
    }
    setSortConfig({ key, direction });
  };

  const getSortIndicator = (key) => {
    if (sortConfig.key !== key) return ""; // Return empty string instead of null
    return sortConfig.direction === "ascending" ? " ▲" : " ▼";
  };

  // This is the main rendering structure
  return (
    <Box sx={{ my: 2 }}>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          mb: 1,
        }}
      >
        <Typography level="title-md">
          Pull Requests ({pullRequests ? pullRequests.length : 0})
        </Typography>
        <Input
          size="sm"
          placeholder="Search PRs (Title, Author, Repo, State, ID)..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          sx={{ width: "300px" }} // Increased width for longer placeholder
        />
      </Box>

      {(!pullRequests || pullRequests.length === 0) && !loading && fetchAttempted && (
        <Typography sx={{ my: 2, textAlign: 'center' }}>
          No pull requests found for the selected criteria.
        </Typography>
      )}

      {pullRequests && pullRequests.length > 0 && sortedPullRequests.length === 0 && (
        <Typography sx={{ my: 2, textAlign: 'center' }}>
          No pull requests match your search term "{searchTerm}".
        </Typography>
      )}

      {pullRequests && pullRequests.length > 0 && sortedPullRequests.length > 0 && (
        <Box
          sx={{
            maxHeight: 400,
            overflow: "auto",
            border: "1px solid",
            borderColor: "divider",
            borderRadius: "sm",
          }}
        >
          <Table
            aria-label="Pull Request Table"
            stickyHeader
            hoverRow
            sx={{
          "& thead th": { fontWeight: "lg", cursor: "pointer" },
          "& tbody td": { verticalAlign: "top" },
          "--TableCell-paddingX": "0.75rem",
          "--TableCell-paddingY": "0.5rem",
        }}
      >
        <thead>
          <tr>
            <th onClick={() => requestSort("title")} style={{ width: "20%" }}>Title{getSortIndicator("title")}</th>
            <th onClick={() => requestSort("author")} style={{ width: "10%" }}>Author{getSortIndicator("author")}</th>
            <th onClick={() => requestSort("repo_name")} style={{ width: "12%" }}>Repository{getSortIndicator("repo_name")}</th>
            <th onClick={() => requestSort("jira_references")} style={{ width: "10%" }}>Jira Keys{getSortIndicator("jira_references")}</th>
            <th onClick={() => requestSort("pr_reviews_requested_count")} style={{ width: "8%" }}>Reviews Req.{getSortIndicator("pr_reviews_requested_count")}</th>
            <th onClick={() => requestSort("lead_time_to_code_seconds")} style={{ width: "9%" }}>LT Code{getSortIndicator("lead_time_to_code_seconds")}</th>
            <th onClick={() => requestSort("lead_time_to_review_seconds")} style={{ width: "9%" }}>LT Review{getSortIndicator("lead_time_to_review_seconds")}</th>
            <th onClick={() => requestSort("lead_time_to_merge_seconds")} style={{ width: "9%" }}>LT Merge{getSortIndicator("lead_time_to_merge_seconds")}</th>
            <th onClick={() => requestSort("state")} style={{ width: "7%" }}>State{getSortIndicator("state")}</th>
            <th onClick={() => requestSort("size")} style={{ width: "6%" }}>Size{getSortIndicator("size")}</th>
          </tr>
        </thead>
        <tbody>
          {sortedPullRequests.map((pr) => (
            <tr key={pr.id}>
              <td>
                <Link href={pr.url || "#"} target="_blank" rel="noopener noreferrer" level="title-sm">
                  {pr.title || "No Title"}
                </Link>
              </td>
              <td>{pr.author || "Unknown"}</td>
              <td>{pr.repo_name || "N/A"}</td>
              <td>
                {pr.jira_references && pr.jira_references.length > 0
                  ? pr.jira_references.map((ref, index) => {
                      const jiraKey = ref.substring(ref.lastIndexOf("/") + 1);
                      return (
                        <React.Fragment key={ref}>
                          <Link href={ref} target="_blank" rel="noopener noreferrer">
                            {jiraKey}
                          </Link>
                          {index < pr.jira_references.length - 1 ? ", " : ""}
                        </React.Fragment>
                      );
                    })
                  : "N/A"}
              </td>
              <td>{pr.pr_reviews_requested_count === null || pr.pr_reviews_requested_count === undefined ? "-" : pr.pr_reviews_requested_count}</td>
              <td>{formatLeadTime(pr.lead_time_to_code_seconds)}</td>
              <td>{formatLeadTime(pr.lead_time_to_review_seconds)}</td>
              <td>{formatLeadTime(pr.lead_time_to_merge_seconds)}</td>
              <td>
                <Chip size="sm" variant="soft" color={getStateColor(pr.state)}>
                  {pr.state || "Unknown"}
                </Chip>
              </td>
              <td>{getPrSizeLabel(pr.additions, pr.deletions)}</td>
            </tr>
          ))}
        </tbody>
          </Table>
        </Box>
      )}

      {pullRequests && pullRequests.length > 0 && sortedPullRequests.length > 0 && (
        <Box sx={{ mt: 2, p: 2, border: '1px solid', borderColor: 'divider', borderRadius: 'sm' }}>
          <Typography level="title-sm" sx={{ mb: 1 }}>Legend:</Typography>
          <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 1 }}>
            <Typography level="body-xs"><strong>LT Code:</strong> Lead Time to Code - Time from first commit to PR creation.</Typography>
            <Typography level="body-xs"><strong>LT Review:</strong> Lead Time to Review - Time from PR creation to first review.</Typography>
            <Typography level="body-xs"><strong>LT Merge:</strong> Lead Time to Merge - Time from PR creation to merge.</Typography>
            <Typography level="body-xs"><strong>Reviews Req.:</strong> Number of reviews requested for the PR.</Typography>
            <Typography level="body-xs"><strong>Size:</strong> PR size based on total changes (XS, S, M, L, XL).</Typography>
          </Box>
        </Box>
      )}
    </Box>
  );
}

export default PullRequestList;
