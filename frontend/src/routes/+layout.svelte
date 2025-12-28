<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import AdminTokenModal from '$lib/components/AdminTokenModal.svelte';

	let showAdminModal = false;
	let hasAdminToken = false;

	// API base URL from environment
	const API_BASE = browser
		? (import.meta.env.VITE_API_BASE_URL || 'http://localhost:8000')
		: 'http://localhost:8000';

	// Check admin authentication status via API
	async function checkAdminToken(): Promise<boolean> {
		if (!browser) return false;

		try {
			const response = await fetch(`${API_BASE}/api/auth/check`, {
				credentials: 'include'
			});
			const data = await response.json();
			return data.authenticated === true;
		} catch (error) {
			console.error('Failed to check admin status:', error);
			return false;
		}
	}

	onMount(async () => {
		// Check if admin token is set
		hasAdminToken = await checkAdminToken();
	});

	function openAdminModal() {
		showAdminModal = true;
	}

	async function closeAdminModal() {
		showAdminModal = false;
		hasAdminToken = await checkAdminToken();
	}
</script>

<svelte:head>
	<title>WikiKeeper</title>
	<meta name="description" content="Wiki statistics tracker and archive status checker" />
</svelte:head>

<div class="min-h-screen bg-gray-50">
	<nav class="bg-white shadow-sm dark:bg-gray-800">
		<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
			<div class="flex justify-between h-16">
				<div class="flex">
					<div class="flex-shrink-0 flex items-center">
						<a href="/" class="text-xl font-bold text-primary-600">
							WikiKeeper
						</a>
					</div>
					<div class="hidden sm:ml-6 sm:flex sm:space-x-8">
						<a
							href="/"
							class="border-transparent text-gray-900 dark:text-gray-100 inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium"
						>
							Dashboard
						</a>
						<a
							href="/wikis"
							class="border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium"
						>
							Wikis
						</a>
					</div>
				</div>
				<div class="flex items-center">
					<button
						onclick={openAdminModal}
						class="inline-flex items-center px-3 py-2 border border-gray-300 shadow-sm text-sm leading-4 font-medium rounded-md text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary-500"
						title="Admin Settings"
					>
						{#if hasAdminToken}
							<span class="text-green-600 dark:text-green-400 mr-2">●</span>
							Admin
						{:else}
							Admin
						{/if}
					</button>
				</div>
			</div>
		</div>
	</nav>

	{#if showAdminModal}
		<AdminTokenModal onClose={closeAdminModal} />
	{/if}

	<main>
		<slot />
	</main>

	<footer class="bg-white dark:bg-gray-800 mt-auto">
		<div class="mx-auto max-w-7xl py-6 px-4 sm:px-6 lg:px-8">
			<p class="text-center text-sm text-gray-500 dark:text-gray-400">
				WikiKeeper. Tracking MediaWiki sites worldwide.
			</p>
		</div>
	</footer>
</div>
