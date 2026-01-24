import { api, auth, type User } from '$lib/api/client';
import { goto } from '$app/navigation';

function createAuthStore() {
	let user = $state<User | null>(null);
	let isLoading = $state(true);

	function handleAuthFailure() {
		user = null;
		api.setToken(null);
		api.setRefreshToken(null);
		goto('/login');
	}

	async function initialize() {
		api.onAuthenticationFailure(handleAuthFailure);

		const token = api.getToken();
		if (!token) {
			isLoading = false;
			return;
		}

		const result = await auth.me();
		if (result.data) {
			user = result.data;
		} else {
			api.setToken(null);
		}
		isLoading = false;
	}

	async function login(email: string, password: string) {
		const result = await auth.login(email, password);
		if (result.error) {
			throw new Error(result.error.message);
		}

		if (result.data?.tokens) {
			api.setToken(result.data.tokens.access_token);
			api.setRefreshToken(result.data.tokens.refresh_token);

			const meResult = await auth.me();
			if (meResult.data) {
				user = meResult.data;
			}
		}
	}

	async function logout() {
		await auth.logout();
		api.setToken(null);
		api.setRefreshToken(null);
		user = null;
	}

	return {
		get user() {
			return user;
		},
		get isLoading() {
			return isLoading;
		},
		get isAuthenticated() {
			return !!user;
		},
		initialize,
		login,
		logout
	};
}

export const authStore = createAuthStore();
