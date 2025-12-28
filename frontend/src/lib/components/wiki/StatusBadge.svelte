<script lang="ts">
	import type { WikiStatus } from '$lib/types';

	export let status: WikiStatus;

	const statusConfig: Record<
		WikiStatus,
		{ label: string; bgClass: string; textClass: string }
	> = {
		pending: {
			label: 'Pending',
			bgClass: 'bg-yellow-100',
			textClass: 'text-yellow-800'
		},
		ok: {
			label: 'OK',
			bgClass: 'bg-green-100',
			textClass: 'text-green-800'
		},
		error: {
			label: 'Error',
			bgClass: 'bg-red-100',
			textClass: 'text-red-800'
		},
		offline: {
			label: 'Offline',
			bgClass: 'bg-gray-100',
			textClass: 'text-gray-800'
		}
	};

	const config = statusConfig[status];
</script>

<span
	class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {config.bgClass} {config.textClass}"
>
	{config.label}
</span>
