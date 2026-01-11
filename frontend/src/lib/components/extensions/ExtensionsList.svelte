<script lang="ts">
	import type { WikiExtensionItem } from '$lib/types/extensions';

	let { items, title = "Extensions & Skins", mediawikiVersion }: { items: WikiExtensionItem[]; title?: string; mediawikiVersion?: string | null } = $props();

	// Group by type
	const extensions = $derived(items.filter((item) => item.ext_type === 'skin'));
	const others = $derived(items.filter((item) => item.ext_type !== 'skin'));
</script>

<div class="bg-white shadow rounded-lg">
	<div class="px-4 py-5 sm:p-6">
		<h2 class="text-lg font-medium text-gray-900 mb-2">{title}</h2>
		{#if mediawikiVersion}
			<p class="text-sm text-gray-500 mb-4">MediaWiki: {mediawikiVersion}</p>
		{/if}

		{#if others.length > 0}
			<div class="mb-6">
				<h3 class="text-sm font-medium text-gray-700 mb-3">
					Extensions ({others.length})
				</h3>
				<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
					{#each others as ext (ext.ext_type + ':' + ext.name)}
						<div class="border border-gray-200 rounded-lg p-4 hover:shadow-md transition-shadow">
							<div>
								{#if ext.url}
									<a
										href={ext.url}
										target="_blank"
										rel="noopener noreferrer"
										class="text-sm font-medium text-primary-600 hover:text-primary-700 hover:underline"
									>
										{ext.name} ↗
									</a>
								{:else}
									<h4 class="text-sm font-medium text-gray-900">{ext.name}</h4>
								{/if}
								{#if ext.version}
									<p class="text-xs text-gray-500 mt-1">Version: {ext.version}</p>
								{/if}
								{#if ext.license_name}
									<p class="text-xs text-gray-500">License: {ext.license_name}</p>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			</div>
		{/if}

		{#if extensions.length > 0}
			<div>
				<h3 class="text-sm font-medium text-gray-700 mb-3">Skins ({extensions.length})</h3>
				<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
					{#each extensions as skin (skin.ext_type + ':' + skin.name)}
						<div class="border border-gray-200 rounded-lg p-4 hover:shadow-md transition-shadow">
							<div>
								{#if skin.url}
									<a
										href={skin.url}
										target="_blank"
										rel="noopener noreferrer"
										class="text-sm font-medium text-primary-600 hover:text-primary-700 hover:underline"
									>
										{skin.name} ↗
									</a>
								{:else}
									<h4 class="text-sm font-medium text-gray-900">{skin.name}</h4>
								{/if}
								{#if skin.version}
									<p class="text-xs text-gray-500 mt-1">Version: {skin.version}</p>
								{/if}
								{#if skin.license_name}
									<p class="text-xs text-gray-500">License: {skin.license_name}</p>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			</div>
		{/if}

		{#if items.length === 0}
			<p class="text-sm text-gray-500">No extensions or skins found</p>
		{/if}
	</div>
</div>
