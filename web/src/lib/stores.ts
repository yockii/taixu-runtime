// 全局响应式信号：SSE 收到事件后增量计数，各面板 $effect 依赖之触发重拉。
// 也存最近一条 speech 给 InjectForm 即时回响。
import { writable } from 'svelte/store';

export const goalVer = writable(0);
export const actionVer = writable(0);
export const reflectionVer = writable(0);
export const episodeVer = writable(0);
export const toolVer = writable(0);

export type LatestSpeech = {
	id: number;
	content: string;
	goalID: number;
	at: number; // unix sec
};

export const latestSpeech = writable<LatestSpeech | null>(null);

let speechSeq = 0;

export function pushSpeech(content: string, goalID: number) {
	speechSeq += 1;
	latestSpeech.set({ id: speechSeq, content, goalID, at: Math.floor(Date.now() / 1000) });
}
