import { useState, useEffect } from 'react';

export default function Dashboard({ token, onLogout }) {
  const [wallets, setWallets] = useState([]);
  const [selectedWalletId, setSelectedWalletId] = useState('');
  const [transactions, setTransactions] = useState([]);
  
  const [transferDest, setTransferDest] = useState('');
  const [transferAmount, setTransferAmount] = useState('');
  const [transferMsg, setTransferMsg] = useState({ text: '', type: '' });
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const fetchWallets = async () => {
      try {
        const res = await fetch(`http://localhost:8080/api/v1/wallets`, {
          headers: { 'Authorization': `Bearer ${token}` }
        });
        if (res.ok) {
          const data = await res.json();
          setWallets(data || []);
          if (data && data.length > 0 && !selectedWalletId) {
            setSelectedWalletId(data[0].id);
            fetchTransactions(data[0].id);
          }
        }
      } catch (err) {
        console.error(err);
      }
    };
    if (token) fetchWallets();
  }, [token]);

  const fetchTransactions = async (walletId) => {
    if (!walletId) return;
    try {
      const res = await fetch(`http://localhost:8080/api/v1/wallets/${walletId}/transactions`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.ok) {
        const data = await res.json();
        setTransactions(data || []);
      }
    } catch (err) {
      console.error(err);
    }
  };

  const createWallet = async () => {
    try {
      const res = await fetch(`http://localhost:8080/api/v1/wallets`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ currency: 'USD' })
      });
      const data = await res.json();
      if (res.ok) {
        setWallets([...wallets, { ...data, available_balance: 0, ledger_balance: 0 }]);
        if (!selectedWalletId) setSelectedWalletId(data.id);
      }
    } catch (err) {
      console.error(err);
    }
  };

  const fundWallet = async (walletId) => {
    try {
      const res = await fetch(`http://localhost:8080/api/v1/payments/demo-fund`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ wallet_id: walletId, amount: 50000, currency: 'USD' })
      });
      if (!res.ok) throw new Error('Demo fund failed on backend');
      fetchTransactions(walletId);
      // update balance in state immediately for visual feedback
      setWallets(wallets.map(w => w.id === walletId ? { ...w, available_balance: w.available_balance + 50000 } : w));
    } catch (err) {
      console.error(err);
    }
  };

  const handleTransfer = async (e) => {
    e.preventDefault();
    setLoading(true);
    setTransferMsg({ text: '', type: '' });

    try {
      const res = await fetch(`http://localhost:8080/api/v1/transfers`, {
        method: 'POST',
        headers: { 
          'Content-Type': 'application/json', 
          'Authorization': `Bearer ${token}`,
          'Idempotency-Key': `transfer-${Date.now()}`
        },
        body: JSON.stringify({
          source_wallet_id: selectedWalletId,
          destination_wallet_id: transferDest,
          amount: parseInt(transferAmount * 100), // convert to cents
          currency: 'USD'
        })
      });

      const text = await res.text();
      let data = {};
      try { data = JSON.parse(text); } catch(e) { data = { error: text.trim() }; }
      
      if (!res.ok) throw new Error(data.error || 'Transfer failed');
      
      setTransferMsg({ text: 'Transfer successful!', type: 'success' });
      setTransferDest('');
      setTransferAmount('');
      fetchTransactions(selectedWalletId);
    } catch (err) {
      setTransferMsg({ text: err.message, type: 'danger' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="container animate-fade-in">
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '40px' }}>
        <h2>Wallet Ledger Dashboard</h2>
        <button onClick={onLogout} className="btn btn-secondary">Sign Out</button>
      </header>

      <div className="grid-2">
        <div className="glass-panel">
          <h3>Your Wallets</h3>
          {wallets.length === 0 ? (
            <div style={{ padding: '24px 0', textAlign: 'center' }}>
              <p style={{ marginBottom: '16px' }}>You don't have any wallets yet.</p>
              <button onClick={createWallet} className="btn">Create USD Wallet</button>
            </div>
          ) : (
            <div style={{ marginTop: '16px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
              {wallets.map(w => (
                <div 
                  key={w.id} 
                  onClick={() => { setSelectedWalletId(w.id); fetchTransactions(w.id); }}
                  style={{ 
                    padding: '16px', 
                    border: `1px solid ${selectedWalletId === w.id ? 'var(--accent-primary)' : 'var(--border-color)'}`,
                    borderRadius: '8px',
                    cursor: 'pointer',
                    background: selectedWalletId === w.id ? 'rgba(88, 101, 242, 0.1)' : 'transparent'
                  }}
                >
                  <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)' }}>Wallet ID</div>
                  <div style={{ fontFamily: 'monospace', margin: '4px 0 12px 0' }}>{w.id}</div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div>
                      <span className="badge success">ACTIVE</span>
                      <strong style={{ marginLeft: '12px' }}>${(w.available_balance / 100).toFixed(2)} {w.currency}</strong>
                    </div>
                    {selectedWalletId === w.id && (
                      <button 
                        onClick={(e) => { e.stopPropagation(); fundWallet(w.id); }}
                        className="badge success" 
                        style={{ cursor: 'pointer', border: '1px solid var(--success)', background: 'transparent' }}
                      >
                        + $500
                      </button>
                    )}
                  </div>
                </div>
              ))}
              <button onClick={createWallet} className="btn btn-secondary" style={{ marginTop: '8px' }}>+ New Wallet</button>
            </div>
          )}
        </div>

        <div className="glass-panel">
          <h3>Transfer Funds</h3>
          <p style={{ marginBottom: '24px' }}>Send money instantly to any wallet ID.</p>
          
          <form onSubmit={handleTransfer}>
            <div className="input-group">
              <label>From Wallet</label>
              <select 
                value={selectedWalletId}
                onChange={e => { setSelectedWalletId(e.target.value); fetchTransactions(e.target.value); }}
                required
                style={{ 
                  background: 'rgba(0,0,0,0.2)', border: '1px solid var(--border-color)', 
                  color: 'white', padding: '12px', borderRadius: '8px' 
                }}
              >
                <option value="" disabled>Select a wallet</option>
                {wallets.map(w => (
                  <option key={w.id} value={w.id}>{w.id}</option>
                ))}
              </select>
            </div>

            <div className="input-group">
              <label>Destination Wallet ID</label>
              <input 
                type="text" 
                required 
                value={transferDest}
                onChange={e => setTransferDest(e.target.value)}
                placeholder="Paste destination UUID"
              />
            </div>

            <div className="input-group">
              <label>Amount (USD)</label>
              <input 
                type="number" 
                step="0.01"
                min="0.01"
                required 
                value={transferAmount}
                onChange={e => setTransferAmount(e.target.value)}
                placeholder="0.00"
              />
            </div>

            {transferMsg.text && (
              <div style={{ color: `var(--${transferMsg.type})`, marginBottom: '16px', fontSize: '0.875rem' }}>
                {transferMsg.text}
              </div>
            )}

            <button type="submit" className="btn" style={{ width: '100%' }} disabled={loading || !selectedWalletId}>
              {loading ? 'Processing...' : 'Send Funds'}
            </button>
          </form>
        </div>
      </div>

      {selectedWalletId && (
        <div className="glass-panel" style={{ marginTop: '24px' }}>
          <h3>Transaction History</h3>
          <div style={{ marginTop: '16px', overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left' }}>
              <thead>
                <tr style={{ borderBottom: '1px solid var(--border-color)', color: 'var(--text-secondary)' }}>
                  <th style={{ padding: '12px' }}>Date</th>
                  <th style={{ padding: '12px' }}>Type</th>
                  <th style={{ padding: '12px' }}>Amount</th>
                  <th style={{ padding: '12px' }}>Status</th>
                </tr>
              </thead>
              <tbody>
                {transactions.length === 0 ? (
                  <tr><td colSpan="4" style={{ padding: '24px', textAlign: 'center', color: 'var(--text-secondary)' }}>No transactions found.</td></tr>
                ) : (
                  transactions.map(t => (
                    <tr key={t.id} style={{ borderBottom: '1px solid rgba(255,255,255,0.05)' }}>
                      <td style={{ padding: '12px', fontSize: '0.875rem' }}>{new Date(t.created_at).toLocaleString()}</td>
                      <td style={{ padding: '12px' }}>
                        <span className={`badge ${t.direction === 'CREDIT' ? 'success' : 'failed'}`}>
                          {t.direction} ({t.reference_type})
                        </span>
                      </td>
                      <td style={{ padding: '12px', fontWeight: 'bold' }}>
                        {t.direction === 'CREDIT' ? '+' : '-'}${(t.amount / 100).toFixed(2)}
                      </td>
                      <td style={{ padding: '12px' }}><span className="badge pending" style={{ background: 'rgba(255,255,255,0.1)', color: '#fff' }}>{t.status}</span></td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
