import { writable } from 'svelte/store';
import { statsService, wikiService } from '$lib/services';
import type { StatsSummary, WikiStats } from '$lib/types';

interface StatsState {
	summary: StatsSummary | null;
	loading: boolean;
	error: string | null;
}

const createStatsStore = () => {
	const { subscribe, set, update } = writable<StatsState>({
		summary: null,
		loading: false,
		error: null
	});

	return {
		subscribe,
		loadSummary: async () => {
			update((s) => ({ ...s, loading: true, error: null }));
			try {
				const summary = await statsService.getSummary();
				update((s) => ({ ...s, summary, loading: false }));
			} catch (error) {
				update((s) => ({
					...s,
					error: error instanceof Error ? error.message : 'Failed to load stats',
					loading: false
				}));
			}
		},
		loadWikiStats: async (wikiId: string, days: number = 30): Promise<WikiStats[]> => {
			return wikiService.getStats(wikiId, days);
		}
	};
};

export const statsStore = createStatsStore();
