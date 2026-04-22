import { useState, useEffect, useCallback, useRef, InputHTMLAttributes, TextareaHTMLAttributes } from "react";
import "./App.css";

// Disable OS auto-correction on all text inputs (macOS forces capitalization etc.)
const inputProps: InputHTMLAttributes<HTMLInputElement> = {
  autoCapitalize: "off",
  autoCorrect: "off",
  spellCheck: false,
};
const textareaProps: TextareaHTMLAttributes<HTMLTextAreaElement> = {
  autoCapitalize: "off",
  autoCorrect: "off",
  spellCheck: false,
};

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
          Query(workspaceID: string, channelID: string, question: string): Promise<QueryResponseType>;
          GetPendingProposals(): Promise<Proposal[]>;
          ApproveProposal(id: string): Promise<void>;
          RejectProposal(id: string): Promise<void>;
          EditAndApproveProposal(id: string, text: string): Promise<void>;
          ListKnowledge(scope: string): Promise<KnowledgeEntry[]>;
          AddKnowledge(title: string, content: string, scope: string, workspaceID: string, tags: string[]): Promise<KnowledgeEntry>;
          UpdateKnowledge(id: string, title: string, content: string, scope: string, workspaceID: string, tags: string[]): Promise<void>;
          DeleteKnowledge(id: string): Promise<void>;
          GetChannelStats(workspace: string): Promise<ChannelStatsInfo[]>;
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
interface TimelineMsg { workspace_id: string; channel_id: string; channel_name: string; user_name: string; text: string; ts: string; author_type: string; }
interface ChannelInfoRemote { id: string; name: string; is_private: boolean; num_members: number; topic: string; monitored: boolean; }
interface ChannelStatsInfo { channel_id: string; channel_name: string; msg_count: number; last_ts: string; }
interface QueryResult { record_id: string; workspace_id: string; channel_id: string; channel_name: string; user_name: string; content: string; ts: string; score: number; }
interface Proposal { id: string; workspace_name: string; channel_name: string; trigger_text: string; draft_text: string; state: string; created_at: string; }
interface KnowledgeEntry { id: string; title: string; content: string; scope: string; workspace_id: string; tags: string[]; created_at: string; updated_at: string; }

type Tab = "dashboard" | "query" | "proposals" | "knowledge" | "settings";

// Convert Slack timestamp (e.g., "1713488400.000100") to readable date/time
function formatSlackTs(ts: string): string {
  if (!ts) return "";
  const secs = parseFloat(ts);
  if (isNaN(secs)) return ts;
  const d = new Date(secs * 1000);
  const now = new Date();
  const isToday = d.toDateString() === now.toDateString();
  const time = d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  if (isToday) return time;
  return d.toLocaleDateString([], { month: "short", day: "numeric" }) + " " + time;
}

function App() {
  const [tab, setTab] = useState<Tab>("dashboard");
  const [version, setVersion] = useState("");
  const [workspaces, setWorkspaces] = useState<WorkspaceStatus[]>([]);
  const [memoryStats, setMemoryStats] = useState<Record<string, number>>({});
  const [error, setError] = useState("");
  const [toast, setToast] = useState<{ type: string; message: string } | null>(null);

  // Listen for agent events from backend
  useEffect(() => {
    const onRespond = (data: any) => {
      setToast({ type: "respond", message: `Response proposal: ${data?.channel_name || ""} — ${data?.summary || ""}` });
      setTab("proposals");
      setTimeout(() => setToast(null), 8000);
    };
    const onReview = (data: any) => {
      setToast({ type: "review", message: `Action needed: ${data?.channel_name || ""} — ${data?.summary || ""}` });
      setTimeout(() => setToast(null), 8000);
    };
    // @ts-ignore — Wails runtime events
    window.runtime?.EventsOn("agent:respond", onRespond);
    // @ts-ignore
    window.runtime?.EventsOn("agent:review", onReview);
    return () => {
      // @ts-ignore
      window.runtime?.EventsOff("agent:respond");
      // @ts-ignore
      window.runtime?.EventsOff("agent:review");
    };
  }, []);

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

      {toast && (
        <div className={`toast toast-${toast.type}`} onClick={() => setToast(null)}>
          {toast.type === "respond" ? "💬 " : "⚠️ "}{toast.message}
        </div>
      )}

      {error && <div className="error">{error}</div>}

      {tab === "dashboard" && <DashboardTab workspaces={workspaces} memoryStats={memoryStats} />}
      {tab === "query" && <QueryTab setError={setError} />}
      {tab === "proposals" && <ProposalsTab setError={setError} />}
      {tab === "knowledge" && <KnowledgeTab setError={setError} />}
      {tab === "settings" && <SettingsTab setError={setError} onRefresh={refresh} />}
    </div>
  );
}

