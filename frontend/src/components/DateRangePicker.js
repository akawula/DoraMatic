import React from 'react';
import Box from '@mui/joy/Box';
import Input from '@mui/joy/Input';
import Button from '@mui/joy/Button';
import IconButton from '@mui/joy/IconButton';
import Typography from '@mui/joy/Typography';
import ArrowBackIcon from '@mui/icons-material/ArrowBack';
import ArrowForwardIcon from '@mui/icons-material/ArrowForward';

function DateRangePicker({
  startDate,
  setStartDate,
  endDate,
  setEndDate,
  // onFetchStats, // No longer needed, fetch is automatic
  // loadingStats, // No longer needed, handled in App.js
  // isFetchDisabled, // No longer needed, handled in App.js
}) {
  const handlePreviousPeriod = () => {
    const start = new Date(startDate);
    const end = new Date(endDate);
    const diffTime = Math.abs(end - start);

    const newEndDate = new Date(start.getTime() - (24 * 60 * 60 * 1000)); // Subtract 1 day from current start to get new end
    const newStartDate = new Date(newEndDate.getTime() - diffTime);

    setStartDate(newStartDate.toISOString().split('T')[0]);
    setEndDate(newEndDate.toISOString().split('T')[0]);
  };

  const handleNextPeriod = () => {
    const start = new Date(startDate);
    const end = new Date(endDate);
    const diffTime = Math.abs(end - start);
    const today = new Date();
    today.setHours(0, 0, 0, 0); // Normalize today to the start of the day

    // If current end date is already today or in the future, do nothing.
    if (end >= today) {
      return;
    }

    let newStartDate = new Date(end.getTime() + (24 * 60 * 60 * 1000)); // Add 1 day to current end
    let newEndDate = new Date(newStartDate.getTime() + diffTime);

    // If the calculated newEndDate goes beyond today, cap it at today
    // and adjust newStartDate to maintain the duration.
    if (newEndDate > today) {
      newEndDate = today;
      newStartDate = new Date(today.getTime() - diffTime);
      // Ensure newStartDate does not go before the original start date if today was the initial end.
      // This can happen if the period is very short.
      // A simpler rule: if newEndDate is capped, newStartDate is today - diff.
      // This might shorten the last interval if it hits 'today'.
    }

    setStartDate(newStartDate.toISOString().split('T')[0]);
    setEndDate(newEndDate.toISOString().split('T')[0]);
  };

  const todayString = new Date().toISOString().split('T')[0];
  const isNextDisabled = new Date(endDate) >= new Date(todayString);

  const handleSetLastWeek = () => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    const dayOfWeek = today.getDay(); // 0 for Sunday, 1 for Monday, ..., 6 for Saturday

    const endLastWeek = new Date(today);
    // Subtract days to get to the most recent Sunday, then subtract 7 more if today is Sunday to get previous week's Sunday
    // Or more simply: go to previous day until it's a Sunday.
    // If today is Sunday, dayOfWeek is 0. We want to subtract 7 days to get to *last* Sunday.
    // If today is Monday, dayOfWeek is 1. We want to subtract 1 day.
    // So, daysToSubtractForSunday = dayOfWeek === 0 ? 7 : dayOfWeek;
    endLastWeek.setDate(today.getDate() - (dayOfWeek === 0 ? 7 : dayOfWeek));


    const startLastWeek = new Date(endLastWeek);
    startLastWeek.setDate(endLastWeek.getDate() - 6); // Monday of that week

    // Ensure the calculated dates are not in the future (shouldn't happen with this logic for "last week")
    // and also respect the overall maxDate (todayString) for the endDate input.
    // The logic for "last week" inherently produces past dates.

    setStartDate(startLastWeek.toISOString().split('T')[0]);
    setEndDate(endLastWeek.toISOString().split('T')[0]);
  };

  const handleSetLastTwoWeeks = () => {
    const today = new Date();
    today.setHours(0, 0, 0, 0); // Normalize to start of day

    const endDateTwoWeeks = new Date(today); // End date is today

    const startDateTwoWeeks = new Date(today);
    startDateTwoWeeks.setDate(today.getDate() - 13); // 14 days inclusive of today

    setStartDate(startDateTwoWeeks.toISOString().split('T')[0]);
    setEndDate(endDateTwoWeeks.toISOString().split('T')[0]);
  };

  const handleSetLastMonth = () => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    const firstDayCurrentMonth = new Date(today.getFullYear(), today.getMonth(), 1);
    const endDateLastMonth = new Date(firstDayCurrentMonth);
    endDateLastMonth.setDate(firstDayCurrentMonth.getDate() - 1);

    const startDateLastMonth = new Date(endDateLastMonth.getFullYear(), endDateLastMonth.getMonth(), 1);

    setStartDate(startDateLastMonth.toISOString().split('T')[0]);
    setEndDate(endDateLastMonth.toISOString().split('T')[0]);
  };

  const handleSetLastQuarter = () => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    const currentMonth = today.getMonth();
    const currentYear = today.getFullYear();

    const firstMonthOfCurrentQuarter = Math.floor(currentMonth / 3) * 3;
    const firstDayOfCurrentQuarter = new Date(currentYear, firstMonthOfCurrentQuarter, 1);

    const endDateLastQuarter = new Date(firstDayOfCurrentQuarter);
    endDateLastQuarter.setDate(firstDayOfCurrentQuarter.getDate() - 1);

    // The year for the start of the last quarter is the same as the end date's year.
    // The first month of the last quarter is two months prior to the last month of the last quarter.
    const startDateLastQuarter = new Date(endDateLastQuarter.getFullYear(), endDateLastQuarter.getMonth() - 2, 1);

    setStartDate(startDateLastQuarter.toISOString().split('T')[0]);
    setEndDate(endDateLastQuarter.toISOString().split('T')[0]);
  };

  return (
    <Box sx={{ display: 'flex', gap: 2, mb: 3, alignItems: 'flex-end', flexWrap: 'wrap' }}>
      <IconButton onClick={handlePreviousPeriod} aria-label="Previous period" size="lg">
        <ArrowBackIcon />
      </IconButton>
      <Box>
        <Typography level="title-sm" sx={{ mb: 0.5 }}>Start Date</Typography>
        <Input
          type="date"
          value={startDate}
          onChange={(e) => setStartDate(e.target.value)}
          slotProps={{ input: { max: endDate } }} // Prevent start date > end date
        />
      </Box>
      <Box>
        <Typography level="title-sm" sx={{ mb: 0.5 }}>End Date</Typography>
        <Input
          type="date"
          value={endDate}
          onChange={(e) => setEndDate(e.target.value)}
          slotProps={{ input: { min: startDate, max: todayString } }} // Prevent end date < start date and > today
        />
      </Box>
      <IconButton onClick={handleNextPeriod} aria-label="Next period" size="lg" disabled={isNextDisabled}>
        <ArrowForwardIcon />
      </IconButton>
      <Button variant="outlined" onClick={handleSetLastWeek} sx={{ ml: 1 }}> {/* Added margin for spacing */}
        Last Week
      </Button>
      <Button variant="outlined" onClick={handleSetLastTwoWeeks} sx={{ ml: 1 }}>
        Last 2 Weeks
      </Button>
      <Button variant="outlined" onClick={handleSetLastMonth} sx={{ ml: 1 }}>
        Last Month
      </Button>
      <Button variant="outlined" onClick={handleSetLastQuarter} sx={{ ml: 1 }}>
        Last Quarter
      </Button>
      {/* Button removed, fetch is automatic */}
    </Box>
  );
}

export default DateRangePicker;
