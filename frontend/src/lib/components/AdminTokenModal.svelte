<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';

	export let onClose: () => void;

	let token = '';
	let message = '';

	// API base URL from environment
	const API_BASE = browser
		? (import.meta.env.VITE_API_BASE_URL || 'http://localhost:8000')
		: 'http://localhost:8000';

	// Helper functions to manage admin token cookie
	function getAdminToken(): string | undefined {
		if (!browser) return undefined;
		const cookies = document.cookie.split(';');
		const adminCookie = cookies.find(c => c.trim().startsWith('admintoken='));
		if (adminCookie) {
			return adminCookie.split('=')[1];
		}
		return undefined;
	}

	// Note: We no longer set cookie directly on frontend
	// Instead, we redirect to API domain's callback endpoint
	function clearAdminToken() {
		if (!browser) return;
		document.cookie = 'admintoken=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT';
	}

	onMount(() => {
		// Load existing token from cookie
		const existing = getAdminToken();
		if (existing) {
			token = existing;
		}
	});

	function saveToken() {
		if (token.trim()) {
			// Redirect to API callback endpoint to set cookie
			// The API will set the cookie on its domain and redirect back
			const currentUrl = browser ? window.location.href : '/';
			const callbackUrl = new URL('/api/auth/callback', API_BASE);
			callbackUrl.searchParams.set('token', token.trim());
			callbackUrl.searchParams.set('redirect_to', currentUrl);

			if (browser) {
				window.location.href = callbackUrl.toString();
			}
		} else {
			// Clear token
			clearAdminToken();
			message = 'Admin token cleared';
			setTimeout(() => {
				onClose();
			}, 500);
		}
	}

	function clearToken() {
		token = '';
		clearAdminToken();
		message = 'Admin token cleared';
		setTimeout(() => {
			message = '';
		}, 1000);
	}

	function handleBackdropClick(event: MouseEvent) {
		if (event.target === event.currentTarget) {
			onClose();
		}
	}

	function handleBackdropKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			onClose();
		}
	}
</script>

<div
	class="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full flex items-center justify-center z-50"
	onclick={handleBackdropClick}
	onkeydown={handleBackdropKeydown}
	role="presentation"
>
	<div class="relative bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
		<div class="flex justify-between items-center mb-4">
			<h3 class="text-lg font-medium text-gray-900">
				Admin Token Settings
			</h3>
			<button
				onclick={onClose}
				class="text-gray-400 hover:text-gray-500 focus:outline-none"
				aria-label="Close"
			>
				<svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M6 18L18 6M6 6l12 12"
					/>
				</svg>
			</button>
		</div>

		<div class="space-y-4">
			<div>
				<label for="admintoken" class="block text-sm font-medium text-gray-700 mb-2">
					Admin Token
				</label>
				<input
					id="admintoken"
					type="password"
					bind:value={token}
					placeholder="Enter your admin token"
					class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
					onkeydown={(e) => e.key === 'Enter' && saveToken()}
				/>
				<p class="mt-2 text-sm text-gray-500">
					With admin token, you can:
				</p>
				<ul class="list-disc list-inside mt-1 ml-4 text-sm text-gray-500">
					<li>Delete wikis</li>
					<li>Trigger unlimited checks (bypass rate limits)</li>
					<li>Run bulk collection operations</li>
				</ul>
			</div>

			{#if message}
				<div class="bg-blue-50 border border-blue-200 rounded-md p-3">
					<p class="text-sm text-blue-700">{message}</p>
				</div>
			{/if}

			<div class="flex justify-end space-x-3 pt-4 border-t border-gray-200">
				<button
					onclick={clearToken}
					class="px-4 py-2 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none"
				>
					Clear
				</button>
				<button
					onclick={saveToken}
					class="px-4 py-2 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-primary-600 hover:bg-primary-700 focus:outline-none"
				>
					Save
				</button>
			</div>
		</div>
	</div>
</div>
