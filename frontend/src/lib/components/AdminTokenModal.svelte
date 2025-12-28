<script lang="ts">
	import { onMount } from 'svelte';

	export let onClose: () => void;

	let token = '';
	let message = '';

	onMount(() => {
		// Load existing token
		const existing = localStorage.getItem('admintoken');
		if (existing) {
			token = existing;
		}
	});

	function saveToken() {
		if (token.trim()) {
			localStorage.setItem('admintoken', token.trim());
			message = 'Admin token saved!';
			setTimeout(() => {
				onClose();
			}, 500);
		} else {
			// Clear token
			localStorage.removeItem('admintoken');
			message = 'Admin token cleared';
			setTimeout(() => {
				onClose();
			}, 500);
		}
	}

	function clearToken() {
		token = '';
		localStorage.removeItem('admintoken');
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
	<div class="relative bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
		<div class="flex justify-between items-center mb-4">
			<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">
				Admin Token Settings
			</h3>
			<button
				onclick={onClose}
				class="text-gray-400 hover:text-gray-500 dark:hover:text-gray-300 focus:outline-none"
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
				<label for="admintoken" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
					Admin Token
				</label>
				<input
					id="admintoken"
					type="password"
					bind:value={token}
					placeholder="Enter your admin token"
					class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary-500 dark:bg-gray-700 dark:text-white"
					onkeydown={(e) => e.key === 'Enter' && saveToken()}
				/>
				<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
					With admin token, you can:
				</p>
				<ul class="list-disc list-inside mt-1 ml-4 text-sm text-gray-500 dark:text-gray-400">
					<li>Delete wikis</li>
					<li>Trigger unlimited checks (bypass rate limits)</li>
					<li>Run bulk collection operations</li>
				</ul>
			</div>

			{#if message}
				<div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-md p-3">
					<p class="text-sm text-blue-700 dark:text-blue-300">{message}</p>
				</div>
			{/if}

			<div class="flex justify-end space-x-3 pt-4 border-t border-gray-200 dark:border-gray-700">
				<button
					onclick={clearToken}
					class="px-4 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm font-medium rounded-md text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none"
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
