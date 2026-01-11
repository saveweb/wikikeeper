<script lang="ts">
	export let onTimeRangeChange: (from: string, to: string) => void;

	type RangeOption = '7d' | '30d' | '90d';
	let selectedRange: RangeOption = '30d';

	const ranges: { value: RangeOption; label: string }[] = [
		{ value: '7d', label: 'Last 7 days' },
		{ value: '30d', label: 'Last 30 days' },
		{ value: '90d', label: 'Last 90 days' }
	];

	function handleRangeChange(range: RangeOption) {
		selectedRange = range;

		const now = new Date();
		let from: Date;
		const to = now;

		switch (range) {
			case '7d':
				from = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
				break;
			case '30d':
				from = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);
				break;
			case '90d':
				from = new Date(now.getTime() - 90 * 24 * 60 * 60 * 1000);
				break;
		}

		onTimeRangeChange(from.toISOString(), to.toISOString());
	}
</script>

<div class="flex items-center gap-2 mb-4">
	<span class="text-sm text-gray-700">Time Range:</span>
	<div class="flex gap-2">
		{#each ranges as range}
			<button
				onclick={() => handleRangeChange(range.value)}
				class="px-3 py-1 text-sm rounded-md {
					selectedRange === range.value
						? 'bg-primary-600 text-white'
						: 'bg-gray-100 text-gray-700 hover:bg-gray-200'
				}"
			>
				{range.label}
			</button>
		{/each}
	</div>
</div>
