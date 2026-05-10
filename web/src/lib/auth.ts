import { writable } from 'svelte/store';
import { api, type User } from './api';

export const user = writable<User | null>(null);

export async function checkUser() {
	try {
		const u = await api.auth.me();
		user.set(u);
	} catch (e) {
		user.set(null);
	}
}

export async function logout() {
	await api.auth.logout();
	user.set(null);
}
