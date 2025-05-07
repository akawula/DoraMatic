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
import { LineChart, Line, Tooltip, ResponsiveContainer, YAxis } from 'recharts';

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

const MiniTrendChart = ({ data, dataKey, isLeadTime }) => {
  if (!data || data.length < 2) {
    return (
      <Box sx={{ height: 60, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Typography level="body-xs" color="neutral">Not enough data for trend</Typography>
      </Box>
    );
  }

  const chartData = data.map((value, index) => ({ name: `P${index}`, value }));

  // Determine a contrasting color for the line based on Joy UI theme
  // This is a simplified example; you might need a more robust way to access theme colors
  const lineColor = "#1976d2"; // Example: Joy UI primary color

  return (
    <Box sx={{ height: 60, width: '100%', mt: 1 }}>
      <ResponsiveContainer>
        <LineChart data={chartData} margin={{ top: 5, right: 5, left: 5, bottom: 5 }}>
          <Tooltip
            contentStyle={{ backgroundColor: 'rgba(255, 255, 255, 0.8)', borderRadius: '4px', fontSize: '12px', padding: '4px 8px' }}
            formatter={(value) => isLeadTime ? formatLeadTimeDisplay(value) : value.toLocaleString()}
          />
          <YAxis hide domain={['dataMin - dataMin * 0.1', 'dataMax + dataMax * 0.1']} />
          <Line type="monotone" dataKey="value" stroke={lineColor} strokeWidth={2} dot={false} />
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
  const calculateChange = (current, previous) => {
    if (previous === 0) {
      if (current > 0) return { percentage: 100, icon: <ArrowUpwardIcon />, color: "success.main" };
      return { percentage: 0, icon: <RemoveIcon />, color: "text.secondary" }; // No change from zero
    }
    if (current === null || previous === null) {
       return { percentage: null, icon: null, color: "text.secondary" };
    }

    const change = ((current - previous) / previous) * 100;
    const percentage = Math.abs(change).toFixed(1); // Keep one decimal place

    if (change > 0) {
      return { percentage, icon: <ArrowUpwardIcon />, color: "success.main" };
    } else if (change < 0) {
      return { percentage, icon: <ArrowDownwardIcon />, color: "danger.main" };
    } else {
      return { percentage: 0, icon: <RemoveIcon />, color: "text.secondary" }; // No change
    }
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
      label: "Closed PRs (Not Merged)",
      current: stats.current?.closed_prs_count,
      previous: stats.previous?.closed_prs_count,
      trendKey: "closed_prs_count",
      isLeadTime: false,
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
  ];

  return (
    <Box sx={{ mt: 3 }}>
      <Typography level="body-xs" sx={{ mb: 2, textAlign: 'center', fontStyle: 'italic' }}>
        Note: Trend graphs show data for the last 6 periods, including the current one. Backend needs to provide this data via `stats.trend`.
      </Typography>
      <Grid container spacing={2} sx={{ flexGrow: 1 }}>
        {statsItems.map((item, index) => {
          const changeInfo = calculateChange(item.current, item.previous);
          const trendData = stats.trend && stats.trend[item.trendKey] ? stats.trend[item.trendKey] : [];

          let currentValueDisplay = "N/A";
          let previousValueText = "";

          if (item.isLeadTime) {
            currentValueDisplay = formatLeadTimeDisplay(item.current);
            previousValueText = item.previous !== undefined && item.previous !== null
              ? `(Prev: ${formatLeadTimeDisplay(item.previous)})`
              : "";
          } else if (item.isPercentage) {
            currentValueDisplay = typeof item.current === 'number' ? `${item.current.toFixed(1)}%` : "N/A";
            previousValueText = typeof item.previous === 'number' ? `(Prev: ${item.previous.toFixed(1)}%)` : "";
          } else if (item.label === "Avg PR Size (Lines)") {
            const currentSize = typeof item.current === 'number' ? item.current.toFixed(1) : "N/A";
            const currentLabel = typeof item.current === 'number' ? getPrSizeLabel(item.current) : "";
            currentValueDisplay = `${currentSize}${currentLabel ? ` (${currentLabel})` : ""}`;

            const prevSize = typeof item.previous === 'number' ? item.previous.toFixed(1) : "";
            const prevLabel = typeof item.previous === 'number' ? getPrSizeLabel(item.previous) : "";
            previousValueText = prevSize ? `(Prev: ${prevSize}${prevLabel ? ` (${prevLabel})` : ""})` : "";
          }
          else {
            currentValueDisplay = typeof item.current === 'number' ? item.current.toLocaleString(undefined, {maximumFractionDigits: 1}) : "N/A";
            previousValueText = typeof item.previous === 'number' ? `(Prev: ${item.previous.toLocaleString(undefined, {maximumFractionDigits: 1})})` : "";
          }

          return (
            <Grid xs={12} sm={6} md={4} lg={3} key={index}> {/* Adjusted grid for potentially more items or wider cards */}
              <Card variant="outlined" sx={{ height: "100%", display: 'flex', flexDirection: 'column' }}>
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
                        color: changeInfo.color,
                        mt: 0.5,
                      }}
                    >
                      {React.cloneElement(changeInfo.icon, { fontSize: "small" })}
                      <Typography level="body-sm" sx={{ ml: 0.5 }}>
                        {changeInfo.percentage}%
                      </Typography>
                    </Box>
                  )}
                  <Typography level="body-xs" sx={{ color: 'text.tertiary', mt: 0.5 }}>
                    {previousValueText}
                  </Typography>
                  {stats.trend && stats.trend[item.trendKey] && (
                     <MiniTrendChart data={trendData} dataKey="value" isLeadTime={item.isLeadTime} />
                  )}
                  {(!stats.trend || !stats.trend[item.trendKey]) && (
                    <Box sx={{ height: 60, display: 'flex', alignItems: 'center', justifyContent: 'center', mt: 1 }}>
                      <Typography level="body-xs" color="neutral">Trend data not available</Typography>
                    </Box>
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
