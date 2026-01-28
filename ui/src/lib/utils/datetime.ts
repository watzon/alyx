/**
 * Format a date/time value for display
 * Format: yyyy-mm-dd hh:mm AM/PM
 */
export function formatDateTime(value: unknown): string {
	if (value === null || value === undefined) return '-';
	
	try {
		let date: Date;
		if (typeof value === 'number') {
			date = new Date(value > 1e12 ? value : value * 1000);
		} else if (typeof value === 'string') {
			date = new Date(value);
		} else if (value instanceof Date) {
			date = value;
		} else {
			return '-';
		}
		
		if (isNaN(date.getTime())) return '-';
		
		const year = date.getFullYear();
		const month = String(date.getMonth() + 1).padStart(2, '0');
		const day = String(date.getDate()).padStart(2, '0');
		
		let hours = date.getHours();
		const minutes = String(date.getMinutes()).padStart(2, '0');
		const ampm = hours >= 12 ? 'PM' : 'AM';
		hours = hours % 12;
		hours = hours ? hours : 12;
		const hoursStr = String(hours).padStart(2, '0');
		
		return `${year}-${month}-${day} ${hoursStr}:${minutes} ${ampm}`;
	} catch {
		return '-';
	}
}

/**
 * Format a date value for display (date only)
 * Format: yyyy-mm-dd
 */
export function formatDate(value: unknown): string {
	if (value === null || value === undefined) return '-';
	
	try {
		let date: Date;
		if (typeof value === 'number') {
			date = new Date(value > 1e12 ? value : value * 1000);
		} else if (typeof value === 'string') {
			date = new Date(value);
		} else if (value instanceof Date) {
			date = value;
		} else {
			return '-';
		}
		
		if (isNaN(date.getTime())) return '-';
		
		const year = date.getFullYear();
		const month = String(date.getMonth() + 1).padStart(2, '0');
		const day = String(date.getDate()).padStart(2, '0');
		
		return `${year}-${month}-${day}`;
	} catch {
		return '-';
	}
}

/**
 * Parse a datetime string into components for editing
 */
export function parseDateTimeForEdit(value: string | null): { date: string; time: string } {
	if (!value || value === 'now' || value === 'CURRENT_TIMESTAMP') {
		return { date: '', time: '' };
	}
	
	try {
		const d = new Date(value);
		if (isNaN(d.getTime())) {
			return { date: '', time: '' };
		}
		
		const year = d.getFullYear();
		const month = String(d.getMonth() + 1).padStart(2, '0');
		const day = String(d.getDate()).padStart(2, '0');
		const hours = String(d.getHours()).padStart(2, '0');
		const minutes = String(d.getMinutes()).padStart(2, '0');
		
		return {
			date: `${year}-${month}-${day}`,
			time: `${hours}:${minutes}`
		};
	} catch {
		return { date: '', time: '' };
	}
}

/**
 * Combine date and time strings into ISO format
 */
export function combineDateTimeToISO(date: string, time: string): string | null {
	if (!date) return null;
	
	const timeStr = time || '00:00';
	const [hours, minutes] = timeStr.split(':').map(Number);
	
	try {
		const d = new Date(date);
		d.setHours(hours || 0, minutes || 0, 0, 0);
		return d.toISOString();
	} catch {
		return null;
	}
}
