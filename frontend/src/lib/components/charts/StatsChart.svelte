<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Chart } from 'chart.js/auto';
	import type { WikiStats } from '$lib/types';

	export let stats: WikiStats[] = [];
	export let title = 'Statistics Over Time';
	export let height = 400;

	let canvasElement: HTMLCanvasElement;
	let chart: Chart | null = null;

	onMount(() => {
		if (!canvasElement) return;

		// Sort stats by time
		const sortedStats = [...stats].sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime());

		const labels = sortedStats.map((s) => new Date(s.time).toLocaleDateString());
		const pagesData = sortedStats.map((s) => s.pages);
		const editsData = sortedStats.map((s) => s.edits);
		const articlesData = sortedStats.map((s) => s.articles);
		const activeUsersData = sortedStats.map((s) => s.active_users);

		const ctx = canvasElement.getContext('2d');
		if (!ctx) return;

		chart = new Chart(ctx, {
			type: 'line',
			data: {
				labels,
				datasets: [
					{
						label: 'Pages',
						data: pagesData,
						borderColor: 'rgb(59, 130, 246)',
						backgroundColor: 'rgba(59, 130, 246, 0.1)',
						yAxisID: 'y',
						tension: 0.1
					},
					{
						label: 'Articles',
						data: articlesData,
						borderColor: 'rgb(16, 185, 129)',
						backgroundColor: 'rgba(16, 185, 129, 0.1)',
						yAxisID: 'y',
						tension: 0.1
					},
					{
						label: 'Edits',
						data: editsData,
						borderColor: 'rgb(245, 158, 11)',
						backgroundColor: 'rgba(245, 158, 11, 0.1)',
						yAxisID: 'y',
						tension: 0.1
					},
					{
						label: 'Active Users',
						data: activeUsersData,
						borderColor: 'rgb(139, 92, 246)',
						backgroundColor: 'rgba(139, 92, 246, 0.1)',
						yAxisID: 'y1',
						tension: 0.1
					}
				]
			},
			options: {
				responsive: true,
				maintainAspectRatio: false,
				interaction: {
					mode: 'index',
					intersect: false
				},
				plugins: {
					title: {
						display: true,
						text: title,
						font: {
							size: 16
						}
					},
					legend: {
						position: 'top'
					},
					tooltip: {
						mode: 'index',
						intersect: false
					}
				},
				scales: {
					x: {
						display: true,
						title: {
							display: true,
							text: 'Date'
						}
					},
					y: {
						type: 'linear',
						display: true,
						position: 'left',
						title: {
							display: true,
							text: 'Count'
						}
					},
					y1: {
						type: 'linear',
						display: true,
						position: 'right',
						grid: {
							drawOnChartArea: false
						},
						title: {
							display: true,
							text: 'Users'
						}
					}
				}
			}
		});
	});

	onDestroy(() => {
		if (chart) {
			chart.destroy();
		}
	});

	$: if (chart && stats.length > 0) {
		const sortedStats = [...stats].sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime());

		chart.data.labels = sortedStats.map((s) => new Date(s.time).toLocaleDateString());
		chart.data.datasets[0].data = sortedStats.map((s) => s.pages);
		chart.data.datasets[1].data = sortedStats.map((s) => s.articles);
		chart.data.datasets[2].data = sortedStats.map((s) => s.edits);
		chart.data.datasets[3].data = sortedStats.map((s) => s.active_users);
		chart.update();
	}
</script>

<div class="bg-white shadow rounded-lg p-4 sm:p-6">
	<div style="height: {height}px">
		<canvas bind:this={canvasElement}></canvas>
	</div>
</div>
