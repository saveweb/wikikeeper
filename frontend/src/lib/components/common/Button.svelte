<script lang="ts">
	let {
		type = 'button',
		variant = 'primary',
		size = 'md',
		disabled = false,
		loading = false,
		className = '',
		children
	}: {
		type?: 'button' | 'submit' | 'reset';
		variant?: 'primary' | 'secondary' | 'danger';
		size?: 'sm' | 'md' | 'lg';
		disabled?: boolean;
		loading?: boolean;
		className?: string;
		children?: any;
	} = $props();

	const variantClasses = {
		primary:
			'bg-primary-600 hover:bg-primary-700 text-white focus:ring-primary-500 focus:ring-offset-2',
		secondary:
			'bg-gray-200 hover:bg-gray-300 text-gray-900 focus:ring-gray-500 focus:ring-offset-2',
		danger:
			'bg-red-600 hover:bg-red-700 text-white focus:ring-red-500 focus:ring-offset-2'
	};

	const sizeClasses = {
		sm: 'px-3 py-1.5 text-sm',
		md: 'px-4 py-2 text-base',
		lg: 'px-6 py-3 text-lg'
	};

	const buttonClass = $derived(`
		inline-flex items-center justify-center
		border border-transparent
		rounded-md
		font-medium
		shadow-sm
		focus:outline-none focus:ring-2 focus:ring-offset-2
		disabled:opacity-50 disabled:cursor-not-allowed
		transition-colors
		${variantClasses[variant]}
		${sizeClasses[size]}
		${className}
	`);
</script>

<button {type} {disabled} class={buttonClass}>
	{#if loading}
		<span class="mr-2">
			<span class="w-4 h-4 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
		</span>
	{/if}
	{@render children()}
</button>
