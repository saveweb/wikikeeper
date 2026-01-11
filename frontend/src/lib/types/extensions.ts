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
