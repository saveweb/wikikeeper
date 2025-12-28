export function formatNumber(num: number): string {
	return num.toLocaleString();
}

export function formatBytes(bytes: number): string {
	const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
	if (bytes === 0) return '0 Bytes';
	const i = Math.floor(Math.log(bytes) / Math.log(1024));
	return Math.round((bytes / Math.pow(1024, i)) * 100) / 100 + ' ' + sizes[i];
}

export function validateUrl(url: string): { valid: boolean; error?: string } {
	try {
		new URL(url);
		if (!url.match(/^https?:\/\/.+/)) {
			return { valid: false, error: 'URL must start with http:// or https://' };
		}
		return { valid: true };
	} catch {
		return { valid: false, error: 'Invalid URL format' };
	}
}
