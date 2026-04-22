import { useState, useEffect, useCallback } from "react";
import "./App.css";

// Wails bindings (generated at build time, typed manually for now)
declare global {
  interface Window {
    go: {
      main: {
        App: {
          Version(): Promise<string>;
          GetWorkspaces(): Promise<WorkspaceStatus[]>;
          GetMemoryStats(): Promise<Record<string, number>>;
          StartPolling(workspace: string): Promise<void>;
          StopPolling(workspace: string): Promise<void>;
          SetWorkspaceToken(workspace: string, token: string): Promise<void>;
          Query(workspaceID: string, channelID: string, question: string): Promise<QueryResult[]>;
        };
      };
    };
  }
}

interface WorkspaceStatus {
  name: string;
  has_token: boolean;
  polling: boolean;
}

interface QueryResult {
  record_id: string;
  workspace_id: string;
  channel_id: string;
  score: number;
}

function App() {
  const [version, setVersion] = useState("");
  const [workspaces, setWorkspaces] = useState<WorkspaceStatus[]>([]);
  const [memoryStats, setMemoryStats] = useState<Record<string, number>>({});
  const [query, setQuery] = useState("");
  const [queryWs, setQueryWs] = useState("");
  const [queryCh, setQueryCh] = useState("");
  const [results, setResults] = useState<QueryResult[]>([]);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    try {
      const [ver, ws, stats] = await Promise.all([
        window.go.main.App.Version(),
        window.go.main.App.GetWorkspaces(),
        window.go.main.App.GetMemoryStats(),
      ]);
      setVersion(ver);
      setWorkspaces(ws || []);
      setMemoryStats(stats || {});
    } catch (e) {
      console.error("refresh error:", e);
    }
  }, []);

  useEffect(() => {
    refresh();
    const interval = setInterval(refresh, 5000);
    return () => clearInterval(interval);
  }, [refresh]);

  const handleStartPolling = async (ws: string) => {
    try {
      setError("");
      await window.go.main.App.StartPolling(ws);
      await refresh();
    } catch (e: any) {
      setError(e?.message || String(e));
    }
  };

  const handleStopPolling = async (ws: string) => {
    try {
      setError("");
      await window.go.main.App.StopPolling(ws);
      await refresh();
    } catch (e: any) {
      setError(e?.message || String(e));
    }
  };

  const handleQuery = async () => {
    if (!queryWs || !queryCh || !query) return;
    try {
      setError("");
      const res = await window.go.main.App.Query(queryWs, queryCh, query);
      setResults(res || []);
    } catch (e: any) {
      setError(e?.message || String(e));
    }
  };

  return (
    <div className="app">
      <header className="header">
        <h1>slack-personal-agent</h1>
        <span className="version">v{version}</span>
      </header>

      {error && <div className="error">{error}</div>}

      <section className="section">
        <h2>Workspaces</h2>
        {workspaces.length === 0 ? (
          <p className="muted">No workspaces configured. Edit config.toml to add workspaces.</p>
        ) : (
          <div className="workspace-list">
            {workspaces.map((ws) => (
              <div key={ws.name} className="workspace-card">
                <div className="workspace-info">
                  <span className="workspace-name">{ws.name}</span>
                  <span className={`badge ${ws.has_token ? "badge-ok" : "badge-warn"}`}>
                    {ws.has_token ? "Token set" : "No token"}
                  </span>
                  <span className={`badge ${ws.polling ? "badge-active" : "badge-inactive"}`}>
                    {ws.polling ? "Polling" : "Stopped"}
                  </span>
                </div>
                <div className="workspace-actions">
                  {ws.has_token && !ws.polling && (
                    <button onClick={() => handleStartPolling(ws.name)}>Start</button>
                  )}
                  {ws.polling && (
                    <button onClick={() => handleStopPolling(ws.name)}>Stop</button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      <section className="section">
        <h2>Memory</h2>
        <div className="stats-grid">
          <div className="stat">
            <span className="stat-value">{memoryStats["hot"] || 0}</span>
            <span className="stat-label">Hot</span>
          </div>
          <div className="stat">
            <span className="stat-value">{memoryStats["warm"] || 0}</span>
            <span className="stat-label">Warm</span>
          </div>
          <div className="stat">
            <span className="stat-value">{memoryStats["cold"] || 0}</span>
            <span className="stat-label">Cold</span>
          </div>
        </div>
      </section>

      <section className="section">
        <h2>Query</h2>
        <div className="query-form">
          <input
            type="text"
            placeholder="Workspace ID"
            value={queryWs}
            onChange={(e) => setQueryWs(e.target.value)}
          />
          <input
            type="text"
            placeholder="Channel ID"
            value={queryCh}
            onChange={(e) => setQueryCh(e.target.value)}
          />
          <input
            type="text"
            placeholder="Ask a question..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleQuery()}
          />
          <button onClick={handleQuery}>Search</button>
        </div>

        {results.length > 0 && (
          <div className="results">
            {results.map((r, i) => (
              <div key={i} className="result-card">
                <div className="result-meta">
                  <span>{r.workspace_id} / {r.channel_id}</span>
                  <span className="score">Score: {r.score.toFixed(4)}</span>
                </div>
                <div className="result-id">{r.record_id}</div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

export default App;
