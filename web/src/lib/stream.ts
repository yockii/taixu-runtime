// SSE 客户端：订阅 /api/stream。
import type { LifeState, MentalState } from './api';

export type StreamEvent =
	| { type: 'state'; life: LifeState; mental: MentalState; reason: string }
	| { type: 'lifecycle'; from_state: string; to_state: string; reason: string }
	| { type: 'tick'; cycle_id: number }
	| { type: 'speech'; content: string; cycle_id: number; goal_id: number }
	| { type: 'episode_sealed'; episode_id: number; summary: string; events: number; started_at: number; ended_at: number }
	| { type: 'reflection'; reflection_id: number; kind: string; promoted: number; summary: string }
	| { type: 'goal_enqueued'; goal_id: number; source: string; intent: string; priority: number; payload: string }
	| { type: 'action_done'; cycle_id: number; goal_id: number; action: string; success: boolean; started_at: number }
	| { type: 'tool_audited'; tool_name: string; success: boolean; duration_ms: number };

const EVENT_TYPES: StreamEvent['type'][] = [
	'state',
	'lifecycle',
	'tick',
	'speech',
	'episode_sealed',
	'reflection',
	'goal_enqueued',
	'action_done',
	'tool_audited'
];

export function openStream(onEvent: (e: StreamEvent) => void): () => void {
	const es = new EventSource('/api/stream');
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
