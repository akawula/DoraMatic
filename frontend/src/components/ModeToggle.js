import React, { useState, useEffect } from 'react';
import { useColorScheme } from '@mui/joy/styles';
import Button from '@mui/joy/Button';

function ModeToggle() {
  const { mode, setMode } = useColorScheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) {
    // Avoid server-client mismatch
    // Also apply zIndex here to maintain consistency if it ever becomes visible during hydration
    return <Button variant="outlined" sx={{ position: 'absolute', top: 16, right: 16, visibility: 'hidden', zIndex: 1300 }} />;
  }

  return (
    <Button
      variant="outlined"
      onClick={() => {
        setMode(mode === 'light' ? 'dark' : 'light');
      }}
      sx={{ position: 'absolute', top: 16, right: 16, zIndex: 1300 }} // Added zIndex
    >
      {mode === 'light' ? 'Turn dark' : 'Turn light'}
    </Button>
  );
}

export default ModeToggle;
