<script lang="ts">
	import { wikiStore } from '$lib/stores';
	import StatusBadge from '$lib/components/wiki/StatusBadge.svelte';
	import { APP_CONFIG } from '$lib/constants';
	import type { Wiki, WikiStatus } from '$lib/types';

	// Filter state
	let search = $state('');
	let statusFilter = $state<WikiStatus | ''>('');
	let hasArchiveFilter = $state<string>('');
	let currentPage = $state(1);
	const pageSize = 50;

	let loading = $state(false);
	let error = $state('');
	let wikis: Wiki[] = $state([]);

	async function loadWikis() {
		loading = true;
		error = '';

		try {
			const filters: Record<string, string | number | boolean> = {
				page: currentPage,
				page_size: pageSize
			};

			if (search) filters.search = search;
			if (statusFilter) filters.status = statusFilter;
			if (hasArchiveFilter === 'true') filters.has_archive = true;
			if (hasArchiveFilter === 'false') filters.has_archive = false;

			wikis = await wikiStore.load(filters);
		} catch (e) {
			error = (e as any)?.detail || (e as Error)?.message || 'Failed to load wikis';
		} finally {
			loading = false;
		}
	}

	// Debounced search
	let searchTimeout: ReturnType<typeof setTimeout>;
	function onSearchChange(value: string) {
		search = value;
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			currentPage = 1;
			loadWikis();
		}, 500);
	}

	function onFilterChange() {
		currentPage = 1;
		loadWikis();
	}

	function resetFilters() {
		search = '';
		statusFilter = '';
		hasArchiveFilter = '';
		currentPage = 1;
		loadWikis();
	}

	function nextPage() {
		currentPage++;
		loadWikis();
	}

	function prevPage() {
		if (currentPage > 1) {
			currentPage--;
			loadWikis();
		}
	}

	// Initial load
	loadWikis();
</script>

<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8">
	<div class="mb-8 flex justify-between items-center">
		<div>
			<h1 class="text-3xl font-bold text-gray-900">Wikis</h1>
			<p class="mt-2 text-gray-600">
				Manage and monitor tracked MediaWiki sites
			</p>
		</div>
		<a href="/wikis/add" class="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-primary-600 hover:bg-primary-700">
			Add Wiki
		</a>
	</div>

	<!-- Filters -->
	<div class="bg-white shadow rounded-lg mb-6">
		<div class="px-4 py-5 sm:p-6">
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-4">
				<div>
					<label for="search" class="block text-sm font-medium text-gray-700 mb-1">
						Search
					</label>
					<input
						id="search"
						type="text"
						value={search}
						oninput={(e) => onSearchChange(e.currentTarget.value)}
						placeholder="Search by URL or name..."
						class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500"
					/>
				</div>
				<div>
					<label for="status" class="block text-sm font-medium text-gray-700 mb-1">
						Status
					</label>
					<select
						id="status"
						bind:value={statusFilter}
						onchange={onFilterChange}
						class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500"
					>
						<option value="">All Statuses</option>
						<option value="pending">Pending</option>
						<option value="ok">OK</option>
						<option value="error">Error</option>
						<option value="offline">Offline</option>
					</select>
				</div>
				<div>
					<label for="archive" class="block text-sm font-medium text-gray-700 mb-1">
						Archive
					</label>
					<select
						id="archive"
						bind:value={hasArchiveFilter}
						onchange={onFilterChange}
						class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500"
					>
						<option value="">All</option>
						<option value="true">Has Archive</option>
						<option value="false">No Archive</option>
					</select>
				</div>
				<div class="flex items-end">
					<button
						onclick={resetFilters}
						class="w-full px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
					>
						Reset Filters
					</button>
				</div>
			</div>
		</div>
	</div>

	{#if loading}
		<div class="flex justify-center items-center h-64">
			<div class="w-8 h-8 animate-spin rounded-full border-2 border-current border-t-transparent"></div>
		</div>
	{:else if error}
		<div class="bg-red-50 border border-red-200 rounded-md p-4">
			<p class="text-sm text-red-800">{error}</p>
		</div>
	{:else if wikis.length === 0}
		<div class="text-center py-12">
			<p class="text-gray-500 text-lg mb-4">
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
		<!-- Wiki List -->
		<div class="bg-white shadow overflow-hidden sm:rounded-md">
			<ul class="divide-y divide-gray-200">
				{#each wikis as wiki (wiki.id)}
					<li class="hover:bg-gray-50">
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
												class="text-sm font-medium text-primary-600 truncate"
											>
												{wiki.sitename || wiki.url}
											</p>
											<p class="mt-1 text-sm text-gray-500 truncate">
												{wiki.url}
											</p>
											{#if wiki.lang}
												<p class="mt-1 text-xs text-gray-400">
													Language: {wiki.lang.toUpperCase()}
												</p>
											{/if}
										</div>
									</div>
									<div class="ml-5 flex-shrink-0 flex items-center gap-3">
										<StatusBadge status={wiki.status} />
										<span
											class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {wiki.has_archive
												? 'bg-green-100 text-green-800'
												: 'bg-gray-100 text-gray-800'}"
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

		<!-- Pagination -->
		<div class="mt-6 flex items-center justify-between">
			<div class="text-sm text-gray-700">
				Page <span class="font-medium">{currentPage}</span>
			</div>
			<div class="flex gap-2">
				<button
					onclick={prevPage}
					disabled={currentPage === 1}
					class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					Previous
				</button>
				<button
					onclick={nextPage}
					disabled={wikis.length < pageSize}
					class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					Next
				</button>
			</div>
		</div>
	{/if}
</div>
