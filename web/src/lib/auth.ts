// 访问令牌的响应式状态（写操作鉴权 · R87）。
//
// token   当前本机保存的访问令牌（''=未设）
// authRequired  服务端是否要求令牌（来自 /api/config 的 auth_required）
// locked = authRequired && !token —— 此时交互区块应替换为「输入令牌」占位
//
// 在任一处填入令牌即更新 store，所有受控区块响应式解锁。
import { writable, derived } from 'svelte/store';
import { getToken, setToken } from './api';

export const token = writable<string>(getToken());
export const authRequired = writable<boolean>(false);

/** 写操作是否被锁（需要但还没有令牌）。 */
export const locked = derived([token, authRequired], ([$t, $req]) => $req && !$t);

export function saveToken(value: string): void {
	setToken(value);
	token.set(getToken());
}
