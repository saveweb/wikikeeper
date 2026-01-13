export interface WikiExtensionsSnapshot {
	id: string;
	wiki_id: string;
	snapshot_at: string;
	valid_until: string | null;
	mediawiki_version: string | null;
	items: WikiExtensionItem[];
}

export interface WikiExtensionItem {
	id: number;
	snapshot_id: string;
	ext_type: string; // 'extension' or 'skin'
	name: string;
	url: string | null;
	version: string | null;
	license_name: string | null;
	created_at: string;
}

export interface ExtensionsHistoryResponse {
	wiki_id: string;
	from: string;
	to: string;
	snapshots: WikiExtensionsSnapshot[];
}

export interface ExtensionWikiInfo {
	wiki_id: string;
	wiki_name: string | null;
	sitename: string | null;
	url: string;
	snapshot_at: string;
	version: string | null;
}

export interface ExtensionWikisResponse {
	extension_name: string;
	total: number;
	page: number;
	limit: number;
	data: ExtensionWikiInfo[];
}

export interface ExtensionVersionStats {
	version: string;
	count: number;
}

export interface ExtensionVersionsResponse {
	extension_name: string;
	total_wikis: number;
	versions: ExtensionVersionStats[];
}

export interface ExtensionStats {
	name: string;
	count: number;
}

export interface ExtensionsListResponse {
	extensions: ExtensionStats[];
	total: number;
	page: number;
	limit: number;
	pages: number;
}
