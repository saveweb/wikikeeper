<script lang="ts">
	import { onMount } from 'svelte';
	import { statsStore, wikiStore } from '$lib/stores';
	import LoadingSpinner from '$lib/components/common/LoadingSpinner.svelte';
	import WikiList from '$lib/components/wiki/WikiList.svelte';
	import { formatNumber } from '$lib/utils/format';

	onMount(async () => {
		await statsStore.loadSummary();
		await wikiStore.load({ page: 1, page_size: 10 });
	});

	const stats = $derived.by(() => {
		const s = $statsStore.summary;
		return [
			{ name: 'Total Wikis', value: s?.total_wikis || 0, color: 'bg-blue-500' },
			{ name: 'OK', value: s?.status_ok_wikis || 0, color: 'bg-green-500' },
			{ name: 'Errors', value: s?.status_error_wikis || 0, color: 'bg-red-500' },
			{ name: 'Archived', value: s?.archived_wikis || 0, color: 'bg-purple-500' }
		];
	});

	const loading = $derived.by(() => $statsStore.loading);
	const wikis = $derived.by(() => $wikiStore.wikis);
</script>

<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8">
	<div class="mb-8">
		<h1 class="text-3xl font-bold text-gray-900">Dashboard</h1>
		<p class="mt-2 text-gray-600">
			Overview of tracked wiki statistics and archive status
		</p>
	</div>

	{#if loading}
		<div class="flex justify-center items-center h-64">
			<LoadingSpinner size="lg" />
		</div>
	{:else}
		<!-- Stats Cards -->
		<div class="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4 mb-8">
			{#each stats as stat}
				<div class="bg-white overflow-hidden shadow rounded-lg">
					<div class="p-5">
						<div class="flex items-center">
							<div class="flex-shrink-0">
								<div class="{stat.color} w-8 h-8 rounded-md"></div>
							</div>
							<div class="ml-5 w-0 flex-1">
								<dl>
									<dt
										class="text-sm font-medium text-gray-500 truncate"
									>
										{stat.name}
									</dt>
									<dd>
										<div
											class="text-lg font-medium text-gray-900"
										>
											{formatNumber(stat.value)}
										</div>
									</dd>
								</dl>
							</div>
						</div>
					</div>
				</div>
			{/each}
		</div>

		<!-- Quick Actions -->
		<div class="bg-white shadow rounded-lg mb-8">
			<div class="px-4 py-5 sm:p-6">
				<div class="flex flex-wrap gap-4">
					<a
						href="/wikis/add"
						class="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-primary-600 hover:bg-primary-700"
					>
						Add Wiki
					</a>
					<a
						href="/wikis"
						class="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
					>
						View All Wikis
					</a>
				</div>
			</div>
		</div>

		<!-- Recent Wikis -->
		<div class="bg-white shadow rounded-lg">
			<div class="px-4 py-5 sm:p-6">
				<h2 class="text-lg font-medium text-gray-900 mb-4">
					Recent Wikis
				</h2>
				<WikiList wikis={wikis} showFilters={false} limit={10} />
			</div>
		</div>
	{/if}
</div>
