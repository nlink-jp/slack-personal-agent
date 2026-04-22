export namespace config {
	
	export class WindowConfig {
	    x: number;
	    y: number;
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new WindowConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}
	export class ScopeMember {
	    workspace: string;
	    channel: string;
	
	    static createFrom(source: any = {}) {
	        return new ScopeMember(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspace = source["workspace"];
	        this.channel = source["channel"];
	    }
	}
	export class ScopeGroup {
	    name: string;
	    members: ScopeMember[];
	
	    static createFrom(source: any = {}) {
	        return new ScopeGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.members = this.convertValues(source["members"], ScopeMember);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ResponseConfig {
	    timeout_sec: number;
	    signature: string;
	
	    static createFrom(source: any = {}) {
	        return new ResponseConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timeout_sec = source["timeout_sec"];
	        this.signature = source["signature"];
	    }
	}
	export class MemoryConfig {
	    hot_to_warm_min: number;
	    warm_to_cold_min: number;
	
	    static createFrom(source: any = {}) {
	        return new MemoryConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hot_to_warm_min = source["hot_to_warm_min"];
	        this.warm_to_cold_min = source["warm_to_cold_min"];
	    }
	}
	export class PollingConfig {
	    interval_sec: number;
	    priority_boost_sec: number;
	    max_rate_per_min: number;
	
	    static createFrom(source: any = {}) {
	        return new PollingConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.interval_sec = source["interval_sec"];
	        this.priority_boost_sec = source["priority_boost_sec"];
	        this.max_rate_per_min = source["max_rate_per_min"];
	    }
	}
	export class EmbeddingVertexAIConfig {
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new EmbeddingVertexAIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	    }
	}
	export class EmbeddingLocalConfig {
	    endpoint: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new EmbeddingLocalConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = source["endpoint"];
	        this.model = source["model"];
	    }
	}
	export class EmbeddingConfig {
	    backend: string;
	    local: EmbeddingLocalConfig;
	    vertex_ai: EmbeddingVertexAIConfig;
	
	    static createFrom(source: any = {}) {
	        return new EmbeddingConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backend = source["backend"];
	        this.local = this.convertValues(source["local"], EmbeddingLocalConfig);
	        this.vertex_ai = this.convertValues(source["vertex_ai"], EmbeddingVertexAIConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LocalLLMConfig {
	    endpoint: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalLLMConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = source["endpoint"];
	        this.model = source["model"];
	    }
	}
	export class VertexAIConfig {
	    project: string;
	    region: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new VertexAIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.project = source["project"];
	        this.region = source["region"];
	        this.model = source["model"];
	    }
	}
	export class LLMConfig {
	    backend: string;
	
	    static createFrom(source: any = {}) {
	        return new LLMConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backend = source["backend"];
	    }
	}
	export class WorkspaceConfig {
	    name: string;
	    channels: string[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.channels = source["channels"];
	    }
	}
	export class Config {
	    workspaces: WorkspaceConfig[];
	    llm: LLMConfig;
	    vertex_ai: VertexAIConfig;
	    local_llm: LocalLLMConfig;
	    embedding: EmbeddingConfig;
	    polling: PollingConfig;
	    memory: MemoryConfig;
	    response: ResponseConfig;
	    scope_groups: ScopeGroup[];
	    window: WindowConfig;
	    theme: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaces = this.convertValues(source["workspaces"], WorkspaceConfig);
	        this.llm = this.convertValues(source["llm"], LLMConfig);
	        this.vertex_ai = this.convertValues(source["vertex_ai"], VertexAIConfig);
	        this.local_llm = this.convertValues(source["local_llm"], LocalLLMConfig);
	        this.embedding = this.convertValues(source["embedding"], EmbeddingConfig);
	        this.polling = this.convertValues(source["polling"], PollingConfig);
	        this.memory = this.convertValues(source["memory"], MemoryConfig);
	        this.response = this.convertValues(source["response"], ResponseConfig);
	        this.scope_groups = this.convertValues(source["scope_groups"], ScopeGroup);
	        this.window = this.convertValues(source["window"], WindowConfig);
	        this.theme = source["theme"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	
	
	
	
	
	
	

}

export namespace knowledge {
	
	export class Entry {
	    id: string;
	    title: string;
	    content: string;
	    scope: string;
	    workspace_id?: string;
	    tags?: string[];
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.content = source["content"];
	        this.scope = source["scope"];
	        this.workspace_id = source["workspace_id"];
	        this.tags = source["tags"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {
	
	export class ChannelInfo {
	    id: string;
	    name: string;
	    is_private: boolean;
	    num_members: number;
	    topic: string;
	    monitored: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChannelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.is_private = source["is_private"];
	        this.num_members = source["num_members"];
	        this.topic = source["topic"];
	        this.monitored = source["monitored"];
	    }
	}
	export class ChannelStatsInfo {
	    channel_id: string;
	    channel_name: string;
	    msg_count: number;
	    last_ts: string;
	
	    static createFrom(source: any = {}) {
	        return new ChannelStatsInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channel_id = source["channel_id"];
	        this.channel_name = source["channel_name"];
	        this.msg_count = source["msg_count"];
	        this.last_ts = source["last_ts"];
	    }
	}
	export class QueryResult {
	    record_id: string;
	    workspace_id: string;
	    channel_id: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new QueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.record_id = source["record_id"];
	        this.workspace_id = source["workspace_id"];
	        this.channel_id = source["channel_id"];
	        this.score = source["score"];
	    }
	}
	export class WorkspaceStatus {
	    name: string;
	    has_token: boolean;
	    polling: boolean;
	    num_channels: number;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.has_token = source["has_token"];
	        this.polling = source["polling"];
	        this.num_channels = source["num_channels"];
	    }
	}

}

export namespace mitl {
	
	export class Proposal {
	    id: string;
	    workspace_id: string;
	    workspace_name: string;
	    channel_id: string;
	    channel_name: string;
	    thread_ts?: string;
	    trigger_text: string;
	    draft_text: string;
	    state: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    resolved_at?: any;
	
	    static createFrom(source: any = {}) {
	        return new Proposal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspace_id = source["workspace_id"];
	        this.workspace_name = source["workspace_name"];
	        this.channel_id = source["channel_id"];
	        this.channel_name = source["channel_name"];
	        this.thread_ts = source["thread_ts"];
	        this.trigger_text = source["trigger_text"];
	        this.draft_text = source["draft_text"];
	        this.state = source["state"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.resolved_at = this.convertValues(source["resolved_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

