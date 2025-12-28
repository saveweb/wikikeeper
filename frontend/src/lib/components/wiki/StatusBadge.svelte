<script lang="ts">
	import type { WikiStatus } from '$lib/types';

	export let status: WikiStatus;

	const statusConfig: Record<
		WikiStatus,
		{ label: string; bgClass: string; textClass: string }
	> = {
		pending: {
			label: 'Pending',
			bgClass: 'bg-yellow-100 dark:bg-yellow-900',
			textClass: 'text-yellow-800 dark:text-yellow-200'
		},
		ok: {
			label: 'OK',
			bgClass: 'bg-green-100 dark:bg-green-900',
			textClass: 'text-green-800 dark:text-green-200'
		},
		error: {
			label: 'Error',
			bgClass: 'bg-red-100 dark:bg-red-900',
			textClass: 'text-red-800 dark:text-red-200'
		},
		offline: {
			label: 'Offline',
			bgClass: 'bg-gray-100 dark:bg-gray-700',
			textClass: 'text-gray-800 dark:text-gray-200'
		}
	};

	const config = statusConfig[status];
</script>

<span
	class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {config.bgClass} {config.textClass}"
>
	{config.label}
</span>
