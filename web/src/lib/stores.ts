// 全局响应式信号：SSE 收到事件后增量计数，各面板 $effect 依赖之触发重拉。
// 反射对话多轮 reply 序列存于 reflexConversation；finished 后清。
import { writable } from 'svelte/store';

export const goalVer = writable(0);
export const actionVer = writable(0);
export const reflectionVer = writable(0);
export const episodeVer = writable(0);
export const toolVer = writable(0);
export const interestVer = writable(0);
export const skillVer = writable(0);

export type ReflexReply = {
	id: number;
	round: number;
	content: string;
	channel: string;
	to: string;
	at: number;
};

// 当前在进行的反射对话回复序列；reflex_finished 时不立即清，保留显示直到下一次发送
export const reflexReplies = writable<ReflexReply[]>([]);
export const reflexInProgress = writable(false);

let replySeq = 0;

export function pushReflexReply(round: number, content: string, channel: string, to: string, at: number) {
	replySeq += 1;
	reflexReplies.update((arr) => [
		...arr,
		{ id: replySeq, round, content, channel, to, at }
	]);
}

export function markReflexFinished() {
	reflexInProgress.set(false);
}

export function resetReflexConversation() {
	reflexReplies.set([]);
	reflexInProgress.set(true);
}

// ============ 实况流（统一事件时间线）============
// SSE 各类事件归一为 feed 条目，环形缓冲最近 200 条，LiveFeed 渲染。
export type FeedItem = {
	id: number;
	at: number; // unix 秒
	kind: string; // 事件类型（决定左缘色 + 标签）
	tone: 'glow' | 'cool' | 'violet' | 'warm' | 'dim';
	title: string; // 短标签 key（i18n key，渲染时翻译）
	text: string; // 正文（原始内容，不翻译）
};

export const feed = writable<FeedItem[]>([]);
let feedSeq = 0;
const FEED_MAX = 200;

export function pushFeed(kind: string, tone: FeedItem['tone'], title: string, text: string, at?: number) {
	feedSeq += 1;
	const item: FeedItem = { id: feedSeq, at: at ?? Math.floor(Date.now() / 1000), kind, tone, title, text };
	feed.update((arr) => {
		const next = [item, ...arr];
		return next.length > FEED_MAX ? next.slice(0, FEED_MAX) : next;
	});
}

// ============ 体征历史（sparkline 用）============
// 每次 state 事件推入一帧，环形缓冲最近 90 帧。
export type VitalFrame = {
	energy: number;
	stress: number;
	motivation: number;
	satisfaction: number;
	anxiety: number;
	at: number;
};

export const vitalHistory = writable<VitalFrame[]>([]);
const VITAL_MAX = 90;

export function pushVitalFrame(f: VitalFrame) {
	vitalHistory.update((arr) => {
		const next = [...arr, f];
		return next.length > VITAL_MAX ? next.slice(next.length - VITAL_MAX) : next;
	});
}
