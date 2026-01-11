import { apiClient } from '../apiClient';
import type { WikiExtensionsSnapshot, ExtensionsHistoryResponse } from '$lib/types/extensions';

export const extensionsService = {
	async getLatest(id: string): Promise<WikiExtensionsSnapshot> {
		return apiClient.get<WikiExtensionsSnapshot>(`/api/wikis/${id}/extensions`);
	},

	async getHistory(
		id: string,
		from?: string,
		to?: string
	): Promise<ExtensionsHistoryResponse> {
		const params = new URLSearchParams();
		if (from) params.set('from', from);
		if (to) params.set('to', to);

		const query = params.toString();
		return apiClient.get<ExtensionsHistoryResponse>(
			`/api/wikis/${id}/extensions/history${query ? `?${query}` : ''}`
		);
	}
};
