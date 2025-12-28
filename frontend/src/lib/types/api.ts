export class ApiError extends Error {
	constructor(message: string, public status: number) {
		super(message);
		this.name = 'ApiError';
	}
}

export interface PaginationMeta {
	total: number;
	page: number;
	page_size: number;
}

export interface ListResponse<T> {
	data: T[];
	meta: PaginationMeta;
}
