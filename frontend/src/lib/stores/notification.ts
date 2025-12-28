import { writable } from 'svelte/store';

export type NotificationType = 'success' | 'error' | 'warning' | 'info';

export interface Notification {
	id: string;
	message: string;
	type: NotificationType;
	duration?: number;
}

interface NotificationState {
	notifications: Notification[];
}

const createNotificationStore = () => {
	const { subscribe, update } = writable<NotificationState>({
		notifications: []
	});

	return {
		subscribe,
		show: (message: string, type: NotificationType = 'info', duration: number = 3000) => {
			const id = Math.random().toString(36).substring(2, 9);
			const notification: Notification = { id, message, type, duration };

			update((state) => ({
				notifications: [...state.notifications, notification]
			}));

			if (duration > 0) {
				setTimeout(() => {
					update((state) => ({
						notifications: state.notifications.filter((n) => n.id !== id)
					}));
				}, duration);
			}
		},
		success: (message: string, duration?: number) => {
			// Create a temporary notification store instance to call show
			const temp = createNotificationStore();
			temp.show(message, 'success', duration);
		},
		error: (message: string, duration?: number) => {
			const temp = createNotificationStore();
			temp.show(message, 'error', duration);
		},
		warning: (message: string, duration?: number) => {
			const temp = createNotificationStore();
			temp.show(message, 'warning', duration);
		},
		info: (message: string, duration?: number) => {
			const temp = createNotificationStore();
			temp.show(message, 'info', duration);
		},
		remove: (id: string) => {
			update((state) => ({
				notifications: state.notifications.filter((n) => n.id !== id)
			}));
		},
		clear: () => {
			update((state) => ({ notifications: [] }));
		}
	};
};

export const notificationStore = createNotificationStore();
