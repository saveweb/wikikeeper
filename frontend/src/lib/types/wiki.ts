export type WikiStatus = 'pending' | 'ok' | 'error' | 'offline';

export interface Wiki {
	id: string;
	url: string;
	api_url: string | null;
	sitename: string | null;
	lang: string | null;
	status: WikiStatus;
	has_archive: boolean;
	api_available: boolean;
	created_at: string;
	updated_at: string;
	last_check_at: string | null;
	last_error: string | null;
	last_error_at: string | null;
	archive_last_check_at: string | null;
	archive_last_error: string | null;
	archive_last_error_at: string | null;
	wiki_name?: string;
	mediawiki_version?: string;
	dbtype?: string;
	dbversion?: string;
	max_page_id?: number | null;
	is_active?: boolean;
}

export interface WikiCreate {
	url: string;
	wiki_name?: string;
}

export interface WikiFilters {
	page?: number;
	page_size?: number;
	status?: WikiStatus;
	has_archive?: boolean;
	search?: string;
}

export interface PaginatedResponse<T> {
	data: T[];
	page: number;
	page_size: number;
	total: number;
}
