import { writable } from 'svelte/store';

export type CarbonTheme = 'white' | 'g10' | 'g80' | 'g90' | 'g100';

interface ThemeStore {
	subscribe: typeof writable.prototype.subscribe;
	set: (theme: CarbonTheme) => void;
	toggle: () => void;
	init: () => void;
}

function createThemeStore(): ThemeStore {
	const { subscribe, set, update } = writable<CarbonTheme>('white');

	return {
		subscribe,
		set,
		toggle: () =>
			update((t) => {
				// Toggle between white and g100 (dark)
				return t === 'white' ? 'g100' : 'white';
			}),
		init: () => {
			if (typeof window !== 'undefined') {
				// Detect system preference
				const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
				set(prefersDark ? 'g100' : 'white');

				// Listen for system theme changes
				window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
					set(e.matches ? 'g100' : 'white');
				});
			}
		}
	};
}

export const themeStore = createThemeStore();
