import { useState, useEffect, useCallback } from 'react';
import {
  getHealth,
  getTeams,
  getSidecars,
  executeQuery,
  type Team,
  type Sidecar,
  type QueryResponse,
} from './api';

function App() {
  const [health, setHealth] = useState<{ status: string; sidecar_count: number } | null>(null);
  const [teams, setTeams] = useState<Team[]>([]);
  const [sidecars, setSidecars] = useState<Sidecar[]>([]);
  const [selectedTeam, setSelectedTeam] = useState<string>('');
  const [query, setQuery] = useState<string>('');
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [result, setResult] = useState<QueryResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Fetch initial data
  const fetchData = useCallback(async () => {
    try {
      const [healthData, teamsData, sidecarsData] = await Promise.all([
        getHealth(),
        getTeams(),
        getSidecars(),
      ]);
      setHealth(healthData);
      setTeams(teamsData.teams || []);
      setSidecars(sidecarsData.sidecars || []);

      // Auto-select first team if available
      if (teamsData.teams?.length > 0 && !selectedTeam) {
        setSelectedTeam(teamsData.teams[0].team_id);
      }
    } catch (err) {
      console.error('Failed to fetch data:', err);
      setHealth(null);
    }
  }, [selectedTeam]);

  useEffect(() => {
    fetchData();
    // Poll for updates every 10 seconds
    const interval = setInterval(fetchData, 10000);
    return () => clearInterval(interval);
  }, [fetchData]);

  // Handle query submission
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedTeam || !query.trim()) return;

    setIsLoading(true);
    setError(null);
    setResult(null);

    try {
      const response = await executeQuery(selectedTeam, query);
      setResult(response);
      if (response.error) {
        setError(response.error);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred');
    } finally {
      setIsLoading(false);
    }
  };

  // Render results table
  const renderResultsTable = () => {
    if (!result?.results) return null;
    const { columns, rows, row_count } = result.results;

    if (!columns || columns.length === 0) {
      return (
        <div className="empty-state">
          <h3>No columns returned</h3>
          <p>The query returned no data</p>
        </div>
      );
    }

    return (
      <>
        <div className="results-header">
          <h2>Results</h2>
          <span className="row-count">{row_count} rows</span>
        </div>
        <div className="results-table-wrapper">
          <table className="results-table">
            <thead>
              <tr>
                {columns.map((col) => (
                  <th key={col}>{col}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((row, idx) => (
                <tr key={idx}>
                  {columns.map((col) => (
                    <td key={col}>
                      {row[col] === null ? (
                        <em style={{ color: '#9ca3af' }}>NULL</em>
                      ) : (
                        String(row[col])
                      )}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </>
    );
  };

  return (
    <div className="app">
      <header className="header">
        <h1>AI Query Dashboard</h1>
        <div className={`status-badge ${health ? '' : 'disconnected'}`}>
          <span className="status-dot"></span>
          {health ? (
            <>
              Connected ({health.sidecar_count} sidecar{health.sidecar_count !== 1 ? 's' : ''})
            </>
          ) : (
            'Disconnected'
          )}
        </div>
      </header>

      <div className="controls">
        <div className="team-selector">
          <label htmlFor="team-select">Team:</label>
          <select
            id="team-select"
            value={selectedTeam}
            onChange={(e) => setSelectedTeam(e.target.value)}
            disabled={teams.length === 0}
          >
            {teams.length === 0 ? (
              <option value="">No teams available</option>
            ) : (
              teams.map((team) => (
                <option key={team.team_id} value={team.team_id}>
                  {team.team_id} ({team.services?.length || 0} services)
                </option>
              ))
            )}
          </select>
        </div>
      </div>

      <section className="query-section">
        <form onSubmit={handleSubmit}>
          <div className="query-input-wrapper">
            <input
              type="text"
              className="query-input"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Ask a question about your data... (e.g., 'Show me all customers')"
              disabled={!selectedTeam || isLoading}
            />
            <button
              type="submit"
              className="submit-btn"
              disabled={!selectedTeam || !query.trim() || isLoading}
            >
              {isLoading ? 'Running...' : 'Run Query'}
            </button>
          </div>
        </form>

        {result?.generated_sql && (
          <div>
            <span className="generated-sql-label">Generated SQL:</span>
            <pre className="generated-sql">{result.generated_sql}</pre>
          </div>
        )}

        {error && <div className="error-message">{error}</div>}
      </section>

      <section className="results-section">
        {isLoading ? (
          <div className="loading">
            <div className="loading-spinner"></div>
            Executing query...
          </div>
        ) : result?.results ? (
          renderResultsTable()
        ) : (
          <div className="empty-state">
            <h3>No results yet</h3>
            <p>Enter a natural language query above to get started</p>
          </div>
        )}
      </section>

      {sidecars.length > 0 && (
        <section className="sidecars-list">
          <h3>Connected Sidecars</h3>
          {sidecars.map((sc) => (
            <div key={sc.sidecar_id} className="sidecar-item">
              <span className={`sidecar-status ${sc.status !== 'healthy' ? 'unhealthy' : ''}`}></span>
              <strong>{sc.team_id}</strong>
              <span>/ {sc.service_name}</span>
              <span style={{ color: '#6c757d' }}>({sc.db_type}: {sc.db_name})</span>
            </div>
          ))}
        </section>
      )}
    </div>
  );
}

export default App;
