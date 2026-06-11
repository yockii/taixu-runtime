// SSE 客户端：订阅 /api/stream。
//
// 鉴权（R87）：EventSource 不能带自定义 header，令牌经 ?token= 查询参数传递。
// 服务端令牌已配置而连接未带有效 token 时，仍可连接但收不到含正文的隐私事件
// （reflex_reply / reflection / episode_sealed）。token 在 openStream 调用时一次性取出，
// TokenGate 解锁后需重建连接才生效——解锁成功路径会 location.reload()（见 TokenGate.svelte）。
import { getToken } from './api';
import type { LifeState, MentalState } from './api';

export type StreamEvent =
	| { type: 'state'; life: LifeState; mental: MentalState; reason: string }
	| { type: 'lifecycle'; from_state: string; to_state: string; reason: string }
	| { type: 'tick'; cycle_id: number }
	| { type: 'reflex_reply'; round: number; channel: string; to: string; content: string; created_at: number }
	| { type: 'reflex_finished'; channel: string; to: string; rounds: number; created_at: number }
	| { type: 'episode_sealed'; episode_id: number; summary: string; events: number; started_at: number; ended_at: number }
	| { type: 'reflection'; reflection_id: number; kind: string; promoted: number; summary: string }
	| { type: 'goal_enqueued'; goal_id: number; source: string; intent: string; priority: number; payload: string }
	| { type: 'action_done'; cycle_id: number; goal_id: number; action: string; success: boolean; started_at: number }
	| { type: 'tool_audited'; tool_name: string; success: boolean; duration_ms: number };

const EVENT_TYPES: StreamEvent['type'][] = [
	'state',
	'lifecycle',
	'tick',
	'reflex_reply',
	'reflex_finished',
	'episode_sealed',
	'reflection',
	'goal_enqueued',
	'action_done',
	'tool_audited'
];

export function openStream(onEvent: (e: StreamEvent) => void): () => void {
	const token = getToken();
	const es = new EventSource('/api/stream' + (token ? '?token=' + encodeURIComponent(token) : ''));
	for (const type of EVENT_TYPES) {
		es.addEventListener(type, (ev: MessageEvent) => {
			try {
				const data = JSON.parse(ev.data);
				onEvent({ type, ...data } as StreamEvent);
			} catch (err) {
				console.warn('SSE parse failed', type, err);
			}
		});
	}
	return () => es.close();
}
