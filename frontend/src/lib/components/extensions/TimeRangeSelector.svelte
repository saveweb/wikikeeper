<script lang="ts">
	export let onTimeRangeChange: (from: string, to: string, label: string) => void;

	type RangeOption = '1y' | '10y';
	let selectedRange: RangeOption | null = null;

	const ranges: { value: RangeOption; label: string }[] = [
		{ value: '1y', label: 'Last 1 Year' },
		{ value: '10y', label: 'Last 10 Years' }
	];

	function handleRangeChange(range: RangeOption) {
		selectedRange = range;

		const now = new Date();
		let from: Date;
		const to = now;

		switch (range) {
			case '1y':
				from = new Date(now.getTime() - 365 * 24 * 60 * 60 * 1000);
				break;
			case '10y':
				from = new Date(now.getTime() - 10 * 365 * 24 * 60 * 60 * 1000);
				break;
		}

		onTimeRangeChange(from.toISOString(), to.toISOString(), ranges.find(r => r.value === range)!.label);
	}
</script>

<div class="bg-gray-50 border border-gray-200 rounded-lg p-4 mb-4">
	<div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
		<div>
			<h3 class="text-sm font-semibold text-gray-900">View Historical Extensions</h3>
			<p class="text-xs text-gray-600 mt-1">Select a time range to browse and view past extension snapshots</p>
		</div>
		<div class="flex gap-2">
			{#each ranges as range}
				<button
					onclick={() => handleRangeChange(range.value)}
					class="px-4 py-2 text-sm font-medium rounded-lg transition-colors {
						selectedRange === range.value
							? 'bg-primary-600 text-white shadow-sm'
							: 'bg-white text-gray-700 border border-gray-300 hover:bg-gray-50'
					}"
				>
					{range.label}
				</button>
			{/each}
		</div>
	</div>
</div>
