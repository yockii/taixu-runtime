// SSE 客户端：订阅 /api/stream。
import type { LifeState, MentalState } from './api';

export type StreamEvent =
	| { type: 'state'; life: LifeState; mental: MentalState; reason: string }
	| { type: 'lifecycle'; from_state: string; to_state: string; reason: string }
	| { type: 'tick'; cycle_id: number }
	| { type: 'speech'; content: string; cycle_id: number; goal_id: number };

export function openStream(onEvent: (e: StreamEvent) => void): () => void {
	const es = new EventSource('/api/stream');

	const wrap = (type: StreamEvent['type']) =>
		es.addEventListener(type, (ev: MessageEvent) => {
			try {
				const data = JSON.parse(ev.data);
				onEvent({ type, ...data } as StreamEvent);
			} catch (err) {
				console.warn('SSE parse failed', type, err);
			}
		});
	wrap('state');
	wrap('lifecycle');
	wrap('tick');
	wrap('speech');

	return () => es.close();
}
