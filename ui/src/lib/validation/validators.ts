import { z } from 'zod';

export const emailValidator = z.string().email('Invalid email address');

export const urlValidator = z.string().url('Invalid URL');

export const uuidValidator = z.string().uuid('Invalid UUID format');

export const timestampValidator = z.string().datetime({ message: 'Invalid timestamp format' });

// Reusable patterns
export const patterns = {
	email: /^[^\s@]+@[^\s@]+\.[^\s@]+$/,
	url: /^https?:\/\/.+/,
	uuid: /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
};
