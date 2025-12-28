import { apiClient } from '../apiClient';
import type { Wiki, WikiCreate, WikiStats, WikiArchive, WikiFilters, ArchiveCheckResult, PaginatedResponse, WikiStatsResponse, WikiArchiveResponse } from '$lib/types';

export const wikiService = {
	async list(filters?: WikiFilters): Promise<PaginatedResponse<Wiki>> {
		const queryParams = new URLSearchParams();
		if (filters?.page) queryParams.set('page', filters.page.toString());
		if (filters?.page_size) queryParams.set('page_size', filters.page_size.toString());
		if (filters?.status) queryParams.set('status', filters.status);
		if (filters?.has_archive !== undefined) {
			queryParams.set('has_archive', filters.has_archive.toString());
		}
		if (filters?.search) queryParams.set('search', filters.search);

		const query = queryParams.toString();
		return apiClient.get<PaginatedResponse<Wiki>>(`/api/wikis${query ? `?${query}` : ''}`);
	},

	async getById(id: string): Promise<Wiki> {
		return apiClient.get<Wiki>(`/api/wikis/${id}`);
	},

	async create(data: WikiCreate): Promise<Wiki> {
		return apiClient.post<Wiki>('/api/wikis', data);
	},

	async delete(id: string): Promise<{ detail: string; wiki_id: string }> {
		return apiClient.delete<{ detail: string; wiki_id: string }>(`/api/admin/wikis/${id}`);
	},

	async triggerCheck(id: string): Promise<{ detail: string; wiki_id: string }> {
		return apiClient.post<{ detail: string; wiki_id: string }>(`/api/wikis/${id}/check`, {});
	},

	async getStats(id: string, days: number = 30): Promise<WikiStats[]> {
		const response = await apiClient.get<WikiStatsResponse>(`/api/wikis/${id}/stats?days=${days}`);
		return response.data;
	},

	async getArchives(id: string): Promise<WikiArchive[]> {
		const response = await apiClient.get<WikiArchiveResponse>(`/api/wikis/${id}/archives`);
		return response.data;
	},

	async checkArchive(id: string): Promise<ArchiveCheckResult> {
		return apiClient.post<ArchiveCheckResult>(`/api/wikis/${id}/check-archive`, {});
	}
};