// ── Dashboard: live timeline + overview ─────────────────

const MAX_TIMELINE = 100;

function DashboardTab({ workspaces, memoryStats }: {
  workspaces: WorkspaceStatus[]; memoryStats: Record<string, number>;
}) {
  const [timeline, setTimeline] = useState<TimelineMsg[]>([]);
  const timelineRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onMessage = (msg: TimelineMsg) => {
      setTimeline((prev) => {
        const next = [msg, ...prev];
        return next.length > MAX_TIMELINE ? next.slice(0, MAX_TIMELINE) : next;
      });
    };
    // @ts-ignore
    window.runtime?.EventsOn("timeline:message", onMessage);
    return () => {
      // @ts-ignore
      window.runtime?.EventsOff("timeline:message");
    };
  }, []);

  const totalMessages = Object.values(memoryStats).reduce((a, b) => a + b, 0);

  return (
    <>
      <section className="section">
        <div className="stats-row">
          {workspaces.map((ws) => (
            <div key={ws.name} className="stat-compact">
              <span className="workspace-name">{ws.name}</span>
              <span className={`badge ${ws.polling ? "badge-active" : "badge-inactive"}`}>{ws.polling ? "Polling" : "Stopped"}</span>
              <span className="badge badge-ok">{ws.num_channels} ch</span>
            </div>
          ))}
          <div className="stat-compact">
            <span className="stat-value-sm">{totalMessages}</span>
            <span className="muted">total</span>
          </div>
          {["hot", "warm", "cold"].map((tier) => (
            <div key={tier} className="stat-compact">
              <span className="stat-value-sm">{memoryStats[tier] || 0}</span>
              <span className="muted">{tier}</span>
            </div>
          ))}
        </div>
      </section>

      <div className="timeline-container" ref={timelineRef}>
        {timeline.length === 0 ? (
          <p className="muted" style={{ padding: 24 }}>Waiting for new messages...</p>
        ) : (
          timeline.map((msg, i) => (
            <div key={`${msg.ts}-${i}`} className={`bubble ${msg.author_type === "self" ? "bubble-self" : ""} ${msg.author_type === "bot" ? "bubble-bot" : ""}`}>
              <div className="bubble-header">
                <span className="bubble-channel">#{msg.channel_name || msg.channel_id}</span>
                <span className="bubble-user">{msg.user_name || "unknown"}</span>
                {msg.author_type === "bot" && <span className="badge badge-inactive">bot</span>}
                {msg.author_type === "self" && <span className="badge badge-ok">you</span>}
                <span className="bubble-time">{formatSlackTs(msg.ts)}</span>
              </div>
              <div className="bubble-text">{msg.text.length > 500 ? msg.text.slice(0, 500) + "..." : msg.text}</div>
            </div>
          ))
        )}
      </div>
    </>
  );
}

// ── Settings: workspace setup (token + channels + polling) ─

