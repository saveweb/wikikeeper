import { writable, derived } from 'svelte/store';
import { wikiService } from '$lib/services';
import type { Wiki, WikiCreate, WikiFilters } from '$lib/types';

interface WikiState {
	wikis: Wiki[];
	loading: boolean;
	error: string | null;
	filters: WikiFilters;
}

const createWikiStore = () => {
	const { subscribe, set, update } = writable<WikiState>({
		wikis: [],
		loading: false,
		error: null,
		filters: { page: 1, page_size: 50 }
	});

	return {
		subscribe,
		load: async (filters?: Partial<WikiFilters>) => {
			update((s) => ({ ...s, loading: true, error: null }));
			try {
				const response = await wikiService.list(filters);
				update((s) => ({
					...s,
					wikis: response.data,
					filters: { ...s.filters, ...filters },
					loading: false
				}));
				return response.data; // Return the loaded data
			} catch (error) {
				update((s) => ({
					...s,
					error: error instanceof Error ? error.message : 'Failed to load wikis',
					loading: false
				}));
				throw error;
			}
		},
		setFilters: async (filters: Partial<WikiFilters>) => {
			let currentFilters: WikiFilters;
			update((s) => {
				currentFilters = { ...s.filters, ...filters };
				return { ...s, loading: true, error: null };
			});
			try {
				const response = await wikiService.list(currentFilters!);
				update((s) => ({
					...s,
					wikis: response.data,
					filters: currentFilters!,
					loading: false
				}));
			} catch (error) {
				update((s) => ({
					...s,
					error: error instanceof Error ? error.message : 'Failed to load wikis',
					loading: false
				}));
			}
		},
		create: async (data: WikiCreate) => {
			update((s) => ({ ...s, loading: true, error: null }));
			try {
				const newWiki = await wikiService.create(data);
				update((s) => ({
					...s,
					wikis: [newWiki, ...s.wikis],
					loading: false
				}));
				return newWiki;
			} catch (error) {
				update((s) => ({
					...s,
					error: error instanceof Error ? error.message : 'Failed to create wiki',
					loading: false
				}));
				throw error;
			}
		},
		refresh: () => {
			update((s) => {
				const { filters } = s;
				return s;
			});
		}
	};
};

export const wikiStore = createWikiStore();
export const wikiCount = derived(wikiStore, ($wikiStore) => $wikiStore.wikis.length);
