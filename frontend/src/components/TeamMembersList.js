import React from 'react';
import Grid from '@mui/joy/Grid'; // Import Grid
import Avatar from '@mui/joy/Avatar';
import Typography from '@mui/joy/Typography';
import CircularProgress from '@mui/joy/CircularProgress';
import Box from '@mui/joy/Box';
import Sheet from '@mui/joy/Sheet'; // Import Sheet for item background
// import Checkbox from '@mui/joy/Checkbox'; // Checkbox removed
import Button from '@mui/joy/Button'; // For Select/Deselect All

function TeamMembersList({
  members,
  loading,
  error,
  selectedTeam,
  fetchAttempted,
  selectedMemberLogins, // New prop
  onToggleMember,       // New prop
  onToggleSelectAllMembers, // New prop
}) {
  // Don't render anything if no team is selected OR if fetch hasn't been attempted yet
  if (!selectedTeam || !fetchAttempted) {
    return null;
  }

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', my: 2 }}>
        <CircularProgress size="sm" />
        <Typography sx={{ ml: 1 }}>Loading members...</Typography>
      </Box>
    );
  }

  if (error) {
    // Error is already displayed globally in App.js, but we could add specific context here if needed
    return null;
    // return <Typography color="danger" sx={{ my: 2 }}>Error loading members: {error}</Typography>;
  }

  // Only show "No members found" if fetch was attempted, not loading, no error, and members is empty
  if (fetchAttempted && !loading && !error && (!members || members.length === 0)) {
    return <Typography sx={{ my: 2 }}>No members found for this team.</Typography>;
  }

  // If members exist, render the grid (even if loading is finished)
  if (members && members.length > 0) {
    const allSelected = members.length > 0 && selectedMemberLogins && members.every(m => selectedMemberLogins.has(m.Member));
    const someSelected = members.length > 0 && selectedMemberLogins && members.some(m => selectedMemberLogins.has(m.Member));

    return (
    <Box sx={{ my: 2 }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1.5 }}>
        <Typography level="title-md">Team Members ({members.length})</Typography>
        {members.length > 0 && onToggleSelectAllMembers && (
          <Button
            size="sm"
            variant="outlined"
            onClick={() => onToggleSelectAllMembers(!allSelected)}
          >
            {allSelected ? 'Deselect All' : (someSelected ? 'Finish Selecting' : 'Select All')}
          </Button>
        )}
      </Box>
      <Grid container spacing={1} sx={{ flexGrow: 1 }}> {/* Reduced spacing for checkboxes */}
        {members.map((member) => (
          <Grid xs={12} sm={6} md={4} lg={3} key={member.Member}> {/* Adjusted grid for better layout with checkbox */}
            <Sheet
              variant={selectedMemberLogins?.has(member.Member) ? "soft" : "outlined"}
              color={selectedMemberLogins?.has(member.Member) ? "primary" : "neutral"}
              sx={{
                p: 1,
                borderRadius: 'sm',
                display: 'flex',
                alignItems: 'center',
                textAlign: 'left',
                height: '100%',
                cursor: 'pointer',
                transition: 'background-color 0.2s, border-color 0.2s', // Smooth transition
                '&:hover': {
                  backgroundColor: selectedMemberLogins?.has(member.Member) ? 'primary.softHoverBg' : 'background.level1',
                }
              }}
              onClick={() => onToggleMember && onToggleMember(member.Member)}
            >
              {/* Checkbox removed */}
              <Avatar
                size="md"
                src={member.AvatarUrl?.String || undefined}
                sx={{ mr: 1.5 }} // Margin right for spacing
              />
              <Typography level="body-sm" sx={{ wordBreak: 'break-word', flexGrow: 1 }}>
                {member.Member}
              </Typography>
            </Sheet>
          </Grid>
        ))}
      </Grid>
    </Box>
    );
  }

  // Fallback if none of the conditions above are met (shouldn't happen in normal flow)
  return null;
}

export default TeamMembersList;
