// 极简 i18n：localStorage 持久 + key 翻译表。
import { writable, derived, type Readable } from 'svelte/store';
import { browser } from '$app/environment';

export type Lang = 'zh' | 'en';

const STORAGE_KEY = 'mindverse.lang';

function initialLang(): Lang {
	if (!browser) return 'zh';
	const saved = localStorage.getItem(STORAGE_KEY);
	if (saved === 'zh' || saved === 'en') return saved;
	const sys = navigator.language?.toLowerCase() ?? 'zh';
	return sys.startsWith('zh') ? 'zh' : 'en';
}

export const lang = writable<Lang>(initialLang());

if (browser) {
	lang.subscribe((v) => localStorage.setItem(STORAGE_KEY, v));
}

const dict: Record<Lang, Record<string, string>> = {
	zh: {
		title: 'Mindverse · 心域文明',
		cycle: 'cycle',
		state_title: '生命状态 / 心境',
		genome_title: '基因（出生即定）',
		values_title: '价值观（Phase 0 只读）',
		episodes_title: 'Episode 流（最近 30 段）',
		goals_title: '目标队列（近 20 条）',
		reflections_title: '反思（近 30 条）',
		actions_title: '行动日志（近 20 条）',
		tools_title: '工具审计（近 30 条）',
		config_title: '配置（运行时）',
		inject_title: '注入外部请求（cli channel）',
		inject_placeholder: '说点什么...',
		inject_send: '发送',
		inject_busy: '入队中...',
		inject_last: '上次入队',
		reply_label: '生命体回应',
		reply_waiting: '生命体正在思考...',
		interests_title: '兴趣种子',
		empty_interest: '尚未识别兴趣',
		ikind_skill: '技能',
		ikind_knowledge: '知识',
		ikind_topic: '话题',
		ikind_experience: '体验',
		explored_n: '探索',
		loading: '加载中...',
		empty_episode: '无段',
		empty_goal: '无目标',
		empty_reflection: '尚无反思',
		empty_action: '尚无行动',
		empty_tool: '尚无工具调用',
		search_placeholder: '搜索 summary...',
		life_id: '生命ID',
		born_at: '出生',
		cap_label: 'EnergyDailyCap',
		cap_used: '已用',
		cap_reset_next: '下次重置',
		// 状态字段
		energy: '能量',
		competence: '能力',
		social_need: '社交需求',
		stress: '压力',
		confidence: '自信',
		stability: '稳定',
		motivation: '动机',
		satisfaction: '满意',
		anxiety: '焦虑',
		curiosity: '好奇心',
		sociability: '社交性',
		creativity: '创造力',
		persistence: '坚韧',
		risk_taking: '冒险',
		empathy: '共情',
		// values keys
		val_growth: '成长',
		val_friendship: '友谊',
		val_creativity: '创造',
		val_safety: '安全',
		val_exploration: '探索',
		val_honesty: '诚实',
		// goal source / intent
		intent_respond_to_user: '回应用户',
		intent_knowledge: '求知',
		intent_social: '社交',
		intent_creativity: '创作',
		intent_stability: '寻求稳定',
		intent_achievement: '达成成就',
		src_ExternalRequest: '外部请求',
		src_IntrinsicDrive: '内驱',
		src_ReflectionGoal: '反思',
		llm_section: 'LLM',
		feishu_section: '飞书',
		// goal status
		status_pending: '待处理',
		status_active: '进行中',
		status_completed: '已完成',
		status_rejected: '已拒绝',
		status_expired: '已过期',
		status_failed: '失败',
		// lifecycle
		state_Unknown: '未知',
		state_Embryonic: '胚胎',
		state_Active: '活跃',
		state_LowPower: '低能',
		state_Dormant: '休眠',
		state_Archived: '归档',
		state_Detached: '脱离',
		state_Memorial: '纪念'
	},
	en: {
		title: 'Mindverse · Digital Life',
		cycle: 'cycle',
		state_title: 'Life State / Mental',
		genome_title: 'Genome (immutable at birth)',
		values_title: 'Values (read-only in Phase 0)',
		episodes_title: 'Episode Stream (latest 30)',
		goals_title: 'Goal Queue (latest 20)',
		reflections_title: 'Reflections (latest 30)',
		actions_title: 'Action Log (latest 20)',
		tools_title: 'Tool Audit (latest 30)',
		config_title: 'Runtime Config',
		inject_title: 'Inject External Request (cli)',
		inject_placeholder: 'Say something...',
		inject_send: 'Send',
		inject_busy: 'Queueing...',
		inject_last: 'Last queued',
		reply_label: 'Life replied',
		reply_waiting: 'Life is thinking...',
		interests_title: 'Interest Seeds',
		empty_interest: 'No interests yet',
		ikind_skill: 'skill',
		ikind_knowledge: 'knowledge',
		ikind_topic: 'topic',
		ikind_experience: 'experience',
		explored_n: 'explored',
		loading: 'Loading...',
		empty_episode: 'No episodes yet',
		empty_goal: 'No goals',
		empty_reflection: 'No reflections yet',
		empty_action: 'No actions yet',
		empty_tool: 'No tool calls yet',
		search_placeholder: 'Search summary...',
		life_id: 'LifeID',
		born_at: 'Born',
		cap_label: 'EnergyDailyCap',
		cap_used: 'Used',
		cap_reset_next: 'Next reset',
		energy: 'Energy',
		competence: 'Competence',
		social_need: 'SocialNeed',
		stress: 'Stress',
		confidence: 'Confidence',
		stability: 'Stability',
		motivation: 'Motivation',
		satisfaction: 'Satisfaction',
		anxiety: 'Anxiety',
		curiosity: 'Curiosity',
		sociability: 'Sociability',
		creativity: 'Creativity',
		persistence: 'Persistence',
		risk_taking: 'RiskTaking',
		empathy: 'Empathy',
		llm_section: 'LLM',
		feishu_section: 'Feishu',
		status_pending: 'pending',
		status_active: 'active',
		status_completed: 'completed',
		status_rejected: 'rejected',
		status_expired: 'expired',
		status_failed: 'failed',
		val_growth: 'growth',
		val_friendship: 'friendship',
		val_creativity: 'creativity',
		val_safety: 'safety',
		val_exploration: 'exploration',
		val_honesty: 'honesty',
		intent_respond_to_user: 'respond_to_user',
		intent_knowledge: 'knowledge',
		intent_social: 'social',
		intent_creativity: 'creativity',
		intent_stability: 'stability',
		intent_achievement: 'achievement',
		src_ExternalRequest: 'ExternalRequest',
		src_IntrinsicDrive: 'IntrinsicDrive',
		src_ReflectionGoal: 'ReflectionGoal',
		state_Unknown: 'Unknown',
		state_Embryonic: 'Embryonic',
		state_Active: 'Active',
		state_LowPower: 'LowPower',
		state_Dormant: 'Dormant',
		state_Archived: 'Archived',
		state_Detached: 'Detached',
		state_Memorial: 'Memorial'
	}
};

export const t: Readable<(key: string) => string> = derived(lang, ($lang) => {
	return (key: string) => dict[$lang][key] ?? key;
});

export function toggleLang() {
	lang.update((l) => (l === 'zh' ? 'en' : 'zh'));
}
