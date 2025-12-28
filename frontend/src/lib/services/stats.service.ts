import { apiClient } from '../apiClient';
import type { StatsSummary } from '$lib/types';

export const statsService = {
	async getSummary(): Promise<StatsSummary> {
		return apiClient.get<StatsSummary>('/api/stats/summary');
	}
};
