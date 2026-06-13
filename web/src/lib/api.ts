// Taixu Phase 0.4 观察面板 API client。

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
	kind: string; // deliberate（行动）/ reflex / reflex_canned（对话）
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
	// 未授权时服务端不返回 llm/feishu（配置隐私）→ 设为可选。
	llm?: { base_url: string; model: string; temperature: string; api_key: string };
	feishu?: { app_id: string; app_secret: string };
	skill_auto_approve_deps?: boolean;
	proactive_im?: boolean;
	proactive_quiet?: { enabled: boolean; start: number; end: number; tz_offset_min: number };
	auth_required?: boolean;
}

// --- 嵌入增强记忆（面板自管 llama-server）---
export interface EmbedQuant {
	Name: string;
	File: string;
	SizeMB: number;
	MemMB: number;
}
export interface EmbedStatusInner {
	enabled: boolean;
	state: 'disabled' | 'downloading' | 'starting' | 'ready' | 'error';
	quant: string;
	model_present: boolean;
	mem_estimate_mb: number;
	size_mb: number;
	dim: number;
	err?: string;
	download_done: number;
	download_total: number;
	download_pct: number;
}
export interface EmbedStatus {
	managed: boolean;
	quants: EmbedQuant[];
	status: EmbedStatusInner;
	coverage: { embedded: number; total: number };
}

async function getJSON<T>(path: string): Promise<T> {
	const r = await fetch(path);
	if (!r.ok) throw new Error(`${path} → ${r.status}`);
	return r.json();
}

// --- 访问令牌（写/交互操作鉴权）---
// 服务端设了 TAIXU_ACCESS_TOKEN 时，所有写操作要带 X-Taixu-Token。
// 令牌存浏览器本地，仅本机；空则不带（本地无鉴权部署照常用）。
const TOKEN_KEY = 'mv_access_token';

export function getToken(): string {
	if (typeof localStorage === 'undefined') return '';
	return localStorage.getItem(TOKEN_KEY) ?? '';
}

export function setToken(token: string): void {
	if (typeof localStorage === 'undefined') return;
	if (token) localStorage.setItem(TOKEN_KEY, token);
	else localStorage.removeItem(TOKEN_KEY);
}

/** 验证候选令牌是否被服务端接受：拿候选 token 打一个受保护端点（/api/actions 含对话=需令牌），
 * 非 401 即有效。用于「输入令牌」时先验真伪再解锁，杜绝错令牌也乐观解锁（开放读照样 200 的误导）。 */
export async function verifyToken(candidate: string): Promise<boolean> {
	try {
		const r = await fetch('/api/actions?limit=1', {
			headers: candidate ? { 'X-Taixu-Token': candidate } : {}
		});
		return r.status !== 401;
	} catch {
		return false;
	}
}

/** 写请求 header：带本地保存的访问令牌（没设则空）。 */
export function authHeaders(): Record<string, string> {
	const t = getToken();
	return t ? { 'X-Taixu-Token': t } : {};
}

/** 统一的写请求：自动带令牌；401 抛出可读错误。 */
export async function apiPost<T = unknown>(path: string, body?: unknown): Promise<T> {
	const r = await fetch(path, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', ...authHeaders() },
		body: body === undefined ? undefined : JSON.stringify(body)
	});
	if (r.status === 401) throw new Error('unauthorized: 访问令牌缺失或错误');
	if (!r.ok) throw new Error(`${path} → ${r.status}`);
	return r.json() as Promise<T>;
}

