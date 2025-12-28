import { format, formatDistanceToNow } from 'date-fns';

/**
 * Parse UTC date string from backend
 * Backend uses datetime.utcnow() which returns UTC but without 'Z' suffix
 */
function parseUTCDate(dateStr: string): Date {
	// Ensure UTC dates have 'Z' suffix for proper parsing
	if (!dateStr.endsWith('Z')) {
		dateStr = dateStr + 'Z';
	}
	return new Date(dateStr);
}

export function formatDate(dateStr: string): string {
	return format(parseUTCDate(dateStr), 'yyyy-MM-dd HH:mm');
}

export function formatRelativeTime(dateStr: string): string {
	return formatDistanceToNow(parseUTCDate(dateStr), { addSuffix: true });
}

export function formatShortDate(dateStr: string): string {
	return format(parseUTCDate(dateStr), 'yyyy-MM-dd');
}

export function formatFileSize(bytes: number | null | undefined): string {
	if (!bytes || bytes === 0) return '0 bytes';

	const units = ['bytes', 'KiB', 'MiB', 'GiB', 'TiB'];
	const k = 1024;
	const i = Math.floor(Math.log(bytes) / Math.log(k));
	const size = bytes / Math.pow(k, i);

	// Use toLocaleString for thousands separator and fixed decimal places
	const formattedSize = size.toLocaleString('en-US', {
		maximumFractionDigits: i === 0 ? 0 : 2
	});

	return `${formattedSize} ${units[i]}`;
}
