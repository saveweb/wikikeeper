import { apiClient } from '../apiClient';

export interface BulkOperationResult {
	detail: string;
}

export const adminService = {
	async deleteWiki(id: string): Promise<{ detail: string; wiki_id: string }> {
		return apiClient.delete<{ detail: string; wiki_id: string }>(`/api/admin/wikis/${id}`);
	},

	async collectAll(): Promise<BulkOperationResult> {
		return apiClient.post<BulkOperationResult>('/api/admin/collect-all');
	},

	async checkAllArchives(): Promise<BulkOperationResult> {
		return apiClient.post<BulkOperationResult>('/api/admin/check-all-archives');
	}
};
