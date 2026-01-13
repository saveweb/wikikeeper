<script lang="ts">
	import { page } from '$app/stores';
	import { extensionsService } from '$lib/services/extensions.service';
	import type { ExtensionWikiInfo, ExtensionVersionStats } from '$lib/types/extensions';
	import LoadingSpinner from '$lib/components/common/LoadingSpinner.svelte';

	// State
	let extensionName = $state('');
	let wikis = $state<ExtensionWikiInfo[]>([]);
	let wikisLoading = $state(false);
	let wikisError = $state('');
	let wikisTotal = $state(0);
	let wikisPage = $state(1);
	let wikisLimit = $state(20);

	let versions = $state<ExtensionVersionStats[]>([]);
	let versionsLoading = $state(false);
	let versionsError = $state('');
	let totalWikis = $state(0);

	// Get extension name from route parameter
	$effect(() => {
		const name = $page.params.name;
		if (name && name !== extensionName) {
			extensionName = decodeURIComponent(name);
			wikisPage = 1;
			loadData();
		}
	});

	async function loadData() {
		if (!extensionName) return;
		await Promise.all([loadWikis(), loadVersions()]);
	}

	async function loadWikis() {
		wikisLoading = true;
		wikisError = '';
		try {
			const response = await extensionsService.getExtensionWikis(
				extensionName,
				wikisPage,
				wikisLimit
			);
			wikis = response.data;
			wikisTotal = response.total;
		} catch (err: any) {
			wikisError = err?.detail || 'Failed to load wikis';
		} finally {
			wikisLoading = false;
		}
	}

	async function loadVersions() {
		versionsLoading = true;
		versionsError = '';
		try {
			const response = await extensionsService.getExtensionVersions(extensionName);
			versions = response.versions;
			totalWikis = response.total_wikis;
		} catch (err: any) {
			versionsError = err?.detail || 'Failed to load version stats';
		} finally {
			versionsLoading = false;
		}
	}

	function nextPage() {
		if (wikisPage * wikisLimit < wikisTotal) {
			wikisPage++;
			loadWikis();
		}
	}

	function prevPage() {
		if (wikisPage > 1) {
			wikisPage--;
			loadWikis();
		}
	}
</script>

<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8">
	{#if extensionName}
		<div class="mb-4">
			<a href="/extensions" class="text-sm text-blue-600 hover:text-blue-700 hover:underline">
				← Back to all extensions
			</a>
		</div>
		<h1 class="text-3xl font-bold text-gray-900 mb-8">
			Extension: {extensionName}
		</h1>

		<!-- Version Distribution -->
		<div class="mb-8 bg-white shadow rounded-lg">
			<div class="px-4 py-5 sm:p-6">
				<h2 class="text-lg font-medium text-gray-900 mb-4">
					Version Distribution
					{#if versionsLoading}
						<span class="ml-2">
							<span class="w-4 h-4 animate-spin rounded-full border-2 border-current border-t-transparent inline-block"></span>
						</span>
					{/if}
				</h2>

				{#if versionsError}
					<div class="bg-red-50 border border-red-200 rounded-md p-4 mb-4">
						<p class="text-sm text-red-700">{versionsError}</p>
					</div>
				{:else if versions.length > 0}
					<div class="mb-4">
						<p class="text-sm text-gray-700">
							Total Wikis: <span class="font-semibold">{totalWikis}</span>
						</p>
					</div>

					<div class="space-y-3">
						{#each versions as stat (stat.version)}
							<div class="flex items-center justify-between p-3 border border-gray-200 rounded-lg">
								<div class="flex items-center gap-4">
									<span class="text-sm font-medium text-gray-900">
										{stat.version || '<null>'}
									</span>
									<span class="text-xs text-gray-500">
										{((stat.count / totalWikis) * 100).toFixed(1)}%
									</span>
								</div>
								<span class="text-sm font-semibold text-blue-600">
									{stat.count} wikis
								</span>
							</div>
						{/each}
					</div>
				{:else}
					<p class="text-sm text-gray-500">No version data available</p>
				{/if}
			</div>
		</div>

		<!-- Wikis Using This Extension -->
		<div class="bg-white shadow rounded-lg">
			<div class="px-4 py-5 sm:p-6">
				<h2 class="text-lg font-medium text-gray-900 mb-4">
					Wikis Using This Extension
					<span class="ml-2 text-sm font-normal text-gray-500">
						({wikisTotal} total)
					</span>
				</h2>

				{#if wikisLoading}
					<div class="flex justify-center items-center h-32">
						<LoadingSpinner size="md" />
					</div>
				{:else if wikisError}
					<div class="bg-red-50 border border-red-200 rounded-md p-4">
						<p class="text-sm text-red-700">{wikisError}</p>
					</div>
				{:else if wikis.length > 0}
					<div class="space-y-3">
						{#each wikis as wiki (wiki.wiki_id)}
							<div class="border border-gray-200 rounded-lg p-4">
								<div class="flex items-start justify-between">
									<div class="flex-1">
										<div class="flex items-center gap-3">
											{#if wiki.sitename || wiki.wiki_name}
												<h3 class="text-sm font-medium text-gray-900">
													{wiki.sitename || wiki.wiki_name}
												</h3>
											{/if}
											{#if wiki.version}
												<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800">
													v{wiki.version}
												</span>
											{:else}
												<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800">
													no version
												</span>
											{/if}
										</div>

										<a
											href={wiki.url}
											target="_blank"
											rel="noopener noreferrer"
											class="text-sm text-blue-600 hover:text-blue-700 hover:underline mt-1 inline-block"
										>
											{wiki.url}
										</a>

										<p class="text-xs text-gray-500 mt-2">
											Snapshot: {new Date(wiki.snapshot_at).toLocaleString()}
										</p>
									</div>

									<a
										href="/wikis/{wiki.wiki_id}"
										class="text-sm text-blue-600 hover:text-blue-700 hover:underline"
									>
										View Details →
									</a>
								</div>
							</div>
						{/each}
					</div>

					<!-- Pagination -->
					{#if wikisTotal > wikisLimit}
						<div class="mt-6 flex items-center justify-between border-t border-gray-200 pt-4">
							<div class="text-sm text-gray-700">
								Showing {(wikisPage - 1) * wikisLimit + 1} to {Math.min(wikisPage * wikisLimit, wikisTotal)} of {wikisTotal}
								wikis
							</div>
							<div class="flex gap-2">
								<button
									onclick={prevPage}
									disabled={wikisPage === 1}
									class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
								>
									Previous
								</button>
								<button
									onclick={nextPage}
									disabled={wikisPage * wikisLimit >= wikisTotal}
									class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
								>
									Next
								</button>
							</div>
						</div>
					{/if}
				{:else}
					<p class="text-sm text-gray-500">No wikis found using this extension</p>
				{/if}
			</div>
		</div>
	{:else}
		<div class="bg-yellow-50 border border-yellow-200 rounded-md p-4">
			<p class="text-sm text-yellow-700">
				Extension name not provided.
			</p>
		</div>
	{/if}
</div>
