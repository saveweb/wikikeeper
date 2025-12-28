export const APP_CONFIG = {
	apiBaseUrl: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8000',
	appName: import.meta.env.VITE_APP_NAME || 'WikiKeeper',
	version: import.meta.env.VITE_APP_VERSION || '0.1.0',
	features: {
		archiveCheck: import.meta.env.VITE_ENABLE_ARCHIVE_CHECK === 'true'
	},
	pagination: {
		defaultPageSize: 50,
		pageSizeOptions: [25, 50, 100]
	},
	charts: {
		defaultDays: 30,
		dayOptions: [7, 30, 90, 180, 365]
	}
} as const;
