import React from 'react';
import Box from '@mui/joy/Box';
import Autocomplete from '@mui/joy/Autocomplete';
import Typography from '@mui/joy/Typography';

function TeamSearch({
  searchTerm,
  setSearchTerm,
  teamOptions,
  loadingTeams,
  selectedTeam,
  setSelectedTeam,
}) {
  return (
    <Box sx={{ display: 'flex', gap: 2, mb: 3, alignItems: 'flex-end' }}>
      <Box sx={{ flexGrow: 1 }}>
        <Typography level="title-sm" sx={{ mb: 0.5 }}>Search Team</Typography>
        <Autocomplete
          placeholder="Type to search..."
          options={teamOptions}
          loading={loadingTeams}
          inputValue={searchTerm}
          onInputChange={(event, newInputValue) => {
            setSearchTerm(newInputValue);
          }}
          value={selectedTeam}
          onChange={(event, newValue) => {
            setSelectedTeam(newValue);
            // Optionally clear stats when team changes
            // setStats(null); // This logic should stay in App.js or be passed via prop if needed
          }}
          getOptionLabel={(option) => option || ""} // Handle potential null/undefined options
          isOptionEqualToValue={(option, value) => option === value}
          noOptionsText={searchTerm.length > 0 ? "No teams found" : "Type to search"}
          sx={{ width: '100%' }}
        />
      </Box>
    </Box>
  );
}

export default TeamSearch;
