// Mindverse Phase 0.4 观察面板 API client。

export interface Genome {
	life_id: string;
	curiosity: number;
	sociability: number;
	creativity: number;
	persistence: number;
	risk_taking: number;
	empathy: number;
	born_at: number;
	genome_version: string;
}

export interface LifeState {
	life_id: string;
	energy: number;
	competence: number;
	social_need: number;
	stress: number;
	confidence: number;
	stability: number;
	energy_daily_cap: number;
	energy_used_today: number;
	cap_reset_at: number;
	updated_at: number;
}

export interface MentalState {
	life_id: string;
	motivation: number;
	satisfaction: number;
	anxiety: number;
	updated_at: number;
}

export interface Values {
	life_id: string;
	weights: Record<string, number>;
	updated_at: number;
}

export interface Episode {
	id: number;
	title?: string;
	summary: string;
	started_at: number;
	ended_at: number;
	raw_start_id?: number;
	raw_end_id?: number;
	salience: number;
	emotion_score?: number;
	created_at: number;
	sealed_at?: number;
}

export interface Goal {
	id: number;
	source: string;
	intent: string;
	payload: string;
	priority: number;
	status: string;
	created_at: number;
	started_at?: number;
	finished_at?: number;
	arbitration_note?: string;
}

export interface Reflection {
	id: number;
	kind: string;
	summary: string;
	insight?: string;
	triggered_by?: string;
	created_at: number;
}

export interface ActionLog {
	id: number;
	goal_id: number;
	cycle_id: number;
	plan: string;
	action: string;
	result: string;
	feedback: string;
	success: boolean;
	started_at: number;
	finished_at: number;
}

export interface ToolAudit {
	id: number;
	cycle_id: number;
	tool_name: string;
	args_summary: string;
	result_summary: string;
	duration_ms: number;
	success: boolean;
	error?: string;
	started_at: number;
}

export interface Ledger {
	id: number;
	resource: string;
	delta: number;
	balance_after: number;
	reason: string;
	source_kind: string;
	source_ref: string;
	created_at: number;
}

export interface Config {
	llm: { base_url: string; model: string; temperature: string; api_key: string };
	feishu: { app_id: string; app_secret: string };
}

async function getJSON<T>(path: string): Promise<T> {
	const r = await fetch(path);
	if (!r.ok) throw new Error(`${path} → ${r.status}`);
	return r.json();
}

export const api = {
	state: () => getJSON<{ life: LifeState; mental: MentalState }>('/api/state'),
	lifecycle: () => getJSON<{ state: string }>('/api/lifecycle'),
	genome: () => getJSON<Genome>('/api/genome'),
	values: () => getJSON<Values>('/api/values'),
	episodes: (q = '', limit = 50, offset = 0) =>
		getJSON<Episode[]>(`/api/episodes?q=${encodeURIComponent(q)}&limit=${limit}&offset=${offset}`),
	goals: (status = '', limit = 50) =>
		getJSON<Goal[]>(`/api/goals?status=${status}&limit=${limit}`),
	reflections: (limit = 50) => getJSON<Reflection[]>(`/api/reflections?limit=${limit}`),
	actions: (limit = 50) => getJSON<ActionLog[]>(`/api/actions?limit=${limit}`),
	toolsAudit: (limit = 50) => getJSON<ToolAudit[]>(`/api/tools/audit?limit=${limit}`),
	ledger: (resource = '', limit = 100) =>
		getJSON<Ledger[]>(`/api/ledger?resource=${resource}&limit=${limit}`),
	config: () => getJSON<Config>('/api/config'),
	injectExternal: async (content: string, channel = 'cli', from = 'panel') => {
		const r = await fetch('/api/external-request', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ content, channel, from })
		});
		if (!r.ok) throw new Error(`inject → ${r.status}`);
		return r.json() as Promise<{ id: string; queued_at: string }>;
	}
};

export function unixToDate(unix: number, locale = 'zh-CN'): string {
	if (!unix) return '—';
	return new Date(unix * 1000).toLocaleString(locale, { hour12: false });
}
