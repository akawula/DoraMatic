import React, { useState, useEffect, useCallback } from "react";
import axios from "axios";
import { CssVarsProvider } from "@mui/joy/styles";
import CssBaseline from "@mui/joy/CssBaseline";
import Sheet from "@mui/joy/Sheet";
import Typography from "@mui/joy/Typography";

// Import extracted components
import ModeToggle from "./components/ModeToggle";
import TeamSearch from "./components/TeamSearch";
import DateRangePicker from "./components/DateRangePicker";
import StatsGrid from "./components/StatsGrid";
import TeamMembersList from "./components/TeamMembersList";
import PullRequestList from "./components/PullRequestList";
import PaginationControls from "./components/PaginationControls"; // Import PaginationControls

// Helper to get default dates (last 7 days)
const getDefaultStartDate = () => {
  const date = new Date();
  date.setDate(date.getDate() - 7);
  return date.toISOString().split("T")[0]; // YYYY-MM-DD
};

const getDefaultEndDate = () => {
  const date = new Date();
  return date.toISOString().split("T")[0]; // YYYY-MM-DD
};

function App() {
  // State remains in the main App component
  const [searchTerm, setSearchTerm] = useState("");
  const [teamOptions, setTeamOptions] = useState([]);
  const [selectedTeam, setSelectedTeam] = useState(null);
  const [startDate, setStartDate] = useState(getDefaultStartDate());
  const [endDate, setEndDate] = useState(getDefaultEndDate());
  const [stats, setStats] = useState(null);
  const [teamMembers, setTeamMembers] = useState([]); // New state for team members
  const [pullRequests, setPullRequests] = useState([]); // New state for PRs
  const [loadingTeams, setLoadingTeams] = useState(false);
  const [loadingStats, setLoadingStats] = useState(false);
  const [loadingMembers, setLoadingMembers] = useState(false); // New loading state
  const [loadingPRs, setLoadingPRs] = useState(false); // New loading state
  const [error, setError] = useState(null);
  const [membersError, setMembersError] = useState(null); // New error state
  const [prsError, setPrsError] = useState(null); // New error state
  const [fetchAttempted, setFetchAttempted] = useState(false); // Track if fetch button was clicked
  // PR Pagination State
  const [prCurrentPage, setPrCurrentPage] = useState(1);
  const [prPageSize] = useState(20); // Match backend default/logic
  const [prTotalCount, setPrTotalCount] = useState(0);
  const [selectedMemberLogins, setSelectedMemberLogins] = useState(new Set());

  // Data fetching logic remains here
  const fetchTeams = useCallback(async (prefix) => {
    if (!prefix || prefix.length < 1) {
      setTeamOptions([]);
      return;
    }
    setLoadingTeams(true);
    setError(null);
    try {
      const response = await axios.get(`/search/teams`, {
        params: { q: prefix },
      });
      setTeamOptions(response.data || []);
    } catch (err) {
      console.error("Error fetching teams:", err);
      setError("Failed to fetch teams. Is the backend server running?");
      setTeamOptions([]);
    } finally {
      setLoadingTeams(false);
    }
  }, []);

  useEffect(() => {
    const handler = setTimeout(() => {
      fetchTeams(searchTerm);
    }, 500);
    return () => clearTimeout(handler);
  }, [searchTerm, fetchTeams]);

  // Effect to fetch team members and initialize selection (from localStorage or default) when selectedTeam changes
  useEffect(() => {
    if (selectedTeam) {
      setLoadingMembers(true);
      setTeamMembers([]);
      // setSelectedMemberLogins(new Set()); // Don't clear here, load from storage or default after fetch
      setStats(null);
      setPullRequests([]);
      setPrCurrentPage(1);
      setPrTotalCount(0);
      setFetchAttempted(false);
      setError(null);
      setMembersError(null);
      setPrsError(null);

      axios.get(`/teams/${encodeURIComponent(selectedTeam)}/members`)
        .then(response => {
          const fetchedTeamMembers = response.data || [];
          setTeamMembers(fetchedTeamMembers);

          const storageKey = `selectedMembers_${selectedTeam}`;
          const storedSelectionStr = localStorage.getItem(storageKey);
          let initialSelection;

          if (storedSelectionStr) {
            try {
              const storedLoginsArray = JSON.parse(storedSelectionStr);
              if (Array.isArray(storedLoginsArray)) { // Basic validation
                // Further validate: ensure all stored logins are actual members of the fetched team
                const validStoredLogins = storedLoginsArray.filter(login =>
                  fetchedTeamMembers.some(member => member.Member === login)
                );
                initialSelection = new Set(validStoredLogins);
                // If stored selection is empty, it's a valid "all deselected" state
                if (storedLoginsArray.length === 0 && fetchedTeamMembers.length > 0) {
                    initialSelection = new Set();
                } else if (validStoredLogins.length === 0 && storedLoginsArray.length > 0 && fetchedTeamMembers.length > 0) {
                    // Stored logins were present but none are valid members now, default to all selected
                    initialSelection = new Set(fetchedTeamMembers.map(m => m.Member));
                } else if (validStoredLogins.length === 0 && fetchedTeamMembers.length === 0) {
                    initialSelection = new Set(); // No members, empty selection
                }
              } else {
                console.warn("Invalid stored member selection format, defaulting to all.");
                initialSelection = new Set(fetchedTeamMembers.map(m => m.Member));
              }
            } catch (e) {
              console.error("Failed to parse stored member selection, defaulting to all:", e);
              initialSelection = new Set(fetchedTeamMembers.map(m => m.Member));
            }
          } else {
            // No stored selection, default to all members
            initialSelection = new Set(fetchedTeamMembers.map(m => m.Member));
          }
          setSelectedMemberLogins(initialSelection);
        })
        .catch(err => {
          console.error("Error fetching team members:", err);
          setMembersError(`Failed to fetch members for team "${selectedTeam}".`);
          setTeamMembers([]); // Ensure members are empty on error
          setSelectedMemberLogins(new Set()); // Ensure selection is empty on error
        })
        .finally(() => {
          setLoadingMembers(false);
        });
    } else {
      // Clear all data if no team is selected
      setTeamMembers([]);
      setSelectedMemberLogins(new Set());
      setStats(null);
      setPullRequests([]);
      setPrTotalCount(0);
      setFetchAttempted(false);
    }
  }, [selectedTeam]);

  // Effect to save selectedMemberLogins to localStorage
  useEffect(() => {
    if (selectedTeam && teamMembers.length > 0) { // Only save if team and members are loaded
      // This ensures we don't save an empty set just because members haven't loaded yet.
      // We save even if selectedMemberLogins is empty (all deselected).
      const storageKey = `selectedMembers_${selectedTeam}`;
      try {
        localStorage.setItem(storageKey, JSON.stringify(Array.from(selectedMemberLogins)));
      } catch (e) {
        console.error("Failed to save member selection to local storage:", e);
      }
    }
  }, [selectedMemberLogins, selectedTeam, teamMembers]); // Depends on these to save correctly

  // Renamed handleFetchData to handleFetchStatsAndPRs
  // This function will not fetch members anymore.
  const handleFetchStatsAndPRs = useCallback(async (page = 1, currentSelectedLogins) => {
    if (!selectedTeam || !startDate || !endDate || teamMembers.length === 0 && currentSelectedLogins.size === 0) {
      // Do not fetch if no team, no dates, or if members haven't loaded yet (unless a specific selection is already made, which is unlikely here)
      // If teamMembers is empty AND currentSelectedLogins is empty, it implies members are still loading or failed.
      // If currentSelectedLogins has items, it means user interacted, so proceed.
      // If teamMembers has items and currentSelectedLogins is empty, it means user deselected all.
      if (!selectedTeam || !startDate || !endDate) {
         setError("Please select a team and specify a valid date range.");
      }
      // If teamMembers.length === 0 and currentSelectedLogins.size === 0, it might be mid-load of members.
      // The useEffect below will manage this.
      return;
    }

    setFetchAttempted(true);
    setPrCurrentPage(page);

    setLoadingStats(true);
    setLoadingPRs(true);
    // Don't clear stats/PRs here if it's just a page change for PRs.
    // Clearing should happen if selectedTeam, dates, or selectedMemberLogins change.
    // The useEffect below handles clearing for major changes.
    if (page === 1) { // Clear only if it's a "reset" to page 1 due to filter changes
        setStats(null);
        setPullRequests([]);
    }
    setError(null); // Clear general error
    setPrsError(null); // Clear PR specific error


    const rfcStartDate = `${startDate}T00:00:00Z`;
    const rfcEndDate = `${endDate}T23:59:59Z`;
    const selectedLoginsArray = Array.from(currentSelectedLogins);
    const membersQueryParam = selectedLoginsArray.length > 0 ? selectedLoginsArray.join(',') : undefined;

    const promises = [];

    promises.push(
      axios.get(`/teams/${encodeURIComponent(selectedTeam)}/stats`, {
        params: { start_date: rfcStartDate, end_date: rfcEndDate, members: membersQueryParam },
      })
      .then(response => setStats(response.data))
      .catch(err => {
        console.error("Error fetching stats:", err);
        setError(`Failed to fetch stats for team "${selectedTeam}".`);
        setStats(null);
      })
      .finally(() => setLoadingStats(false))
    );

    promises.push(
      axios.get(`/prs`, {
        params: {
          start_date: rfcStartDate,
          end_date: rfcEndDate,
          team: selectedTeam,
          page: page,
          page_size: prPageSize,
          members: membersQueryParam,
        },
      })
      .then(response => {
        setPullRequests(response.data?.pull_requests || []);
        setPrTotalCount(response.data?.total_count || 0);
      })
      .catch(err => {
        console.error("Error fetching pull requests:", err);
        setPrsError(`Failed to fetch pull requests.`);
        setPullRequests([]);
        setPrTotalCount(0);
      })
      .finally(() => setLoadingPRs(false))
    );

    await Promise.all(promises);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedTeam, startDate, endDate, prPageSize, teamMembers]); // teamMembers added as a dependency to re-evaluate the initial condition

  // useEffect to fetch stats and PRs when relevant filters change
  useEffect(() => {
    if (selectedTeam && startDate && endDate && teamMembers.length > 0) {
      // This condition ensures members are loaded before attempting to fetch stats/PRs
      // selectedMemberLogins is now stable after the first member load.
      // If selectedMemberLogins is empty, it means user deselected all (or it's the brief moment before initial auto-select).
      // The handleFetchStatsAndPRs will get the latest selectedMemberLogins.
      handleFetchStatsAndPRs(prCurrentPage, selectedMemberLogins);
    } else if (!selectedTeam) {
        // Clear data if no team is selected (already handled by the other useEffect, but good for safety)
        setStats(null);
        setPullRequests([]);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedTeam, startDate, endDate, selectedMemberLogins, prCurrentPage]); // Removed teamMembers from here, it's handled by selectedTeam effect setting selectedMemberLogins


  const handleToggleMember = useCallback((login) => {
    setSelectedMemberLogins(prevLogins => {
      const newLogins = new Set(prevLogins);
      if (newLogins.has(login)) {
        newLogins.delete(login);
      } else {
        newLogins.add(login);
      }
      return newLogins;
    });
  }, []);

  const handleToggleSelectAllMembers = useCallback((selectAll) => {
    if (selectAll) {
      setSelectedMemberLogins(new Set(teamMembers.map(m => m.Member)));
    } else {
      setSelectedMemberLogins(new Set());
    }
  }, [teamMembers]);


  // Handler for pagination change
  const handlePageChange = (event, newPage) => {
    // When page changes, we need to pass the current selectedMemberLogins
    if (selectedTeam && startDate && endDate) {
      handleFetchStatsAndPRs(newPage, selectedMemberLogins);
    }
  };

  // Render the components, passing state and handlers as props
  return (
    <CssVarsProvider>
      <CssBaseline />
      <ModeToggle />
      <Sheet sx={{ p: 4, mt: 4 }}>
        {" "}
        {/* Main container */}
        <Typography level="h3" component="h1" gutterBottom>
          Team Performance Metrics
        </Typography>
        <TeamSearch
          searchTerm={searchTerm}
          setSearchTerm={setSearchTerm}
          teamOptions={teamOptions}
          loadingTeams={loadingTeams}
          selectedTeam={selectedTeam}
          setSelectedTeam={setSelectedTeam}
        />
        <DateRangePicker
          startDate={startDate}
          setStartDate={setStartDate}
          endDate={endDate}
          setEndDate={setEndDate}
          // onFetchStats, loadingStats, isFetchDisabled are removed as fetch is automatic
        />
        {/* Combined loading indicator, shown when any primary data is loading on initial fetch or team/date change */}
        {(loadingMembers || loadingStats || loadingPRs) && fetchAttempted && prCurrentPage === 1 && (
          <Typography sx={{ my: 2 }}>Loading data...</Typography>
        )}
        {(error || membersError || prsError) && ( // Check if any error exists
          <Typography color="danger" sx={{ mb: 2 }}>
            Error: {error || membersError || prsError}{" "}
            {/* Show the first non-null error */}
          </Typography>
        )}
        {/* Render Team Members List */}
        <TeamMembersList
          members={teamMembers}
          loading={loadingMembers}
          error={membersError}
          selectedTeam={selectedTeam}
          fetchAttempted={fetchAttempted}
          selectedMemberLogins={selectedMemberLogins}
          onToggleMember={handleToggleMember}
          onToggleSelectAllMembers={handleToggleSelectAllMembers}
        />
        {/* Render Stats Grid */}
        <StatsGrid
          stats={stats}
          loadingStats={loadingStats} // Keep separate loading for grid
          selectedTeam={selectedTeam}
          startDate={startDate}
          endDate={endDate}
        />
        {/* Render Pull Request List */}
        <PullRequestList
          pullRequests={pullRequests}
          loading={loadingPRs}
          error={prsError}
          selectedTeam={selectedTeam}
          startDate={startDate}
          endDate={endDate}
          fetchAttempted={fetchAttempted}
        />
        {/* Render Pagination Controls if fetch attempted and PRs exist */}
        {fetchAttempted && prTotalCount > 0 && (
          <PaginationControls
            currentPage={prCurrentPage}
            pageSize={prPageSize}
            totalCount={prTotalCount}
            onPageChange={handlePageChange}
            loading={loadingPRs} // Disable controls while loading PRs
          />
        )}
      </Sheet>
    </CssVarsProvider>
  );
}

export default App;
