const API_URL = '/api/auth/'; // Adjust if your API prefix is different

const login = async (username, password) => {
  try {
    const response = await fetch(API_URL + 'login', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ username, password }),
    });

    if (!response.ok) {
      // Try to parse error message from backend if available
      let errorMessage = `Login failed with status: ${response.status}`;
      try {
        const errorData = await response.json();
        errorMessage = errorData.message || errorData.error || errorMessage;
      } catch (e) {
        // Ignore if error response is not JSON
      }
      throw new Error(errorMessage);
    }

    const data = await response.json();
    if (data.token) {
      localStorage.setItem('userToken', data.token);
      if (data.username) {
        localStorage.setItem('loggedInUser', data.username);
      }
    }
    return data; // Contains token and username
  } catch (error) {
    console.error('Login service error:', error);
    throw error; // Re-throw to be caught by the component
  }
};

const logout = () => {
  localStorage.removeItem('userToken');
  localStorage.removeItem('loggedInUser'); // Remove username on logout
  // Potentially call a backend logout endpoint if you implement token blocklisting
};

const getCurrentUsername = () => {
  return localStorage.getItem('loggedInUser');
};

const getToken = () => {
  return localStorage.getItem('userToken');
};

const isAuthenticated = () => {
  return !!getToken(); // Simple check: true if token exists
  // For more robustness, you might decode the token and check its expiration
};

const authService = {
  login,
  logout,
  getToken,
  getCurrentUsername,
  isAuthenticated,
};

export default authService;
