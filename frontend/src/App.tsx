import { useState, useEffect, useCallback } from "react";
import "./App.css";

declare global {
  interface Window {
    go: {
      main: {
        App: {
          Version(): Promise<string>;
          GetTimeContext(): Promise<string>;
          GetWorkspaces(): Promise<WorkspaceStatus[]>;
          GetMemoryStats(): Promise<Record<string, number>>;
          StartPolling(workspace: string): Promise<void>;
          StopPolling(workspace: string): Promise<void>;
          SetWorkspaceToken(workspace: string, token: string): Promise<void>;
          Query(workspaceID: string, channelID: string, question: string): Promise<QueryResult[]>;
          GetPendingProposals(): Promise<Proposal[]>;
          ApproveProposal(id: string): Promise<void>;
          RejectProposal(id: string): Promise<void>;
          EditAndApproveProposal(id: string, text: string): Promise<void>;
          ListKnowledge(scope: string): Promise<KnowledgeEntry[]>;
          AddKnowledge(title: string, content: string, scope: string, workspaceID: string, tags: string[]): Promise<KnowledgeEntry>;
          UpdateKnowledge(id: string, title: string, content: string, scope: string, workspaceID: string, tags: string[]): Promise<void>;
          DeleteKnowledge(id: string): Promise<void>;
          AddWorkspace(name: string): Promise<void>;
          RemoveWorkspace(name: string): Promise<void>;
          ListAvailableChannels(workspace: string, forceRefresh: boolean): Promise<ChannelInfoRemote[]>;
          SetMonitoredChannels(workspace: string, channelIDs: string[]): Promise<void>;
        };
      };
    };
  }
}

interface WorkspaceStatus { name: string; has_token: boolean; polling: boolean; num_channels: number; }
interface ChannelInfoRemote { id: string; name: string; is_private: boolean; num_members: number; topic: string; monitored: boolean; }
interface QueryResult { record_id: string; workspace_id: string; channel_id: string; score: number; }
interface Proposal { id: string; workspace_name: string; channel_name: string; trigger_text: string; draft_text: string; state: string; created_at: string; }
interface KnowledgeEntry { id: string; title: string; content: string; scope: string; workspace_id: string; tags: string[]; created_at: string; updated_at: string; }

type Tab = "dashboard" | "query" | "proposals" | "knowledge" | "settings";

function App() {
  const [tab, setTab] = useState<Tab>("dashboard");
  const [version, setVersion] = useState("");
  const [workspaces, setWorkspaces] = useState<WorkspaceStatus[]>([]);
  const [memoryStats, setMemoryStats] = useState<Record<string, number>>({});
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

  return (
    <div className="app">
      <header className="header">
        <h1>slack-personal-agent</h1>
        <span className="version">v{version}</span>
      </header>

      <nav className="tabs">
        {(["dashboard", "query", "proposals", "knowledge", "settings"] as Tab[]).map((t) => (
          <button key={t} className={`tab ${tab === t ? "tab-active" : ""}`} onClick={() => setTab(t)}>
            {t.charAt(0).toUpperCase() + t.slice(1)}
          </button>
        ))}
      </nav>

      {error && <div className="error">{error}</div>}

      {tab === "dashboard" && <DashboardTab workspaces={workspaces} memoryStats={memoryStats} setError={setError} onRefresh={refresh} />}
      {tab === "query" && <QueryTab setError={setError} />}
      {tab === "proposals" && <ProposalsTab setError={setError} />}
      {tab === "knowledge" && <KnowledgeTab setError={setError} />}
      {tab === "settings" && <SettingsTab setError={setError} onRefresh={refresh} />}
    </div>
  );
}

