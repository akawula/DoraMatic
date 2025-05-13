import React, { useState } from 'react';
import authService from '../services/authService';

// Basic styling (can be moved to a CSS file)
const styles = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    padding: '20px',
    maxWidth: '400px',
    margin: '50px auto',
    border: '1px solid #ccc',
    borderRadius: '8px',
    boxShadow: '0 2px 4px rgba(0,0,0,0.1)',
  },
  form: {
    display: 'flex',
    flexDirection: 'column',
    width: '100%',
  },
  inputGroup: {
    marginBottom: '15px',
  },
  label: {
    marginBottom: '5px',
    fontWeight: 'bold',
    display: 'block',
  },
  input: {
    width: 'calc(100% - 20px)', // Account for padding
    padding: '10px',
    border: '1px solid #ddd',
    borderRadius: '4px',
    fontSize: '16px',
  },
  button: {
    padding: '10px 15px',
    backgroundColor: '#007bff',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    fontSize: '16px',
    cursor: 'pointer',
    transition: 'background-color 0.2s',
  },
  buttonHover: { // Example: inline hover style (better in CSS)
    backgroundColor: '#0056b3',
  },
  error: {
    color: 'red',
    marginTop: '10px',
    textAlign: 'center',
  },
  loading: {
    marginTop: '10px',
    textAlign: 'center',
  }
};

const Login = ({ onLoginSuccess }) => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [isButtonHovered, setIsButtonHovered] = useState(false);

  const handleSubmit = async (event) => {
    event.preventDefault();
    setError('');
    setLoading(true);

    if (!username || !password) {
      setError('Username and password are required.');
      setLoading(false);
      return;
    }

    try {
      await authService.login(username, password);
      if (onLoginSuccess) {
        onLoginSuccess(); // Notify parent component (App.js)
      }
    } catch (err) {
      setError(err.message || 'An unexpected error occurred. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={styles.container}>
      <h2>Login</h2>
      <form onSubmit={handleSubmit} style={styles.form}>
        <div style={styles.inputGroup}>
          <label htmlFor="username" style={styles.label}>Username:</label>
          <input
            type="text"
            id="username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            style={styles.input}
            disabled={loading}
          />
        </div>
        <div style={styles.inputGroup}>
          <label htmlFor="password" style={styles.label}>Password:</label>
          <input
            type="password"
            id="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            style={styles.input}
            disabled={loading}
          />
        </div>
        <button
          type="submit"
          style={isButtonHovered ? {...styles.button, ...styles.buttonHover} : styles.button}
          disabled={loading}
          onMouseEnter={() => setIsButtonHovered(true)}
          onMouseLeave={() => setIsButtonHovered(false)}
        >
          {loading ? 'Logging in...' : 'Login'}
        </button>
      </form>
      {error && <p style={styles.error}>{error}</p>}
      {loading && <p style={styles.loading}>Please wait...</p>}
    </div>
  );
};

export default Login;