function SettingsTab({ setError, onRefresh }: { setError: (e: string) => void; onRefresh: () => void }) {
  const [workspaces, setWorkspaces] = useState<WorkspaceStatus[]>([]);
  const [newWsName, setNewWsName] = useState("");
  // Per-workspace UI state
  const [tokenWs, setTokenWs] = useState<string | null>(null);
  const [tokenValue, setTokenValue] = useState("");
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

  const handleLoadChannels = async (ws: string) => {
    setChannelWs(ws);
    setLoadingChannels(true);
    try {
      const chs = await window.go.main.App.ListAvailableChannels(ws, false);
      setChannels(chs || []);
    } catch (e: any) { setError(e?.message || String(e)); }
    setLoadingChannels(false);
  };

  const handleToggleChannel = (chId: string) => {
    setChannels((prev) => prev.map((ch) => ch.id === chId ? { ...ch, monitored: !ch.monitored } : ch));
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
        <h3>Add Workspace</h3>
        <div className="form-row" style={{ marginBottom: 16 }}>
          <input {...inputProps} placeholder="Workspace name" value={newWsName} onChange={(e) => setNewWsName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleAddWs()} />
          <button className="btn-approve" onClick={handleAddWs}>Add</button>
        </div>
      </div>

      {workspaces.map((ws) => (
        <div key={ws.name} className="ws-setup-card">
          <div className="ws-setup-header">
            <span className="workspace-name">{ws.name}</span>
            <div className="workspace-actions">
              {ws.has_token && !ws.polling && ws.num_channels > 0 && (
                <button className="btn-approve" onClick={() => handle(() => window.go.main.App.StartPolling(ws.name))}>Start Polling</button>
              )}
              {ws.polling && (
                <button onClick={() => handle(() => window.go.main.App.StopPolling(ws.name))}>Stop</button>
              )}
              <button className="btn-reject" onClick={() => handle(() => window.go.main.App.RemoveWorkspace(ws.name))}>Remove</button>
            </div>
          </div>

          {/* Step 1: Token */}
          <div className="ws-setup-step">
            <span className="step-label">1. Token</span>
            {tokenWs === ws.name ? (
              <div className="token-form">
                <input {...inputProps} type="password" placeholder="xoxp-..." value={tokenValue}
                  onChange={(e) => setTokenValue(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleSaveToken(ws.name)} autoFocus />
                <button className="btn-approve" onClick={() => handleSaveToken(ws.name)}>Save</button>
                <button onClick={() => setTokenWs(null)}>Cancel</button>
              </div>
            ) : (
              <div className="step-status">
                <span className={`badge ${ws.has_token ? "badge-ok" : "badge-warn"}`}>
                  {ws.has_token ? "Configured" : "Not set"}
                </span>
                <button className="btn-muted" onClick={() => { setTokenWs(ws.name); setTokenValue(""); }}>
                  {ws.has_token ? "Update" : "Set Token"}
                </button>
              </div>
            )}
          </div>

          {/* Step 2: Channels */}
          <div className="ws-setup-step">
            <span className="step-label">2. Channels</span>
            {channelWs === ws.name ? (
              <div className="channel-selector">
                <div className="channel-selector-actions">
                  <button className="btn-approve" onClick={handleSaveChannels}>
                    Save ({channels.filter((c) => c.monitored).length} selected)
                  </button>
                  <button onClick={() => { setChannelWs(null); setChannels([]); }}>Cancel</button>
                </div>
                {loadingChannels ? (
                  <p className="muted">Loading channels from Slack...</p>
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
            ) : (
              <div className="step-status">
                <span className={`badge ${ws.num_channels > 0 ? "badge-ok" : "badge-warn"}`}>
                  {ws.num_channels > 0 ? `${ws.num_channels} channels` : "None selected"}
                </span>
                {ws.has_token && (
                  <button className="btn-muted" onClick={() => handleLoadChannels(ws.name)}>Select Channels</button>
                )}
                {!ws.has_token && <span className="muted">Set token first</span>}
              </div>
            )}
          </div>

          {/* Step 3: Status */}
          <div className="ws-setup-step">
            <span className="step-label">3. Status</span>
            <div className="step-status">
              <span className={`badge ${ws.polling ? "badge-active" : "badge-inactive"}`}>
                {ws.polling ? "Polling" : "Stopped"}
              </span>
              {!ws.has_token && <span className="muted">Complete steps 1-2 to start</span>}
              {ws.has_token && ws.num_channels === 0 && <span className="muted">Select channels to start</span>}
            </div>
          </div>
        </div>
      ))}
    </section>
  );
}

// ── Query ──────────────────────────────────────────────

interface QueryResponseType { answer: string; sources: QueryResult[]; }

function QueryTab({ setError }: { setError: (e: string) => void }) {
  const [queryWs, setQueryWs] = useState("");
  const [queryCh, setQueryCh] = useState("");
  const [query, setQuery] = useState("");
  const [response, setResponse] = useState<QueryResponseType | null>(null);
  const [loading, setLoading] = useState(false);

  const handleQuery = async () => {
    if (!queryWs || !queryCh || !query) return;
    try {
      setError(""); setLoading(true);
      const resp = await window.go.main.App.Query(queryWs, queryCh, query);
      setResponse(resp);
    } catch (e: any) { setError(e?.message || String(e)); }
    finally { setLoading(false); }
  };

  return (
    <section className="section">
      <h2>Query</h2>
      <div className="query-form">
        <input {...inputProps} placeholder="Workspace ID" value={queryWs} onChange={(e) => setQueryWs(e.target.value)} />
        <input {...inputProps} placeholder="Channel ID" value={queryCh} onChange={(e) => setQueryCh(e.target.value)} />
        <input {...inputProps} placeholder="Ask a question..." value={query} onChange={(e) => setQuery(e.target.value)} onKeyDown={(e) => e.key === "Enter" && !loading && handleQuery()} />
        <button onClick={handleQuery} disabled={loading}>{loading ? "Thinking..." : "Ask"}</button>
      </div>

      {loading && (
        <div className="loading-indicator">
          <span className="loading-spinner" />
          Searching knowledge base and generating answer...
        </div>
      )}

      {response?.answer && (
        <div className="answer-card">
          <h3>Answer</h3>
          <div className="answer-text">{response.answer}</div>
        </div>
      )}

      {response?.sources && response.sources.length > 0 && (
        <div className="results">
          <h3>Sources ({response.sources.length})</h3>
          {response.sources.map((r, i) => (
            <div key={i} className="result-card">
              <div className="result-meta">
                <span>#{r.channel_name || r.channel_id}</span>
                <span className="muted">{r.user_name}</span>
                <span className="score">{r.score.toFixed(3)}</span>
              </div>
              <div className="source-content">{r.content?.slice(0, 300)}{(r.content?.length || 0) > 300 ? "..." : ""}</div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

// ── Proposals (MITL) ───────────────────────────────────

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
                <textarea {...textareaProps} className="proposal-edit" value={editText} onChange={(e) => setEditText(e.target.value)} rows={4} />
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

// ── Knowledge ──────────────────────────────────────────

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
          <input {...inputProps} placeholder="Title" value={title} onChange={(e) => setTitle(e.target.value)} />
          <textarea {...textareaProps} placeholder="Content" value={content} onChange={(e) => setContent(e.target.value)} rows={4} />
          <div className="form-row">
            <select value={scope} onChange={(e) => setScope(e.target.value)}>
              <option value="global">Global (L3)</option>
              <option value="workspace">Workspace (L2)</option>
            </select>
            {scope === "workspace" && <input {...inputProps} placeholder="Workspace ID" value={wsId} onChange={(e) => setWsId(e.target.value)} />}
            <input {...inputProps} placeholder="Tags (comma-separated)" value={tags} onChange={(e) => setTags(e.target.value)} />
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

export default App;
