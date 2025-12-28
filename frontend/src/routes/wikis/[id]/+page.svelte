<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { wikiService } from '$lib/services';
	import LoadingSpinner from '$lib/components/common/LoadingSpinner.svelte';
	import StatusBadge from '$lib/components/wiki/StatusBadge.svelte';
	import StatsChart from '$lib/components/charts/StatsChart.svelte';
	import { formatRelativeTime, formatShortDate, formatFileSize } from '$lib/utils/date';
	import type { Wiki, WikiStats, WikiArchive } from '$lib/types';

	let wiki = $state<Wiki | null>(null);
	let stats = $state<WikiStats[]>([]);
	let archives = $state<WikiArchive[]>([]);
	let loading = $state(true);
	let error = $state('');
	let checkingStats = $state(false);
	let checkingArchive = $state(false);
	let deleting = $state(false);
	let showDeleteConfirm = $state(false);
	let chartHeight = 400;

	// Calculate max archive size for progress bar comparison
	const maxArchiveSize = $derived.by(() => {
		const sizes = archives.map((a) => a.item_size || 0);
		return Math.max(...sizes, 0);
	});

	onMount(async () => {
		await loadData();
	});

	async function loadData() {
		try {
			const id = $page.params.id;
			if (!id) {
				throw new Error('Wiki ID is required');
			}
			const [wikiData, statsData, archivesData] = await Promise.all([
				wikiService.getById(id),
				wikiService.getStats(id, 30),
				wikiService.getArchives(id)
			]);
			wiki = wikiData;
			stats = statsData;
			archives = archivesData;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load wiki details';
		} finally {
			loading = false;
		}
	}

	async function triggerCheck() {
		if (!wiki) return;
		checkingStats = true;
		try {
			await wikiService.triggerCheck(wiki.id);
			// Reload data after a delay
			setTimeout(async () => {
				await loadData();
			}, 2000);
		} catch (err) {
			alert((err as any)?.detail || (err as Error)?.message || 'Failed to trigger check');
		} finally {
			checkingStats = false;
		}
	}

	async function checkArchive() {
		if (!wiki) return;
		checkingArchive = true;
		try {
			await wikiService.checkArchive(wiki.id);
			await loadData();
		} catch (err) {
			alert((err as any)?.detail || (err as Error)?.message || 'Failed to check archive');
		} finally {
			checkingArchive = false;
		}
	}

	async function deleteWiki() {
		if (!wiki) return;
		if (!confirm(`Are you sure you want to delete "${wiki.sitename || wiki.url}"? This action cannot be undone.`)) {
			return;
		}

		deleting = true;
		try {
			await wikiService.delete(wiki.id);
			// Redirect to wikis list
			window.location.href = '/wikis';
		} catch (err) {
			alert(err instanceof Error ? err.message : 'Failed to delete wiki');
			deleting = false;
		}
	}
</script>

