import { apiClient } from '../apiClient';
import type {
	WikiExtensionsSnapshot,
	ExtensionsHistoryResponse,
	ExtensionWikisResponse,
	ExtensionVersionsResponse,
	ExtensionsListResponse
} from '$lib/types/extensions';

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
	},

	async getExtensionWikis(
		name: string,
		page?: number,
		limit?: number
	): Promise<ExtensionWikisResponse> {
		const params = new URLSearchParams();
		if (page !== undefined) params.set('page', page.toString());
		if (limit !== undefined) params.set('limit', limit.toString());

		const query = params.toString();
		return apiClient.get<ExtensionWikisResponse>(
			`/api/extensions/${name}/wikis${query ? `?${query}` : ''}`
		);
	},

	async getExtensionVersions(name: string): Promise<ExtensionVersionsResponse> {
		return apiClient.get<ExtensionVersionsResponse>(`/api/extensions/${name}/versions`);
	},

	async getAllExtensions(page?: number, limit?: number): Promise<ExtensionsListResponse> {
		const params = new URLSearchParams();
		if (page !== undefined) params.set('page', page.toString());
		if (limit !== undefined) params.set('limit', limit.toString());

		const query = params.toString();
		return apiClient.get<ExtensionsListResponse>(
			`/api/extensions${query ? `?${query}` : ''}`
		);
	}
};
