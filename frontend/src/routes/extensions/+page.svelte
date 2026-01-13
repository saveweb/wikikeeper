<script lang="ts">
	import { onMount } from 'svelte';
	import { extensionsService } from '$lib/services/extensions.service';
	import type { ExtensionStats } from '$lib/types/extensions';
	import LoadingSpinner from '$lib/components/common/LoadingSpinner.svelte';

	// State
	let extensions = $state<ExtensionStats[]>([]);
	let loading = $state(false);
	let error = $state('');
	let currentPage = $state(1);
	let pageSize = $state(50);
	let totalItems = $state(0);
	let totalPages = $state(0);

	// Search/filter
	let searchTerm = $state('');
	let filteredExtensions = $derived(
		extensions.filter((ext) =>
			ext.name.toLowerCase().includes(searchTerm.toLowerCase())
		)
	);

	onMount(() => {
		loadExtensions();
	});

	async function loadExtensions() {
		loading = true;
		error = '';
		try {
			const response = await extensionsService.getAllExtensions(currentPage, pageSize);
			extensions = response.extensions;
			totalItems = response.total;
			totalPages = response.pages;
		} catch (err: any) {
			error = err?.detail || 'Failed to load extensions';
		} finally {
			loading = false;
		}
	}

	function goToPage(page: number) {
		if (page < 1 || page > totalPages) return;
		currentPage = page;
		loadExtensions();
	}

	function getExtensionUrl(name: string) {
		return `/extensions/${encodeURIComponent(name)}`;
	}
</script>

<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8">
	<h1 class="text-3xl font-bold text-gray-900 mb-8">
		All Extensions & Skins
	</h1>

	<!-- Search Bar -->
	<div class="mb-6">
		<input
			type="text"
			placeholder="Search extensions..."
			bind:value={searchTerm}
			class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
		/>
		{#if searchTerm}
			<p class="text-sm text-gray-600 mt-2">
				Found {filteredExtensions.length} extensions (current page)
			</p>
		{:else}
			<p class="text-sm text-gray-600 mt-2">
				Total: {totalItems} extensions | Page {currentPage} of {totalPages}
			</p>
		{/if}
	</div>

	<!-- Extensions List -->
	{#if loading}
		<div class="flex justify-center items-center h-64">
			<LoadingSpinner size="lg" />
		</div>
	{:else if error}
		<div class="bg-red-50 border border-red-200 rounded-md p-4">
			<p class="text-sm text-red-700">{error}</p>
		</div>
	{:else if filteredExtensions.length > 0}
		<div class="bg-white shadow rounded-lg overflow-hidden">
			<div class="overflow-x-auto">
				<table class="min-w-full divide-y divide-gray-200">
					<thead class="bg-gray-50">
						<tr>
							<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
								Rank
							</th>
							<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
								Extension Name
							</th>
							<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
								Wikis Using It
							</th>
							<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
								Action
							</th>
						</tr>
					</thead>
					<tbody class="bg-white divide-y divide-gray-200">
						{#each filteredExtensions as ext (ext.name)}
							<tr class="hover:bg-gray-50">
								<td class="px-6 py-4 whitespace-nowrap">
									<span class="text-sm text-gray-900">
										{(currentPage - 1) * pageSize + extensions.indexOf(ext) + 1}
									</span>
								</td>
								<td class="px-6 py-4 whitespace-nowrap">
									<div class="flex items-center">
										<div>
											<div class="text-sm font-medium text-gray-900">
												{ext.name}
											</div>
										</div>
									</div>
								</td>
								<td class="px-6 py-4 whitespace-nowrap">
									<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
										{ext.count} wiki{ext.count !== 1 ? 's' : ''}
									</span>
								</td>
								<td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
									<a
										href={getExtensionUrl(ext.name)}
										class="text-blue-600 hover:text-blue-900 hover:underline"
									>
										View Details →
									</a>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>

		<!-- Pagination -->
		{#if !searchTerm && totalPages > 1}
			<div class="mt-6 flex items-center justify-between">
				<div class="text-sm text-gray-700">
					Showing {(currentPage - 1) * pageSize + 1} to {Math.min(currentPage * pageSize, totalItems)} of {totalItems} extensions
				</div>
				<div class="flex gap-2">
					<button
						onclick={() => goToPage(currentPage - 1)}
						disabled={currentPage === 1}
						class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						Previous
					</button>
					
					{#if totalPages <= 7}
						{#each Array(totalPages) as _, i (i)}
							<button
								onclick={() => goToPage(i + 1)}
								disabled={currentPage === i + 1}
								class="px-4 py-2 border rounded-md text-sm font-medium {currentPage === i + 1 ? 'bg-blue-600 text-white border-blue-600' : 'bg-white text-gray-700 border-gray-300 hover:bg-gray-50'}"
							>
								{i + 1}
							</button>
						{/each}
					{:else}
						{#if currentPage > 3}
							<button
								onclick={() => goToPage(1)}
								class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
							>
								1
							</button>
							{#if currentPage > 4}
								<span class="px-2 py-2 text-gray-500">...</span>
							{/if}
						{/if}
						
						{#each [currentPage - 1, currentPage, currentPage + 1] as p}
							{#if p > 0 && p <= totalPages}
								<button
									onclick={() => goToPage(p)}
									disabled={currentPage === p}
									class="px-4 py-2 border rounded-md text-sm font-medium {currentPage === p ? 'bg-blue-600 text-white border-blue-600' : 'bg-white text-gray-700 border-gray-300 hover:bg-gray-50'}"
								>
									{p}
								</button>
							{/if}
						{/each}
						
						{#if currentPage < totalPages - 2}
							{#if currentPage < totalPages - 3}
								<span class="px-2 py-2 text-gray-500">...</span>
							{/if}
							<button
								onclick={() => goToPage(totalPages)}
								class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
							>
								{totalPages}
							</button>
						{/if}
					{/if}

					<button
						onclick={() => goToPage(currentPage + 1)}
						disabled={currentPage === totalPages}
						class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						Next
					</button>
				</div>
			</div>
		{/if}
	{:else}
		<div class="bg-yellow-50 border border-yellow-200 rounded-md p-4">
			<p class="text-sm text-yellow-700">
				{#if searchTerm}
					No extensions found matching "{searchTerm}"
				{:else}
					No extensions found
				{/if}
			</p>
		</div>
	{/if}
</div>
