<script lang="ts">
	import StatusBadge from '$lib/components/wiki/StatusBadge.svelte';
	import { APP_CONFIG } from '$lib/constants';
	import type { Wiki, WikiStatus } from '$lib/types';

	interface Props {
		wikis: Wiki[];
		loading?: boolean;
		error?: string;
		showFilters?: boolean;
		limit?: number;
	}

	let { wikis, loading = false, error = '', showFilters = true, limit }: Props = $props();

	// Filter and sort state
	let search = $state('');
	let statusFilter = $state<WikiStatus | ''>('');
	let hasArchiveFilter = $state<string>('');
	let sortBy = $state<'updated_at' | 'created_at' | 'sitename'>('updated_at');
	let sortOrder = $state<'desc' | 'asc'>('desc');

	// Filter and sort wikis
	const filteredAndSortedWikis = $derived.by(() => {
		let result = [...(wikis || [])];

		// Apply search filter
		if (search) {
			const searchLower = search.toLowerCase();
			result = result.filter(
				(wiki) =>
					wiki.url.toLowerCase().includes(searchLower) ||
					(wiki.sitename && wiki.sitename.toLowerCase().includes(searchLower))
			);
		}

		// Apply status filter
		if (statusFilter) {
			result = result.filter((wiki) => wiki.status === statusFilter);
		}

		// Apply archive filter
		if (hasArchiveFilter === 'true') {
			result = result.filter((wiki) => wiki.has_archive);
		} else if (hasArchiveFilter === 'false') {
			result = result.filter((wiki) => !wiki.has_archive);
		}

		// Apply sort
		result.sort((a, b) => {
			let aVal: string | number;
			let bVal: string | number;

			if (sortBy === 'sitename') {
				aVal = a.sitename || a.url;
				bVal = b.sitename || b.url;
			} else {
				aVal = new Date(a[sortBy]).getTime();
				bVal = new Date(b[sortBy]).getTime();
			}

			if (sortOrder === 'asc') {
				return aVal > bVal ? 1 : -1;
			} else {
				return aVal < bVal ? 1 : -1;
			}
		});

		// Apply limit
		if (limit) {
			result = result.slice(0, limit);
		}

		return result;
	});

	const displayedWikis = $derived(filteredAndSortedWikis);

	function resetFilters() {
		search = '';
		statusFilter = '';
		hasArchiveFilter = '';
		sortBy = 'updated_at';
		sortOrder = 'desc';
	}
</script>

{#if loading}
	<div class="flex justify-center items-center h-64">
		<div class="w-8 h-8 animate-spin rounded-full border-2 border-current border-t-transparent"></div>
	</div>
{:else if error}
	<div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
		<p class="text-sm text-red-800 dark:text-red-200">{error}</p>
	</div>
{:else if displayedWikis.length === 0}
	<div class="text-center py-12">
		<p class="text-gray-500 dark:text-gray-400 text-lg mb-4">
			No wikis found matching your filters.
		</p>
		<a
			href="/wikis/add"
			class="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-primary-600 hover:bg-primary-700"
		>
			Add A Wiki
		</a>
	</div>
{:else}
	<div>
		{#if showFilters}
			<!-- Filters -->
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg mb-6">
				<div class="px-4 py-5 sm:p-6">
					<div class="grid grid-cols-1 gap-4 sm:grid-cols-4">
						<div>
							<label for="search" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
								Search
							</label>
							<input
								id="search"
								type="text"
								bind:value={search}
								placeholder="Search by URL or name..."
								class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500 dark:bg-gray-700 dark:border-gray-600 dark:text-gray-100"
							/>
						</div>
						<div>
							<label for="status" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
								Status
							</label>
							<select
								id="status"
								bind:value={statusFilter}
								class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500 dark:bg-gray-700 dark:border-gray-600 dark:text-gray-100"
							>
								<option value="">All Statuses</option>
								<option value="pending">Pending</option>
								<option value="ok">OK</option>
								<option value="error">Error</option>
								<option value="offline">Offline</option>
							</select>
						</div>
						<div>
							<label for="archive" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
								Archive
							</label>
							<select
								id="archive"
								bind:value={hasArchiveFilter}
								class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500 dark:bg-gray-700 dark:border-gray-600 dark:text-gray-100"
							>
								<option value="">All</option>
								<option value="true">Has Archive</option>
								<option value="false">No Archive</option>
							</select>
						</div>
						<div>
							<label for="sort" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
								Sort By
							</label>
							<select
								id="sort"
								bind:value={sortBy}
								class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500 dark:bg-gray-700 dark:border-gray-600 dark:text-gray-100"
							>
								<option value="updated_at">Last Updated</option>
								<option value="created_at">Date Added</option>
								<option value="sitename">Name</option>
							</select>
						</div>
					</div>
					<div class="mt-4 flex gap-3">
						<button
							onclick={() => (sortOrder = sortOrder === 'desc' ? 'asc' : 'desc')}
							class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600"
						>
							{sortOrder === 'desc' ? '↓ Descending' : '↑ Ascending'}
						</button>
						<button
							onclick={resetFilters}
							class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600"
						>
							Reset
						</button>
					</div>
				</div>
			</div>
		{/if}

		<!-- Wiki List -->
		<div class="bg-white dark:bg-gray-800 shadow overflow-hidden sm:rounded-md">
			<ul class="divide-y divide-gray-200 dark:divide-gray-700">
				{#each displayedWikis as wiki (wiki.id)}
					<li class="hover:bg-gray-50 dark:hover:bg-gray-700/50">
						<a href="/wikis/{wiki.id}" class="block">
							<div class="px-4 py-4 sm:px-6">
								<div class="flex items-center justify-between">
									<div class="flex items-center gap-4 flex-1 min-w-0">
										<!-- Thumbnail -->
										<img
											src={`${APP_CONFIG.apiBaseUrl}/api/wikis/${wiki.id}/thumbnail`}
											alt={wiki.sitename || wiki.url}
											class="h-12 w-12 rounded object-cover flex-shrink-0"
										/>
										<div class="flex-1 min-w-0">
											<p
												class="text-sm font-medium text-primary-600 dark:text-primary-400 truncate"
											>
												{wiki.sitename || wiki.url}
											</p>
											<p class="mt-1 text-sm text-gray-500 dark:text-gray-400 truncate">
												{wiki.url}
											</p>
											{#if wiki.lang}
												<p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
													Language: {wiki.lang.toUpperCase()}
												</p>
											{/if}
										</div>
									</div>
									<div class="ml-5 flex-shrink-0 flex items-center gap-3">
										<StatusBadge status={wiki.status} />
										<span
											class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {wiki.has_archive
												? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
												: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'}"
										>
											{wiki.has_archive ? 'Archived' : 'No Archive'}
										</span>
									</div>
								</div>
							</div>
						</a>
					</li>
				{/each}
			</ul>
		</div>

		{#if limit && displayedWikis.length === limit}
			<div class="mt-4 text-center">
				<a
					href="/wikis"
					class="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600"
				>
					View All Wikis
				</a>
			</div>
		{/if}
	</div>
{/if}
