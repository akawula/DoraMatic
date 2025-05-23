import React, { useState, useEffect, useCallback } from "react";
import axios from "axios";
import Button from "@mui/joy/Button"; // Import Button for Logout
import { CssVarsProvider } from "@mui/joy/styles";
import CssBaseline from "@mui/joy/CssBaseline";
import Sheet from "@mui/joy/Sheet";
import Typography from "@mui/joy/Typography";
import Box from "@mui/joy/Box"; // For layout

// Import extracted components
import ModeToggle from "./components/ModeToggle";
import Login from "./components/Login"; // Import Login component
import authService from "./services/authService"; // Import authService
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

  // Authentication State
  const [isAuthenticated, setIsAuthenticated] = useState(authService.isAuthenticated());
  const [currentUsername, setCurrentUsername] = useState(authService.getCurrentUsername());

  const handleLoginSuccess = () => {
    setIsAuthenticated(true);
    setCurrentUsername(authService.getCurrentUsername()); // Set username from service
    // Reset states that might hold data from a previous unauthenticated view or another user
    setSearchTerm("");
    setTeamOptions([]);
    setSelectedTeam(null);
    setStats(null);
    setTeamMembers([]);
    setPullRequests([]);
    setPrCurrentPage(1);
    setPrTotalCount(0);
    setSelectedMemberLogins(new Set());
    setError(null);
    setMembersError(null);
    setPrsError(null);
    setFetchAttempted(false);
  };

  const handleLogout = () => {
    authService.logout();
    setIsAuthenticated(false);
    setCurrentUsername(null); // Clear username
    // Clear all data that might be user-specific or session-specific
    setSearchTerm("");
    setTeamOptions([]);
    setSelectedTeam(null);
    setStats(null);
    setTeamMembers([]);
    setPullRequests([]);
    setPrCurrentPage(1);
    setPrTotalCount(0);
    setSelectedMemberLogins(new Set());
    setError(null);
    setMembersError(null);
    setPrsError(null);
    setFetchAttempted(false);
  };

  // Data fetching logic remains here
  const fetchTeams = useCallback(async (prefix) => {
    if (!prefix || prefix.length < 1) {
      setTeamOptions([]);
      return;
    }
    setLoadingTeams(true);
    setError(null);
    try {
      const response = await axios.get(`${process.env.REACT_APP_API_BASE_URL || ''}/api/search/teams`, { // Added /api prefix
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
      setStats(null);
      setPullRequests([]);
      setPrCurrentPage(1); // Reset to page 1 when team changes
      setPrTotalCount(0);
      setFetchAttempted(false); // Reset fetch attempt for new team
      setError(null);
      setMembersError(null);
      setPrsError(null);

      axios
        .get(`${process.env.REACT_APP_API_BASE_URL || ''}/api/teams/${encodeURIComponent(selectedTeam)}/members`) // Added /api prefix
        .then((response) => {
          const fetchedTeamMembers = response.data || [];
          setTeamMembers(fetchedTeamMembers);

          const storageKey = `selectedMembers_${selectedTeam}`;
          const storedSelectionStr = localStorage.getItem(storageKey);
          let initialSelection;

          if (storedSelectionStr) {
            try {
              const storedLoginsArray = JSON.parse(storedSelectionStr);
              if (Array.isArray(storedLoginsArray)) {
                const validStoredLogins = storedLoginsArray.filter((login) =>
                  fetchedTeamMembers.some((member) => member.Member === login)
                );
                initialSelection = new Set(validStoredLogins);
                if (
                  storedLoginsArray.length === 0 &&
                  fetchedTeamMembers.length > 0
                ) {
                  initialSelection = new Set();
                } else if (
                  validStoredLogins.length === 0 &&
                  storedLoginsArray.length > 0 &&
                  fetchedTeamMembers.length > 0
                ) {
                  initialSelection = new Set(
                    fetchedTeamMembers.map((m) => m.Member)
                  );
                } else if (
                  validStoredLogins.length === 0 &&
                  fetchedTeamMembers.length === 0
                ) {
                  initialSelection = new Set();
                }
              } else {
                console.warn(
                  "Invalid stored member selection format, defaulting to all."
                );
                initialSelection = new Set(
                  fetchedTeamMembers.map((m) => m.Member)
                );
              }
            } catch (e) {
              console.error(
                "Failed to parse stored member selection, defaulting to all:",
                e
              );
              initialSelection = new Set(
                fetchedTeamMembers.map((m) => m.Member)
              );
            }
          } else {
            initialSelection = new Set(fetchedTeamMembers.map((m) => m.Member));
          }
          setSelectedMemberLogins(initialSelection);
        })
        .catch((err) => {
          console.error("Error fetching team members:", err);
          setMembersError(
            `Failed to fetch members for team "${selectedTeam}".`
          );
          setTeamMembers([]);
          setSelectedMemberLogins(new Set());
        })
        .finally(() => {
          setLoadingMembers(false);
        });
    } else {
      setTeamMembers([]);
      setSelectedMemberLogins(new Set());
      setStats(null);
      setPullRequests([]);
      setPrTotalCount(0);
      setPrCurrentPage(1);
      setFetchAttempted(false);
    }
  }, [selectedTeam]);

  // Effect to save selectedMemberLogins to localStorage
  useEffect(() => {
    if (selectedTeam && teamMembers.length > 0) {
      const storageKey = `selectedMembers_${selectedTeam}`;
      try {
        localStorage.setItem(
          storageKey,
          JSON.stringify(Array.from(selectedMemberLogins))
        );
      } catch (e) {
        console.error("Failed to save member selection to local storage:", e);
      }
    }
  }, [selectedMemberLogins, selectedTeam, teamMembers]);

  const handleFetchStatsAndPRs = useCallback(
    async (page, currentSelectedLogins, fetchStats = true) => {
      if (!selectedTeam || !startDate || !endDate) {
        return;
      }
      if (teamMembers.length === 0 && currentSelectedLogins.size === 0 && !fetchAttempted) {
        return;
      }

      setFetchAttempted(true);

      setLoadingPRs(true);
      setPrsError(null);
      if (page === 1) {
        setPullRequests([]);
      }

      if (fetchStats) {
        setLoadingStats(true);
        setError(null);
        if (page === 1) {
          setStats(null);
        }
      }

      const rfcStartDate = `${startDate}T00:00:00Z`;
      const rfcEndDate = `${endDate}T23:59:59Z`;
      const selectedLoginsArray = Array.from(currentSelectedLogins);
      const membersQueryParam =
        selectedLoginsArray.length > 0
          ? selectedLoginsArray.join(",")
          : undefined;

      const promises = [];
      const token = authService.getToken();
      const headers = {};
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      }

      if (fetchStats) {
        // Assuming /stats endpoint might also be protected or will be in the future
        promises.push(
          axios
            .get(`${process.env.REACT_APP_API_BASE_URL || ''}/api/teams/${encodeURIComponent(selectedTeam)}/stats`, { // Added /api prefix
              params: {
                start_date: rfcStartDate,
                end_date: rfcEndDate,
                members: membersQueryParam,
              },
              headers: headers, // Add headers here too if stats becomes protected
            })
            .then((response) => setStats(response.data))
            .catch((err) => {
              console.error("Error fetching stats:", err);
              setError(`Failed to fetch stats for team "${selectedTeam}".`);
              setStats(null);
              if (err.response && err.response.status === 401) {
                handleLogout(); // Logout if unauthorized
              }
            })
            .finally(() => setLoadingStats(false))
        );
      }

      promises.push(
        axios
          .get(`/api/prs`, { // Updated endpoint to /api/prs (protected)
            params: {
              start_date: rfcStartDate,
              end_date: rfcEndDate,
              team: selectedTeam,
              members: membersQueryParam,
              page: page,
              page_size: prPageSize,
            },
            headers: headers, // Send token for protected route
          })
          .then((response) => {
            setPullRequests(response.data?.pull_requests || []);
            setPrTotalCount(response.data?.total_count || 0);
          })
          .catch((err) => {
            console.error("Error fetching pull requests:", err);
            setPrsError(`Failed to fetch pull requests.`);
            setPullRequests([]);
            setPrTotalCount(0);
            if (err.response && err.response.status === 401) {
              handleLogout(); // Logout if unauthorized
            }
          })
          .finally(() => setLoadingPRs(false))
      );

      await Promise.all(promises);
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [selectedTeam, startDate, endDate, prPageSize, teamMembers, fetchAttempted]
  );

  // useEffect to fetch stats and PRs (page 1) when relevant filters change (team, dates, members)
  useEffect(() => {
    if (selectedTeam && startDate && endDate && teamMembers.length > 0) {
      if (prCurrentPage !== 1) {
        setPrCurrentPage(1);
      }
      handleFetchStatsAndPRs(1, selectedMemberLogins, true);
    } else if (!selectedTeam) {
      setStats(null);
      setPullRequests([]);
      setPrTotalCount(0);
      if (prCurrentPage !== 1) {
        setPrCurrentPage(1);
      }
      setFetchAttempted(false);
      setError(null);
      setMembersError(null);
      setPrsError(null);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedTeam, startDate, endDate, selectedMemberLogins, teamMembers]);

  // useEffect for PR pagination (when prCurrentPage changes)
  useEffect(() => {
    // This effect handles pagination once an initial fetch has been attempted.
    // It runs for any change in prCurrentPage.
    if (selectedTeam && startDate && endDate && teamMembers.length > 0 && fetchAttempted) {
      // Determine if stats should be fetched: only for page 1.
      // Also, if navigating to page 1, PRs should be cleared before fetching.
      const shouldFetchStats = prCurrentPage === 1;
      handleFetchStatsAndPRs(prCurrentPage, selectedMemberLogins, shouldFetchStats);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [prCurrentPage, fetchAttempted]); // Dependencies: prCurrentPage and fetchAttempted gate.
  // selectedMemberLogins, selectedTeam, etc., are available to handleFetchStatsAndPRs via its useCallback closure.


  const handleToggleMember = useCallback((login) => {
    setSelectedMemberLogins((prevLogins) => {
      const newLogins = new Set(prevLogins);
      if (newLogins.has(login)) {
        newLogins.delete(login);
      } else {
        newLogins.add(login);
      }
      return newLogins;
    });
  }, []);

  const handleToggleSelectAllMembers = useCallback(
    (selectAll) => {
      if (selectAll) {
        setSelectedMemberLogins(new Set(teamMembers.map((m) => m.Member)));
      } else {
        setSelectedMemberLogins(new Set());
      }
    },
    [teamMembers]
  );

  // Handler for pagination change
  const handlePageChange = (event, newPage) => {
    if (selectedTeam && startDate && endDate) {
      setPrCurrentPage(newPage);
    }
  };

  // Render the components, passing state and handlers as props
  if (!isAuthenticated) {
    return (
      <CssVarsProvider>
        <CssBaseline />
        <Login onLoginSuccess={handleLoginSuccess} />
      </CssVarsProvider>
    );
  }

  return (
    <CssVarsProvider>
      <CssBaseline />
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', p: 2 }}>
        <ModeToggle />
        <Box sx={{ display: 'flex', alignItems: 'center' }}>
          {currentUsername && <Typography sx={{ mr: 2 }}>Logged in as: {currentUsername}</Typography>}
          <Button onClick={handleLogout} color="danger" variant="soft">Logout</Button>
        </Box>
      </Box>
      <Sheet sx={{ p: 4, mt: 0 }}> {/* Adjusted mt from 4 to 0 as ModeToggle/Logout are now above */}
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
        />
        {(loadingMembers || (loadingStats && prCurrentPage === 1)) && fetchAttempted && (
          <Typography sx={{ my: 2 }}>Loading data...</Typography>
        )}
        {(error || membersError || prsError) && (
          <Typography color="danger" sx={{ mb: 2 }}>
            Error: {error || membersError || prsError}{" "}
          </Typography>
        )}
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
        <StatsGrid
          stats={stats}
          loadingStats={loadingStats}
          selectedTeam={selectedTeam}
          startDate={startDate}
          endDate={endDate}
        />
        <PullRequestList
          pullRequests={pullRequests}
          loading={loadingPRs}
          error={prsError}
          selectedTeam={selectedTeam}
          startDate={startDate}
          endDate={endDate}
          fetchAttempted={fetchAttempted}
        />
        {fetchAttempted && prTotalCount > 0 && (
          <PaginationControls
            currentPage={prCurrentPage}
            pageSize={prPageSize}
            totalCount={prTotalCount}
            onPageChange={handlePageChange}
            loading={loadingPRs}
          />
        )}
      </Sheet>
    </CssVarsProvider>
  );
}

export default App;
