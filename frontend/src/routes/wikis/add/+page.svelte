<script lang="ts">
	import { wikiStore } from '$lib/stores';
	import LoadingSpinner from '$lib/components/common/LoadingSpinner.svelte';
	import { validateUrl } from '$lib/utils/format';

	let url = '';
	let wikiName = '';
	let loading = false;
	let error = '';

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = '';

		// Validate URL
		const validation = validateUrl(url);
		if (!validation.valid) {
			error = validation.error || 'Invalid URL';
			return;
		}

		loading = true;
		try {
			await wikiStore.create({ url, wiki_name: wikiName || undefined });
			// Redirect to wikis list
			window.location.href = '/wikis';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create wiki';
		} finally {
			loading = false;
		}
	}
</script>

<div class="mx-auto max-w-2xl px-4 sm:px-6 lg:px-8 py-8">
	<div class="mb-8">
		<h1 class="text-3xl font-bold text-gray-900 dark:text-gray-100">Add Wiki</h1>
		<p class="mt-2 text-gray-600 dark:text-gray-400">
			Add a new MediaWiki site to track
		</p>
	</div>

	<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
		<form onsubmit={handleSubmit} class="px-4 py-5 sm:p-6 space-y-6">
			{#if error}
				<div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
					<p class="text-sm text-red-800 dark:text-red-200">{error}</p>
				</div>
			{/if}

			<div>
				<label for="url" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
					Wiki URL <span class="text-red-500">*</span>
				</label>
				<div class="mt-1">
					<input
						id="url"
						type="url"
						bind:value={url}
						required
						placeholder="https://en.wikipedia.org/"
						class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500 dark:bg-gray-700 dark:border-gray-600 dark:text-gray-100"
					/>
					<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
						The base URL of the MediaWiki site (e.g., https://en.wikipedia.org/)
					</p>
				</div>
			</div>

			<div>
				<label for="wikiName" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
					Wiki Name <span class="text-gray-400">(optional)</span>
				</label>
				<div class="mt-1">
					<input
						id="wikiName"
						type="text"
						bind:value={wikiName}
						placeholder="English Wikipedia"
						class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500 dark:bg-gray-700 dark:border-gray-600 dark:text-gray-100"
					/>
					<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
						A custom name for this wiki (optional, will be fetched from siteinfo if not provided)
					</p>
				</div>
			</div>

			<div class="flex gap-3 pt-4">
				<button
					type="submit"
					disabled={loading || !url}
					class="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-primary-600 hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{#if loading}
						<span class="mr-2">
							<span class="w-4 h-4 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
						</span>
						Adding...
					{:else}
						Add Wiki
					{/if}
				</button>
				<a
					href="/wikis"
					class="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600"
				>
					Cancel
				</a>
			</div>
		</form>
	</div>
</div>
