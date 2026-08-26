import { useState } from 'react';
import Auth from './components/Auth';
import Dashboard from './components/Dashboard';

function App() {
  const [token, setToken] = useState(localStorage.getItem('jwt_token') || '');

  const handleLogin = (newToken) => {
    localStorage.setItem('jwt_token', newToken);
    setToken(newToken);
  };

  const handleLogout = () => {
    localStorage.removeItem('jwt_token');
    setToken('');
  };

  return (
    <>
      {token ? (
        <Dashboard token={token} onLogout={handleLogout} />
      ) : (
        <Auth onLogin={handleLogin} />
      )}
    </>
  );
}

export default App;
