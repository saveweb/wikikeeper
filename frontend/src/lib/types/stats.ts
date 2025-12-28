export interface WikiStats {
	wiki_id: string;
	time: string;
	pages: number;
	articles: number;
	edits: number;
	images: number;
	users: number;
	active_users: number;
	admins: number;
	jobs: number;
	response_time_ms: number | null;
	http_status: number | null;
}

export interface WikiStatsResponse {
	data: WikiStats[];
	wiki_id: string;
	days: number;
}

export interface StatsSummary {
	total_wikis: number;
	archived_wikis: number;
	status_ok_wikis: number;
	status_error_wikis: number;
	active_wikis: number;
	total_pages: number;
	total_edits: number;
}
