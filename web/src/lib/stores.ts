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
