const API_BASE = '/api/v1';

export interface Team {
  team_id: string;
  services: string[];
}

export interface Sidecar {
  sidecar_id: string;
  team_id: string;
  service_name: string;
  status: string;
  db_type: string;
  db_name: string;
  last_seen: string;
}

export interface QueryResult {
  columns: string[];
  rows: Record<string, unknown>[];
  row_count: number;
}

export interface QueryResponse {
  query_id: string;
  generated_sql: string;
  results?: QueryResult;
  error?: string;
  metadata?: Record<string, unknown>;
}

export interface HealthResponse {
  status: string;
  time: string;
  sidecar_count: number;
}

// Fetch health status
export async function getHealth(): Promise<HealthResponse> {
  const response = await fetch(`${API_BASE}/health`);
  if (!response.ok) {
    throw new Error(`Health check failed: ${response.statusText}`);
  }
  return response.json();
}

// Fetch all teams
export async function getTeams(): Promise<{ teams: Team[]; count: number }> {
  const response = await fetch(`${API_BASE}/teams`);
  if (!response.ok) {
    throw new Error(`Failed to fetch teams: ${response.statusText}`);
  }
  return response.json();
}

// Fetch all sidecars
export async function getSidecars(): Promise<{ sidecars: Sidecar[]; count: number }> {
  const response = await fetch(`${API_BASE}/sidecars`);
  if (!response.ok) {
    throw new Error(`Failed to fetch sidecars: ${response.statusText}`);
  }
  return response.json();
}

// Fetch sidecars for a specific team
export async function getTeamSidecars(teamId: string): Promise<{ sidecars: Sidecar[]; count: number }> {
  const response = await fetch(`${API_BASE}/teams/${teamId}/sidecars`);
  if (!response.ok) {
    throw new Error(`Failed to fetch team sidecars: ${response.statusText}`);
  }
  return response.json();
}

// Execute a query
export async function executeQuery(teamId: string, query: string): Promise<QueryResponse> {
  const response = await fetch(`${API_BASE}/teams/${teamId}/query`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ query }),
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(error.error || `Query failed: ${response.statusText}`);
  }

  return response.json();
}
