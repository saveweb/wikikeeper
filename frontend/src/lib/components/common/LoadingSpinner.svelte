<script lang="ts">
	export let size: 'sm' | 'md' | 'lg' = 'md';

	const sizeClasses = {
		sm: 'w-4 h-4',
		md: 'w-8 h-8',
		lg: 'w-12 h-12'
	};
</script>

<div class="flex items-center justify-center">
	<div
		class="{sizeClasses[size]} animate-spin rounded-full border-4 border-gray-200 border-t-primary-600"
		role="status"
		aria-label="Loading"
	>
		<span class="sr-only">Loading...</span>
	</div>
</div>
