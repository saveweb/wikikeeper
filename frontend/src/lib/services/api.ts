import { ApiError } from '$lib/types/api';
import { APP_CONFIG } from '$lib/constants';

export class ApiClient {
	private baseUrl: string;

	constructor(baseUrl: string = APP_CONFIG.apiBaseUrl) {
		this.baseUrl = baseUrl;
	}

	async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
		const url = `${this.baseUrl}${endpoint}`;
		const config: RequestInit = {
			...options,
			headers: {
				'Content-Type': 'application/json',
				...options.headers
			}
		};

		const response = await fetch(url, config);

		if (!response.ok) {
			const error = await response.json().catch(() => ({}));
			throw new ApiError(error.detail || response.statusText, response.status);
		}

		return response.json();
	}

	get<T>(endpoint: string): Promise<T> {
		return this.request<T>(endpoint, { method: 'GET' });
	}

	post<T>(endpoint: string, data: unknown): Promise<T> {
		return this.request<T>(endpoint, {
			method: 'POST',
			body: JSON.stringify(data)
		});
	}

	delete<T>(endpoint: string): Promise<T> {
		return this.request<T>(endpoint, { method: 'DELETE' });
	}
}

export const apiClient = new ApiClient();