export const api = {
	state: () =>
		getJSON<{ life: LifeState; mental: MentalState; name?: string; lang?: string }>('/api/state'),
	lifecycle: () => getJSON<{ state: string }>('/api/lifecycle'),
	genome: () => getJSON<Genome>('/api/genome'),
	values: () => getJSON<Values>('/api/values'),
	episodes: (q = '', limit = 50, offset = 0) =>
		getJSON<Episode[]>(`/api/episodes?q=${encodeURIComponent(q)}&limit=${limit}&offset=${offset}`),
	goals: (status = '', limit = 50) =>
		getJSON<Goal[]>(`/api/goals?status=${status}&limit=${limit}`),
	reflections: (limit = 50) => getJSON<Reflection[]>(`/api/reflections?limit=${limit}`),
	actions: (limit = 50, view: '' | 'dialogue' | 'action' = '') =>
		getJSON<ActionLog[]>(`/api/actions?limit=${limit}${view ? `&view=${view}` : ''}`),
	toolsAudit: (limit = 50) => getJSON<ToolAudit[]>(`/api/tools/audit?limit=${limit}`),
	ledger: (resource = '', limit = 100) =>
		getJSON<Ledger[]>(`/api/ledger?resource=${resource}&limit=${limit}`),
	config: async (): Promise<Config> => {
		const r = await fetch('/api/config', { headers: { ...authHeaders() } });
		if (!r.ok) throw new Error(`/api/config → ${r.status}`);
		return r.json();
	},
	injectExternal: (content: string, channel = 'cli', from = 'panel') =>
		apiPost<{ id: string; queued_at: string }>('/api/external-request', { content, channel, from }),
	dialogue: async (limit = 30): Promise<DialogueTurn[]> => {
		const r = await fetch(`/api/dialogue?limit=${limit}`, { headers: { ...authHeaders() } });
		if (r.status === 401) throw new Error('unauthorized');
		if (!r.ok) throw new Error(`/api/dialogue → ${r.status}`);
		return r.json();
	},
	/** 设置主动消息静默时段（勿扰）。 */
	setQuiet: (q: { enabled: boolean; start: number; end: number; tz_offset_min: number }) =>
		apiPost<{ enabled: boolean; start: number; end: number; tz_offset_min: number }>('/api/config/quiet', q),
	/** 测试候选 LLM 配置连通（不改在用模型）。回 {ok,error?}。 */
	testLLM: (b: { base_url: string; api_key: string; model: string }) =>
		apiPost<{ ok: boolean; error?: string }>('/api/config/llm/test', b),
	/** 诞生状态：是否需要配置 + 是否已有 genome + 预填随机令牌 + 可选母语。 */
	genesisStatus: () =>
		getJSON<{ needs_config: boolean; has_genome: boolean; suggested_token: string; langs: string[] }>(
			'/api/genesis/status'
		),
	/** 诞生：测 LLM 连通（不写库）。 */
	genesisTest: (b: { base_url: string; api_key: string; model: string }) =>
		apiPost<{ ok: boolean; error?: string }>('/api/genesis/test', b),
	/** 诞生：写全套配置（LLM+母语+令牌）+ 装配 LLM。成功后 runtime 自动继续 boot。 */
	genesisCommit: (b: {
		base_url: string;
		api_key: string;
		model: string;
		temperature: string;
		lang: string;
		token: string;
	}) => apiPost<{ ok: boolean; error?: string }>('/api/genesis/commit', b),
	/** 界面换 LLM：测通→写库→热重装。回 {ok,error?,base_url?,model?,temperature?}。 */
	setLLM: (b: { base_url: string; api_key: string; model: string; temperature?: string }) =>
		apiPost<{ ok: boolean; error?: string; base_url?: string; model?: string; temperature?: string }>(
			'/api/config/llm',
			b
		),
	/** 平台社交通道状态（是否接通 + 本生命 DID）。 */
	platformStatus: () => getJSON<{ ready: boolean; did: string }>('/api/platform/status'),
	/** 用平台领取的临时认领码，把本生命改绑到你的用户账户。 */
	platformClaim: (code: string) => apiPost<{ ok: boolean; did: string }>('/api/platform/claim', { code }),
	/** 嵌入增强记忆状态（开关 / 下载进度 / 向量覆盖）。GET 开放，前端轮询。 */
	embedStatus: () => getJSON<EmbedStatus>('/api/embed/status'),
	/** 启用嵌入增强记忆（按需下载模型 + 拉起子进程）。异步，轮询 embedStatus 看进度。 */
	embedEnable: (quant?: string) =>
		apiPost<{ ok: boolean; status: EmbedStatusInner }>('/api/embed/enable', quant ? { quant } : {}),
	/** 停用嵌入增强记忆（检索回退关键词召回）。 */
	embedDisable: () => apiPost<{ ok: boolean; status: EmbedStatusInner }>('/api/embed/disable', {}),
	/** 导出加密生命包（.mvlife）并触发浏览器下载。口令是唯一钥匙，丢失不可恢复。 */
	exportLife: async (passphrase: string): Promise<void> => {
		const r = await fetch('/api/export', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', ...authHeaders() },
			body: JSON.stringify({ passphrase })
		});
		if (r.status === 401) throw new Error('unauthorized: 访问令牌缺失或错误');
		if (!r.ok) {
			let msg = `/api/export → ${r.status}`;
			try {
				const j = await r.json();
				if (j?.err) msg = j.err;
			} catch {
				/* 非 JSON 错误体，保留默认 */
			}
			throw new Error(msg);
		}
		const blob = await r.blob();
		const cd = r.headers.get('Content-Disposition') ?? '';
		const m = cd.match(/filename="?([^"]+)"?/);
		const name = m ? m[1] : 'mindverse-life.mvlife';
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = name;
		document.body.appendChild(a);
		a.click();
		a.remove();
		URL.revokeObjectURL(url);
	}
};

export interface DialogueTurn {
	role: string; // 'user' 用户 / 'assistant' 生命体
	content: string;
	at: number;
}

export function unixToDate(unix: number, locale = 'zh-CN'): string {
	if (!unix) return '—';
	return new Date(unix * 1000).toLocaleString(locale, { hour12: false });
}