function DashboardTab({ workspaces, memoryStats, setError, onRefresh }: {
  workspaces: WorkspaceStatus[]; memoryStats: Record<string, number>;
  setError: (e: string) => void; onRefresh: () => void;
}) {
  const [tokenWs, setTokenWs] = useState<string | null>(null);
  const [tokenValue, setTokenValue] = useState("");

  const handle = async (fn: () => Promise<void>) => {
    try { setError(""); await fn(); onRefresh(); } catch (e: any) { setError(e?.message || String(e)); }
  };

  const handleSaveToken = async (ws: string) => {
    if (!tokenValue.startsWith("xoxp-")) {
      setError("Token must start with xoxp-");
      return;
    }
    await handle(async () => {
      await window.go.main.App.SetWorkspaceToken(ws, tokenValue);
      setTokenWs(null);
      setTokenValue("");
    });
  };

  return (
    <>
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
                  <span className={`badge ${ws.has_token ? "badge-ok" : "badge-warn"}`}>{ws.has_token ? "Token set" : "No token"}</span>
                  <span className={`badge ${ws.num_channels > 0 ? "badge-ok" : "badge-warn"}`}>{ws.num_channels} ch</span>
                  <span className={`badge ${ws.polling ? "badge-active" : "badge-inactive"}`}>{ws.polling ? "Polling" : "Stopped"}</span>
                </div>
                <div className="workspace-actions">
                  {!ws.has_token && <button onClick={() => { setTokenWs(ws.name); setTokenValue(""); }}>Set Token</button>}
                  {ws.has_token && !ws.polling && ws.num_channels > 0 && <button onClick={() => handle(() => window.go.main.App.StartPolling(ws.name))}>Start</button>}
                  {ws.has_token && ws.num_channels === 0 && <span className="muted">Select channels in Settings</span>}
                  {ws.has_token && !ws.polling && <button className="btn-muted" onClick={() => { setTokenWs(ws.name); setTokenValue(""); }}>Update Token</button>}
                  {ws.polling && <button onClick={() => handle(() => window.go.main.App.StopPolling(ws.name))}>Stop</button>}
                </div>
                {tokenWs === ws.name && (
                  <div className="token-form">
                    <input
                      type="password"
                      placeholder="xoxp-..."
                      value={tokenValue}
                      onChange={(e) => setTokenValue(e.target.value)}
                      onKeyDown={(e) => e.key === "Enter" && handleSaveToken(ws.name)}
                      autoFocus
                    />
                    <button className="btn-approve" onClick={() => handleSaveToken(ws.name)}>Save to Keychain</button>
                    <button onClick={() => setTokenWs(null)}>Cancel</button>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </section>
      <section className="section">
        <h2>Memory</h2>
        <div className="stats-grid">
          {["hot", "warm", "cold"].map((tier) => (
            <div key={tier} className="stat">
              <span className="stat-value">{memoryStats[tier] || 0}</span>
              <span className="stat-label">{tier}</span>
            </div>
          ))}
        </div>
      </section>
    </>
  );
}

function QueryTab({ setError }: { setError: (e: string) => void }) {
  const [queryWs, setQueryWs] = useState("");
  const [queryCh, setQueryCh] = useState("");
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<QueryResult[]>([]);

  const handleQuery = async () => {
    if (!queryWs || !queryCh || !query) return;
    try { setError(""); setResults(await window.go.main.App.Query(queryWs, queryCh, query) || []); }
    catch (e: any) { setError(e?.message || String(e)); }
  };

  return (
    <section className="section">
      <h2>Query</h2>
      <div className="query-form">
        <input placeholder="Workspace ID" value={queryWs} onChange={(e) => setQueryWs(e.target.value)} />
        <input placeholder="Channel ID" value={queryCh} onChange={(e) => setQueryCh(e.target.value)} />
        <input placeholder="Ask a question..." value={query} onChange={(e) => setQuery(e.target.value)} onKeyDown={(e) => e.key === "Enter" && handleQuery()} />
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
  );
}

function ProposalsTab({ setError }: { setError: (e: string) => void }) {
  const [proposals, setProposals] = useState<Proposal[]>([]);
  const [editId, setEditId] = useState<string | null>(null);
  const [editText, setEditText] = useState("");

  const refresh = useCallback(async () => {
    try { setProposals(await window.go.main.App.GetPendingProposals() || []); }
    catch (e) { console.error(e); }
  }, []);

  useEffect(() => { refresh(); const i = setInterval(refresh, 3000); return () => clearInterval(i); }, [refresh]);

  const handle = async (fn: () => Promise<void>) => {
    try { setError(""); await fn(); refresh(); } catch (e: any) { setError(e?.message || String(e)); }
  };

  return (
    <section className="section">
      <h2>Pending Proposals</h2>
      {proposals.length === 0 ? (
        <p className="muted">No pending proposals.</p>
      ) : (
        <div className="results">
          {proposals.map((p) => (
            <div key={p.id} className="proposal-card">
              <div className="proposal-header">
                <span className="badge badge-active">{p.workspace_name} / {p.channel_name}</span>
              </div>
              <div className="proposal-trigger"><strong>Trigger:</strong> {p.trigger_text}</div>
              {editId === p.id ? (
                <textarea className="proposal-edit" value={editText} onChange={(e) => setEditText(e.target.value)} rows={4} />
              ) : (
                <div className="proposal-draft">{p.draft_text}</div>
              )}
              <div className="proposal-actions">
                {editId === p.id ? (
                  <>
                    <button className="btn-approve" onClick={() => handle(async () => { await window.go.main.App.EditAndApproveProposal(p.id, editText); setEditId(null); })}>Send edited</button>
                    <button onClick={() => setEditId(null)}>Cancel</button>
                  </>
                ) : (
                  <>
                    <button className="btn-approve" onClick={() => handle(() => window.go.main.App.ApproveProposal(p.id))}>Approve</button>
                    <button onClick={() => { setEditId(p.id); setEditText(p.draft_text); }}>Edit</button>
                    <button className="btn-reject" onClick={() => handle(() => window.go.main.App.RejectProposal(p.id))}>Reject</button>
                  </>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function KnowledgeTab({ setError }: { setError: (e: string) => void }) {
  const [entries, setEntries] = useState<KnowledgeEntry[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [scope, setScope] = useState("global");
  const [wsId, setWsId] = useState("");
  const [tags, setTags] = useState("");

  const refresh = useCallback(async () => {
    try { setEntries(await window.go.main.App.ListKnowledge("") || []); }
    catch (e) { console.error(e); }
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const handleAdd = async () => {
    if (!title || !content) return;
    try {
      setError("");
      const tagList = tags ? tags.split(",").map((t) => t.trim()).filter(Boolean) : [];
      await window.go.main.App.AddKnowledge(title, content, scope, wsId, tagList);
      setTitle(""); setContent(""); setTags(""); setShowForm(false);
      refresh();
    } catch (e: any) { setError(e?.message || String(e)); }
  };

  const handleDelete = async (id: string) => {
    try { setError(""); await window.go.main.App.DeleteKnowledge(id); refresh(); }
    catch (e: any) { setError(e?.message || String(e)); }
  };

  return (
    <section className="section">
      <div className="section-header">
        <h2>Knowledge Base</h2>
        <button onClick={() => setShowForm(!showForm)}>{showForm ? "Cancel" : "Add"}</button>
      </div>

      {showForm && (
        <div className="knowledge-form">
          <input placeholder="Title" value={title} onChange={(e) => setTitle(e.target.value)} />
          <textarea placeholder="Content" value={content} onChange={(e) => setContent(e.target.value)} rows={4} />
          <div className="form-row">
            <select value={scope} onChange={(e) => setScope(e.target.value)}>
              <option value="global">Global (L3)</option>
              <option value="workspace">Workspace (L2)</option>
            </select>
            {scope === "workspace" && <input placeholder="Workspace ID" value={wsId} onChange={(e) => setWsId(e.target.value)} />}
            <input placeholder="Tags (comma-separated)" value={tags} onChange={(e) => setTags(e.target.value)} />
          </div>
          <button className="btn-approve" onClick={handleAdd}>Save</button>
        </div>
      )}

      {entries.length === 0 ? (
        <p className="muted">No knowledge entries. Add your first entry above.</p>
      ) : (
        <div className="results">
          {entries.map((e) => (
            <div key={e.id} className="result-card">
              <div className="result-meta">
                <span><strong>{e.title}</strong></span>
                <span className="badge badge-ok">{e.scope === "global" ? "Global" : `WS: ${e.workspace_id}`}</span>
              </div>
              <div className="knowledge-content">{e.content.slice(0, 200)}{e.content.length > 200 ? "..." : ""}</div>
              {e.tags && e.tags.length > 0 && (
                <div className="knowledge-tags">{e.tags.map((t, i) => <span key={i} className="tag">{t}</span>)}</div>
              )}
              <div className="proposal-actions">
                <button className="btn-reject" onClick={() => handleDelete(e.id)}>Delete</button>
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function SettingsTab({ setError, onRefresh }: { setError: (e: string) => void; onRefresh: () => void }) {
  const [workspaces, setWorkspaces] = useState<WorkspaceStatus[]>([]);
  const [newWsName, setNewWsName] = useState("");
  const [channelWs, setChannelWs] = useState<string | null>(null);
  const [channels, setChannels] = useState<ChannelInfoRemote[]>([]);
  const [loadingChannels, setLoadingChannels] = useState(false);

  const refresh = useCallback(async () => {
    try { setWorkspaces(await window.go.main.App.GetWorkspaces() || []); }
    catch (e) { console.error(e); }
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const handle = async (fn: () => Promise<void>) => {
    try { setError(""); await fn(); refresh(); onRefresh(); } catch (e: any) { setError(e?.message || String(e)); }
  };

  const handleAddWs = async () => {
    if (!newWsName.trim()) return;
    await handle(async () => {
      await window.go.main.App.AddWorkspace(newWsName.trim());
      setNewWsName("");
    });
  };

  const handleLoadChannels = async (ws: string) => {
    setChannelWs(ws);
    setLoadingChannels(true);
    try {
      const chs = await window.go.main.App.ListAvailableChannels(ws, false);
      setChannels(chs || []);
    } catch (e: any) {
      setError(e?.message || String(e));
    }
    setLoadingChannels(false);
  };

  const handleToggleChannel = (chId: string) => {
    setChannels((prev) =>
      prev.map((ch) => ch.id === chId ? { ...ch, monitored: !ch.monitored } : ch)
    );
  };

  const handleSaveChannels = async () => {
    if (!channelWs) return;
    const selected = channels.filter((ch) => ch.monitored).map((ch) => ch.id);
    await handle(async () => {
      await window.go.main.App.SetMonitoredChannels(channelWs, selected);
      setChannelWs(null);
      setChannels([]);
    });
  };

  return (
    <section className="section">
      <h2>Settings</h2>

      <div className="subsection">
        <h3>Workspaces</h3>
        <div className="form-row" style={{ marginBottom: 12 }}>
          <input placeholder="Workspace name" value={newWsName} onChange={(e) => setNewWsName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleAddWs()} />
          <button className="btn-approve" onClick={handleAddWs}>Add Workspace</button>
        </div>

        {workspaces.map((ws) => (
          <div key={ws.name} className="workspace-card">
            <div className="workspace-info">
              <span className="workspace-name">{ws.name}</span>
              <span className={`badge ${ws.num_channels > 0 ? "badge-ok" : "badge-warn"}`}>{ws.num_channels} channels</span>
            </div>
            <div className="workspace-actions">
              <button onClick={() => handleLoadChannels(ws.name)}>Select Channels</button>
              <button className="btn-reject" onClick={() => handle(() => window.go.main.App.RemoveWorkspace(ws.name))}>Remove</button>
            </div>
          </div>
        ))}
      </div>

      {channelWs && (
        <div className="subsection">
          <div className="section-header">
            <h3>Channels for {channelWs}</h3>
            <div className="workspace-actions">
              <button className="btn-approve" onClick={handleSaveChannels}>Save Selection</button>
              <button onClick={() => { setChannelWs(null); setChannels([]); }}>Cancel</button>
            </div>
          </div>
          {loadingChannels ? (
            <p className="muted">Loading channels...</p>
          ) : (
            <div className="channel-list">
              {channels.map((ch) => (
                <label key={ch.id} className="channel-item">
                  <input type="checkbox" checked={ch.monitored} onChange={() => handleToggleChannel(ch.id)} />
                  <span className="channel-name">#{ch.name}</span>
                  {ch.is_private && <span className="badge badge-warn">private</span>}
                  <span className="muted">{ch.num_members} members</span>
                </label>
              ))}
            </div>
          )}
        </div>
      )}
    </section>
  );
}

export default App;