<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8">
	{#if loading}
		<div class="flex justify-center items-center h-64">
			<LoadingSpinner size="lg" />
		</div>
	{:else if error}
		<div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
			<p class="text-sm text-red-800 dark:text-red-200">{error}</p>
		</div>
	{:else if wiki}
		<!-- Header -->
		<div class="mb-8">
			<div class="flex items-start gap-6">
				<!-- Large Thumbnail -->
				<img
					src={`/api/wikis/${wiki.id}/thumbnail`}
					alt={wiki.sitename || wiki.url}
					class="h-24 w-24 rounded-lg object-cover flex-shrink-0 shadow-lg"
				/>
				<div class="flex-1">
					<div class="flex items-start justify-between">
						<div>
							<h1 class="text-3xl font-bold text-gray-900 dark:text-gray-100">
								{wiki.sitename || 'Unnamed Wiki'}
							</h1>
							<p class="mt-2 text-gray-600 dark:text-gray-400">
								<a href={wiki.url} target="_blank" rel="noopener noreferrer" class="hover:underline">
									{wiki.url}
								</a>
							</p>
						</div>
						<div class="flex gap-3">
							<button
								onclick={triggerCheck}
								disabled={checkingStats}
								class="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-primary-600 hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed"
							>
								{#if checkingStats}
									<span class="mr-2">
										<span class="w-4 h-4 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
									</span>
									Checking...
								{:else}
									Check Stats
								{/if}
							</button>
							<button
								onclick={checkArchive}
								disabled={checkingArchive}
								class="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed"
							>
								{#if checkingArchive}
									<span class="mr-2">
										<span class="w-4 h-4 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
									</span>
									Checking...
								{:else}
									Check Archive
								{/if}
							</button>
							<button
								onclick={deleteWiki}
								disabled={deleting}
								class="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-red-600 hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
							>
								{#if deleting}
									<span class="mr-2">
										<span class="w-4 h-4 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
									</span>
									Deleting...
								{:else}
									Delete Wiki
								{/if}
							</button>
						</div>
					</div>
				</div>
				<div class="mt-4 flex flex-wrap items-center gap-4">
					<StatusBadge status={wiki.status} />
					<span
						class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {wiki.has_archive
							? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
							: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'}"
					>
						{wiki.has_archive ? 'Has Archive' : 'No Archive'}
					</span>
				</div>

				<!-- Status Information -->
				<div class="mt-6 grid grid-cols-1 md:grid-cols-2 gap-4">
					<!-- Siteinfo Status -->
					<div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
						<h3 class="text-sm font-medium text-blue-900 dark:text-blue-200 mb-2">
							📊 Siteinfo
						</h3>
						{#if wiki.last_check_at}
							<p class="text-xs text-blue-700 dark:text-blue-300 mb-1">
								Last checked: {formatRelativeTime(wiki.last_check_at)}
							</p>
						{:else}
							<p class="text-xs text-blue-700 dark:text-blue-300 mb-1">
								Not yet checked
							</p>
						{/if}
						{#if wiki.last_error}
							<div class="mt-2">
								<p class="text-xs font-medium text-red-700 dark:text-red-300 mb-1">
									❌ Error
									{#if wiki.last_error_at}
										<span class="text-xs text-red-600 dark:text-red-400 ml-1">
											({formatRelativeTime(wiki.last_error_at)})
										</span>
									{/if}
								</p>
								<p class="text-xs text-red-600 dark:text-red-400 break-words">
									{wiki.last_error}
								</p>
							</div>
						{:else if wiki.last_check_at}
							<p class="text-xs text-green-700 dark:text-green-300">
								✅ Last check successful
							</p>
						{/if}
					</div>

					<!-- Archive Check Status -->
					<div class="bg-purple-50 dark:bg-purple-900/20 border border-purple-200 dark:border-purple-800 rounded-lg p-4">
						<h3 class="text-sm font-medium text-purple-900 dark:text-purple-200 mb-2">
							📦 Archive.org Check
						</h3>
						{#if wiki.archive_last_check_at}
							<p class="text-xs text-purple-700 dark:text-purple-300 mb-1">
								Last checked: {formatRelativeTime(wiki.archive_last_check_at)}
							</p>
						{:else}
							<p class="text-xs text-purple-700 dark:text-purple-300 mb-1">
								Not yet checked
							</p>
						{/if}
						{#if wiki.archive_last_error}
							<div class="mt-2">
								<p class="text-xs font-medium text-red-700 dark:text-red-300 mb-1">
									❌ Error
									{#if wiki.archive_last_error_at}
										<span class="text-xs text-red-600 dark:text-red-400 ml-1">
											({formatRelativeTime(wiki.archive_last_error_at)})
										</span>
									{/if}
								</p>
								<p class="text-xs text-red-600 dark:text-red-400 break-words">
									{wiki.archive_last_error}
								</p>
							</div>
						{:else if wiki.archive_last_check_at}
							<p class="text-xs text-green-700 dark:text-green-300">
								✅ Last check successful
							</p>
						{/if}
					</div>
				</div>
		    </div>
		</div>

		<!-- Wiki Info -->
		<div class="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-8">
			<div class="lg:col-span-2">
				<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
					<div class="px-4 py-5 sm:p-6">
						<h2 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">
							Wiki Information
						</h2>
						<dl class="grid grid-cols-1 gap-x-4 gap-y-6 sm:grid-cols-2">
							{#if wiki.mediawiki_version}
								<div>
									<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">
										MediaWiki Version
									</dt>
									<dd class="mt-1 text-sm text-gray-900 dark:text-gray-100">
										{wiki.mediawiki_version}
									</dd>
								</div>
							{/if}
							{#if wiki.max_page_id}
								<div>
									<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">
										Max Page ID
									</dt>
									<dd class="mt-1 text-sm text-gray-900 dark:text-gray-100">
										{wiki.max_page_id.toLocaleString()}
									</dd>
								</div>
							{/if}
							{#if wiki.lang}
								<div>
									<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">
										Language
									</dt>
									<dd class="mt-1 text-sm text-gray-900 dark:text-gray-100">
										{wiki.lang.toUpperCase()}
									</dd>
								</div>
							{/if}
							{#if wiki.dbtype}
								<div>
									<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">
										Database
									</dt>
									<dd class="mt-1 text-sm text-gray-900 dark:text-gray-100">
										{wiki.dbtype} {wiki.dbversion}
									</dd>
								</div>
							{/if}
							<div>
								<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">
									API URL
								</dt>
								<dd class="mt-1 text-sm text-gray-900 dark:text-gray-100 break-all">
									{#if wiki.api_url}
										{wiki.api_url}
									{:else}
										<span class="text-gray-400">Not available</span>
									{/if}
								</dd>
							</div>
							<div>
								<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">
									Added to WikiKeeper
								</dt>
								<dd class="mt-1 text-sm text-gray-900 dark:text-gray-100">
									{formatShortDate(wiki.created_at)}
								</dd>
							</div>
						</dl>
					</div>
				</div>
			</div>

			<!-- Latest Stats -->
			<div>
				<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
					<div class="px-4 py-5 sm:p-6">
						<h2 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">
							Latest Statistics
						</h2>
						{#if stats.length > 0}
							<dl class="space-y-3">
								<div>
									<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Pages</dt>
									<dd class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">
										{stats[stats.length - 1].pages.toLocaleString()}
									</dd>
								</div>
								<div>
									<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Articles</dt>
									<dd class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">
										{stats[stats.length - 1].articles.toLocaleString()}
									</dd>
								</div>
								<div>
									<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Edits</dt>
									<dd class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">
										{stats[stats.length - 1].edits.toLocaleString()}
									</dd>
								</div>
								<div>
									<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Active Users</dt>
									<dd class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">
										{stats[stats.length - 1].active_users.toLocaleString()}
									</dd>
								</div>
							</dl>
						{:else}
							<p class="text-sm text-gray-500 dark:text-gray-400">No statistics available</p>
						{/if}
					</div>
				</div>
			</div>
		</div>

		<!-- Stats Chart -->
		{#if stats.length > 0}
			<div class="mb-8">
				<StatsChart stats={stats} title="Statistics History (Last 30 Days)" height={chartHeight} />
			</div>
		{/if}

		<!-- Archives -->
		{#if archives.length > 0}
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
				<div class="px-4 py-5 sm:p-6">
					<h2 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">
						Archive.org Backups
					</h2>
					<div class="overflow-hidden">
						<ul class="divide-y divide-gray-200 dark:divide-gray-700">
							{#each archives as archive (archive.id)}
								<li class="py-4">
									<div class="flex items-start space-x-4">
										<!-- Thumbnail -->
										<img
											src={`https://archive.org/services/img/${archive.ia_identifier}`}
											alt={archive.ia_identifier}
											class="h-16 w-16 rounded object-cover flex-shrink-0"
										/>

										<div class="flex-1 min-w-0">
											<!-- Title & Link -->
											<p class="text-sm font-medium text-primary-600 dark:text-primary-400">
												<a href={`https://archive.org/details/${archive.ia_identifier}`} target="_blank" rel="noopener noreferrer">
													{archive.ia_identifier}
												</a>
											</p>

											<!-- Metadata -->
											<div class="mt-2 space-y-2">
												{#if archive.dump_date}
													<p class="text-sm text-gray-500 dark:text-gray-400">
														Dump: {formatShortDate(archive.dump_date)}
													</p>
												{/if}
												{#if archive.item_size}
													<div>
														<p class="text-sm text-gray-500 dark:text-gray-400 mb-1">
															Size: {formatFileSize(archive.item_size)}
														</p>
														<!-- Size comparison progress bar -->
														<div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2 overflow-hidden">
															<div
																class="h-full bg-gradient-to-r from-blue-500 to-blue-600 dark:from-blue-400 dark:to-blue-500 rounded-full transition-all duration-300"
																style="width: {maxArchiveSize > 0 ? (archive.item_size / maxArchiveSize * 100) : 0}%"
															></div>
														</div>
													</div>
												{/if}
												{#if archive.uploader}
													<p class="text-sm text-gray-500 dark:text-gray-400">
														Uploader: {archive.uploader}
													</p>
												{/if}
												{#if archive.scanner}
													<p class="text-sm text-gray-500 dark:text-gray-400">
														Scanner: {archive.scanner}
													</p>
												{/if}
											</div>

											<!-- Tags -->
											<div class="mt-3 flex flex-wrap gap-2">
												{#if archive.has_xml_current}
													<span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">
														XML Current
													</span>
												{/if}
												{#if archive.has_xml_history}
													<span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
														XML History
													</span>
												{/if}
												{#if archive.has_images_dump}
													<span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200">
														Images Dump
													</span>
												{/if}
												{#if archive.has_titles_list}
													<span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200">
														Titles List
													</span>
												{/if}
												{#if archive.has_images_list}
													<span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200">
														Images List
													</span>
												{/if}
												{#if archive.has_legacy_wikidump}
													<span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200">
														Legacy WikiDump
													</span>
												{/if}
												{#if archive.upload_state === null || archive.upload_state === undefined}
													<span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">
														Upload State: Unknown
													</span>
												{/if}
												{#if archive.upload_state && archive.upload_state !== 'uploaded'}
													<span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">
														Upload State: {archive.upload_state}
													</span>
												{/if}
											</div>
										</div>
									</div>
								</li>
							{/each}
						</ul>
					</div>
				</div>
			</div>
		{/if}
	{/if}
</div>