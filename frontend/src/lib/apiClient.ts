/**
 * API Client wrapper that automatically adds admin token
 */

const API_BASE = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8000';

interface RequestOptions {
	method?: string;
	headers?: Record<string, string>;
	body?: any;
}

export class ApiClient {
	private baseUrl: string;

	constructor(baseUrl: string = API_BASE) {
		this.baseUrl = baseUrl;
	}

	private getHeaders(): Record<string, string> {
		const headers: Record<string, string> = {
			'Content-Type': 'application/json',
		};

		// Add admin token if available
		const adminToken = localStorage.getItem('admintoken');
		if (adminToken) {
			headers['Cookie'] = `admintoken=${adminToken}`;
		}

		return headers;
	}

	async request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
		const url = `${this.baseUrl}${endpoint}`;
		const config: RequestInit = {
			method: options.method || 'GET',
			headers: {
				...this.getHeaders(),
				...options.headers,
			},
		};

		if (options.body) {
			config.body = JSON.stringify(options.body);
		}

		const response = await fetch(url, config);

		if (!response.ok) {
			throw await response.json().catch(() => ({ detail: response.statusText }));
		}

		return response.json() as Promise<T>;
	}

	get<T>(endpoint: string): Promise<T> {
		return this.request<T>(endpoint);
	}

	post<T>(endpoint: string, body?: any): Promise<T> {
		return this.request<T>(endpoint, { method: 'POST', body });
	}

	delete<T>(endpoint: string): Promise<T> {
		return this.request<T>(endpoint, { method: 'DELETE' });
	}
}

// Create singleton instance
export const apiClient = new ApiClient();
