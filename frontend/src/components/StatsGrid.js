import React from "react";
import Box from "@mui/joy/Box";
import Grid from "@mui/joy/Grid";
import Card from "@mui/joy/Card";
import CardContent from "@mui/joy/CardContent";
import Typography from "@mui/joy/Typography";
import CircularProgress from "@mui/joy/CircularProgress";
import ArrowUpwardIcon from "@mui/icons-material/ArrowUpward";
import ArrowDownwardIcon from "@mui/icons-material/ArrowDownward";
import RemoveIcon from "@mui/icons-material/Remove";
import { LineChart, Line, Tooltip, ResponsiveContainer, YAxis } from "recharts";

// Thresholds for PR size categorization
const sizeThresholds = { xs: 10, s: 100, m: 500, l: 1000 };

// Helper function to get PR size label
const getPrSizeLabel = (totalChanges) => {
  if (totalChanges === null || totalChanges === undefined) return "";
  if (totalChanges <= sizeThresholds.xs) return "XS";
  if (totalChanges <= sizeThresholds.s) return "S";
  if (totalChanges <= sizeThresholds.m) return "M";
  if (totalChanges <= sizeThresholds.l) return "L";
  return "XL";
};

// Helper function to format lead time in seconds for display
const formatLeadTimeDisplay = (seconds) => {
  if (
    seconds === null ||
    seconds === undefined ||
    typeof seconds !== "number"
  ) {
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

const MiniTrendChart = ({ data, dataKey, isLeadTime }) => {
  if (!data || data.length < 2) {
    return (
      <Box
        sx={{
          height: 60,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <Typography level="body-xs" color="neutral">
          Not enough data for trend
        </Typography>
      </Box>
    );
  }

  const chartData = data.map((value, index) => ({ name: `P${index}`, value }));

  // Determine a contrasting color for the line based on Joy UI theme
  // This is a simplified example; you might need a more robust way to access theme colors
  const lineColor = "#1976d2"; // Example: Joy UI primary color

  return (
    <Box sx={{ height: 60, width: "100%", mt: 1 }}>
      <ResponsiveContainer>
        <LineChart
          data={chartData}
          margin={{ top: 5, right: 5, left: 5, bottom: 5 }}
        >
          <Tooltip
            contentStyle={{
              backgroundColor: "rgba(255, 255, 255, 0.8)",
              borderRadius: "4px",
              fontSize: "12px",
              padding: "4px 8px",
            }}
            formatter={(value) =>
              isLeadTime ? formatLeadTimeDisplay(value) : value.toLocaleString()
            }
          />
          <YAxis
            hide
            domain={["dataMin - dataMin * 0.1", "dataMax + dataMax * 0.1"]}
          />
          <Line
            type="monotone"
            dataKey="value"
            stroke={lineColor}
            strokeWidth={2}
            dot={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </Box>
  );
};

function StatsGrid({ stats, loadingStats, selectedTeam, startDate, endDate }) {
  if (loadingStats) {
    return (
      <CircularProgress sx={{ display: "block", margin: "auto", my: 3 }} />
    );
  }

  if (!stats) {
    return null; // Don't render anything if stats aren't loaded yet or invalid
  }

  // Helper function to calculate percentage change and determine icon/color
  const calculateChange = (current, previous, label) => {
    if (
      previous === null ||
      current === null ||
      previous === undefined ||
      current === undefined
    ) {
      return { percentage: null, icon: null, color: "neutral" }; // Use "neutral"
    }
    if (previous === 0) {
      if (current > 0) {
        // Determine color based on label rules for "up is good" or "up is bad"
        let color = "success"; // Default to green if up is good
        if (
          label === "Closed PRs (Not Merged)" ||
          label === "Rollbacks" ||
          label === "Avg lead time to code" ||
          label === "Avg Lead Time to Review" ||
          label === "Avg Lead Time to Merge" ||
          label === "Avg. Time to First Actual Review" ||
          label === "Avg Reviews Requested / PR" || // Changed metric
          label === "Avg PR Size (Lines)" ||
          label === "Change Failure Rate"
        ) {
          color = "danger"; // Red if up is bad for these metrics
        }
        return { percentage: 100, icon: <ArrowUpwardIcon />, color };
      }
      return { percentage: 0, icon: <RemoveIcon />, color: "neutral" }; // No change from zero, use "neutral"
    }

    // For "Top Reviewer", we don't calculate percentage change of the stat itself,
    // as the hero can change. We'll just show current/previous.
    if (label === "Top Reviewer (Quiet Hero)") {
      // We could compare the review counts if the hero is the same, but that's complex.
      // For now, no percentage change display for this card.
      // We can show previous hero's stat if available.
      return { percentage: null, icon: null, color: "neutral" };
    }

    const change = ((current - previous) / previous) * 100;
    const percentage = Math.abs(change).toFixed(1); // Keep one decimal place

    let icon;
    let color = "neutral"; // Default for no change, use "neutral"

    if (change > 0) {
      icon = <ArrowUpwardIcon />;
      switch (label) {
        case "Merged PRs":
        case "Total Commits":
        case "Avg Commits / PR":
          // case "Avg Reviews per User": // Replaced by Top Reviewer
          color = "success"; // Green if trending up is good
          break;
        case "Closed PRs (Not Merged)":
        case "Rollbacks":
        case "Avg lead time to code":
        case "Avg Lead Time to Review":
        case "Avg Lead Time to Merge":
        case "Avg. Time to First Actual Review":
        case "Avg Reviews Requested / PR": // Changed metric
        case "Avg PR Size (Lines)":
        case "Change Failure Rate":
          color = "danger"; // Red if trending up is bad
          break;
        default:
          color = "success"; // Default to green for upward trend
      }
    } else if (change < 0) {
      icon = <ArrowDownwardIcon />;
      switch (label) {
        case "Merged PRs":
        case "Total Commits":
        case "Avg Commits / PR":
          // case "Avg Reviews per User": // Replaced by Top Reviewer
          color = "danger"; // Red if trending down is bad
          break;
        case "Closed PRs (Not Merged)":
        case "Rollbacks":
        case "Avg lead time to code":
        case "Avg Lead Time to Review":
        case "Avg Lead Time to Merge":
        case "Avg. Time to First Actual Review":
        case "Avg Reviews Requested / PR": // Changed metric
        case "Avg PR Size (Lines)":
        case "Change Failure Rate":
          color = "success"; // Green if trending down is good
          break;
        default:
          color = "danger"; // Default to red for downward trend
      }
    } else {
      icon = <RemoveIcon />;
      color = "neutral"; // No change, use "neutral"
    }
    return { percentage: change === 0 ? 0 : percentage, icon, color };
  };

  const statsItems = [
    {
      label: "Merged PRs",
      current: stats.current?.merged_prs_count,
      previous: stats.previous?.merged_prs_count,
      trendKey: "merged_prs_count",
      isLeadTime: false,
    },
    {
      label: "Avg Reviews Requested / PR", // Changed label
      current: stats.current?.avg_reviews_requested_per_pr, // Changed data key
      previous: stats.previous?.avg_reviews_requested_per_pr, // Changed data key
      trendKey: "avg_reviews_requested_per_pr", // Changed trend key
      isLeadTime: false, // This is a count, not a lead time
    },
    {
      label: "Total Commits",
      current: stats.current?.commits_count,
      previous: stats.previous?.commits_count,
      trendKey: "commits_count",
      isLeadTime: false,
    },
    {
      label: "Rollbacks",
      current: stats.current?.rollbacks_count,
      previous: stats.previous?.rollbacks_count,
      trendKey: "rollbacks_count",
      isLeadTime: false,
    },
    {
      label: "Avg lead time to code",
      current: stats.current?.avg_lead_time_to_code_seconds,
      previous: stats.previous?.avg_lead_time_to_code_seconds,
      trendKey: "avg_lead_time_to_code_seconds",
      isLeadTime: true,
    },
    {
      label: "Avg. Time to First Actual Review",
      current: stats.current?.avg_time_to_first_actual_review_seconds,
      previous: stats.previous?.avg_time_to_first_actual_review_seconds,
      trendKey: "avg_time_to_first_actual_review_seconds",
      isLeadTime: true,
    },
    {
      label: "Avg Lead Time to Review",
      current: stats.current?.avg_lead_time_to_review_seconds,
      previous: stats.previous?.avg_lead_time_to_review_seconds,
      trendKey: "avg_lead_time_to_review_seconds",
      isLeadTime: true,
    },
    {
      label: "Avg Lead Time to Merge",
      current: stats.current?.avg_lead_time_to_merge_seconds,
      previous: stats.previous?.avg_lead_time_to_merge_seconds,
      trendKey: "avg_lead_time_to_merge_seconds",
      isLeadTime: true,
    },
    {
      label: "Avg PR Size (Lines)",
      current: stats.current?.avg_pr_size_lines,
      previous: stats.previous?.avg_pr_size_lines,
      trendKey: "avg_pr_size_lines",
      isLeadTime: false, // It's a count of lines
    },
    {
      label: "Change Failure Rate",
      current: stats.current?.change_failure_rate_percentage,
      previous: stats.previous?.change_failure_rate_percentage,
      trendKey: "change_failure_rate_percentage",
      isLeadTime: false, // It's a percentage
      isPercentage: true, // Custom flag to handle '%' display
    },
    {
      label: "Avg Commits / PR",
      current: stats.current?.avg_commits_per_merged_pr,
      previous: stats.previous?.avg_commits_per_merged_pr,
      trendKey: "avg_commits_per_merged_pr",
      isLeadTime: false,
    },
    {
      label: "Top Reviewer (Quiet Hero)",
      // Current value will be an object { name: string, stat: number } or similar from backend
      // For now, access them directly:
      currentName: stats.current?.quiet_hero_name,
      currentStat: stats.current?.quiet_hero_stat,
      previousName: stats.previous?.quiet_hero_name,
      previousStat: stats.previous?.quiet_hero_stat,
      // trendKey: "quiet_hero_stat", // Trend for hero's stat could be added if backend supports it
      trendKey: null, // No trend chart for this composite display for now
      isLeadTime: false,
      isQuietHero: true, // Custom flag for special rendering
    },
  ];

  return (
    <Box sx={{ mt: 3 }}>
      <Typography
        level="body-xs"
        sx={{ mb: 2, textAlign: "center", fontStyle: "italic" }}
      >
        Note: Trend graphs show data for the last 6 periods, including the
        current one.
      </Typography>
      <Grid container spacing={2} sx={{ flexGrow: 1 }}>
        {statsItems.map((item, index) => {
          const changeInfo = calculateChange(
            item.current,
            item.previous,
            item.label
          );
          const trendData =
            stats.trend && stats.trend[item.trendKey]
              ? stats.trend[item.trendKey]
              : [];

          let currentValueDisplay = "N/A";
          let previousValueText = "";

          if (item.isLeadTime) {
            currentValueDisplay = formatLeadTimeDisplay(item.current);
            previousValueText =
              item.previous !== undefined && item.previous !== null
                ? `(Prev: ${formatLeadTimeDisplay(item.previous)})`
                : "";
          } else if (item.isPercentage) {
            currentValueDisplay =
              typeof item.current === "number"
                ? `${item.current.toFixed(1)}%`
                : "N/A";
            previousValueText =
              typeof item.previous === "number"
                ? `(Prev: ${item.previous.toFixed(1)}%)`
                : "";
          } else if (item.label === "Avg PR Size (Lines)") {
            const currentSize =
              typeof item.current === "number"
                ? item.current.toFixed(1)
                : "N/A";
            const currentLabel =
              typeof item.current === "number"
                ? getPrSizeLabel(item.current)
                : "";
            currentValueDisplay = `${currentSize}${
              currentLabel ? ` (${currentLabel})` : ""
            }`;

            const prevSize =
              typeof item.previous === "number" ? item.previous.toFixed(1) : "";
            const prevLabel =
              typeof item.previous === "number"
                ? getPrSizeLabel(item.previous)
                : "";
            previousValueText = prevSize
              ? `(Prev: ${prevSize}${prevLabel ? ` (${prevLabel})` : ""})`
              : "";
          } else if (item.isQuietHero) {
            currentValueDisplay = item.currentName
              ? `${item.currentName} (${
                  item.currentStat !== undefined
                    ? item.currentStat.toLocaleString()
                    : "N/A"
                } reviews)`
              : "N/A";
            previousValueText = item.previousName
              ? `(Prev: ${item.previousName} (${
                  item.previousStat !== undefined
                    ? item.previousStat.toLocaleString()
                    : "N/A"
                } reviews))`
              : "";
            // No percentage change for this card, so changeInfo might be null
          } else {
            currentValueDisplay =
              typeof item.current === "number"
                ? item.current.toLocaleString(undefined, {
                    maximumFractionDigits: 1,
                  })
                : "N/A";
            previousValueText =
              typeof item.previous === "number"
                ? `(Prev: ${item.previous.toLocaleString(undefined, {
                    maximumFractionDigits: 1,
                  })})`
                : "";
          }

          return (
            <Grid xs={12} sm={6} md={4} lg={3} key={index}>
              {" "}
              {/* Adjusted grid for potentially more items or wider cards */}
              <Card
                variant="outlined"
                sx={{
                  height: "100%",
                  display: "flex",
                  flexDirection: "column",
                }}
              >
                <CardContent sx={{ textAlign: "center", flexGrow: 1 }}>
                  <Typography level="title-md" gutterBottom>
                    {item.label}
                  </Typography>
                  <Typography level="h2">{currentValueDisplay}</Typography>
                  {changeInfo.percentage !== null && changeInfo.icon && (
                    <Box
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        mt: 0.5,
                      }}
                    >
                      {React.cloneElement(changeInfo.icon, {
                        fontSize: "small",
                        color:
                          changeInfo.color === "danger"
                            ? "error"
                            : changeInfo.color === "neutral"
                            ? "inherit"
                            : changeInfo.color,
                      })}
                      <Typography
                        level="body-sm"
                        sx={{ ml: 0.5 }}
                        color={changeInfo.color}
                      >
                        {changeInfo.percentage}%
                      </Typography>
                    </Box>
                  )}
                  <Typography
                    level="body-xs"
                    sx={{ color: "text.tertiary", mt: 0.5 }}
                  >
                    {previousValueText}
                  </Typography>
                  {stats.trend && stats.trend[item.trendKey] && (
                    <MiniTrendChart
                      data={trendData}
                      dataKey="value"
                      isLeadTime={item.isLeadTime}
                    />
                  )}
                </CardContent>
              </Card>
            </Grid>
          );
        })}
      </Grid>
    </Box>
  );
}

export default StatsGrid;
