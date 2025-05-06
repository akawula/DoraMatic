import React from 'react';
import Box from '@mui/joy/Box';
import Button from '@mui/joy/Button';
import Typography from '@mui/joy/Typography';

function PaginationControls({
  currentPage,
  pageSize,
  totalCount,
  onPageChange,
  loading, // To disable buttons while loading
}) {
  if (totalCount <= pageSize) {
    // Don't show pagination if there's only one page
    return null;
  }

  const totalPages = Math.ceil(totalCount / pageSize);

  const handlePrevious = () => {
    if (currentPage > 1) {
      onPageChange(null, currentPage - 1); // Pass null event, new page number
    }
  };

  const handleNext = () => {
    if (currentPage < totalPages) {
      onPageChange(null, currentPage + 1); // Pass null event, new page number
    }
  };

  return (
    <Box
      sx={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        mt: 2, // Margin top
        mb: 1, // Margin bottom
      }}
    >
      <Button
        size="sm"
        variant="outlined"
        onClick={handlePrevious}
        disabled={currentPage === 1 || loading}
      >
        Previous
      </Button>
      <Typography level="body-sm">
        Page {currentPage} of {totalPages} ({totalCount} items)
      </Typography>
      <Button
        size="sm"
        variant="outlined"
        onClick={handleNext}
        disabled={currentPage === totalPages || loading}
      >
        Next
      </Button>
    </Box>
  );
}

export default PaginationControls;
