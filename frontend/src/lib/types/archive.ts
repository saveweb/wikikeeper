export interface WikiArchive {
	id: string;
	wiki_id: string;
	ia_identifier: string;
	added_date: string | null;
	dump_date: string | null;
	item_size: number | null;
	uploader: string | null;
	scanner: string | null;
	upload_state: string | null;
	has_xml_current: boolean;
	has_xml_history: boolean;
	has_images_dump: boolean;
	has_titles_list: boolean;
	has_images_list: boolean;
	has_legacy_wikidump: boolean;
	created_at: string;
	updated_at: string;
}

export interface WikiArchiveResponse {
	data: WikiArchive[];
	wiki_id: string;
}

export interface ArchiveCheckResult {
	found: boolean;
	identifier?: string;
	added_date?: string;
}
