import { z } from 'zod';
import type { Field } from '$lib/api/client';

const UUID_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const ISO_DATE_REGEX = /^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?)?$/;

function isEmpty(val: unknown): boolean {
	return val === '' || val === null || val === undefined;
}

function coerceNumber(val: unknown, isInt: boolean): number | null | undefined {
	if (typeof val === 'number') return val;
	if (typeof val === 'string') {
		const trimmed = val.trim();
		if (trimmed === '') return null;
		const parsed = isInt ? parseInt(trimmed, 10) : parseFloat(trimmed);
		if (!isNaN(parsed)) return parsed;
	}
	return undefined;
}

export function fieldToZod(field: Field): z.ZodType {
	const isNullable = field.nullable === true;
	const hasDefault = field.default !== undefined;
	const isOptional = isNullable || hasDefault;

	switch (field.type) {
		case 'uuid':
			return z.preprocess(
				(val) => (isEmpty(val) ? '' : val),
				z.string().refine(
					(val) => val === '' || UUID_REGEX.test(val),
					{ message: 'Must be a valid UUID or empty for auto-generation' }
				)
			);

		case 'string':
		case 'text':
		case 'richtext': {
			let schema = z.string();
			
			if (field.validate) {
				const v = field.validate;
				if (typeof v.minLength === 'number') schema = schema.min(v.minLength);
				if (typeof v.maxLength === 'number') schema = schema.max(v.maxLength);
				if (typeof v.pattern === 'string') schema = schema.regex(new RegExp(v.pattern));
				if (v.format === 'email') schema = schema.email('Invalid email address');
				if (v.format === 'url') schema = schema.url('Invalid URL');
				
				if (Array.isArray(v.enum) && v.enum.length > 0) {
					const enumValues = v.enum.map(String) as [string, ...string[]];
					return isNullable
						? z.preprocess((val) => (isEmpty(val) ? null : val), z.enum(enumValues).nullable())
						: z.enum(enumValues);
				}
			}

			if (isNullable) {
				return z.preprocess(
					(val) => (isEmpty(val) ? null : val),
					schema.nullable()
				);
			}
			
			return schema.min(1, 'This field is required');
		}

		case 'int': {
			if (isNullable) {
				return z.preprocess(
					(val) => {
						if (isEmpty(val)) return null;
						return coerceNumber(val, true);
					},
					z.number().int('Must be a whole number').nullable()
				);
			}
			
			return z.preprocess(
				(val) => coerceNumber(val, true),
				z.union([
					z.null().refine(() => false, { message: 'This field is required' }),
					z.undefined().refine(() => false, { message: 'This field is required' }),
					z.number().int('Must be a whole number')
				])
			).pipe(
				z.number().refine((val) => {
					if (field.validate) {
						if (typeof field.validate.min === 'number' && val < field.validate.min) return false;
						if (typeof field.validate.max === 'number' && val > field.validate.max) return false;
					}
					return true;
				}, {
					message: field.validate?.min !== undefined && field.validate?.max !== undefined
						? `Must be between ${field.validate.min} and ${field.validate.max}`
						: field.validate?.min !== undefined
							? `Must be at least ${field.validate.min}`
							: field.validate?.max !== undefined
								? `Must be at most ${field.validate.max}`
								: 'Invalid value'
				})
			);
		}

		case 'float': {
			if (isNullable) {
				return z.preprocess(
					(val) => {
						if (isEmpty(val)) return null;
						return coerceNumber(val, false);
					},
					z.number().nullable()
				);
			}
			
			return z.preprocess(
				(val) => coerceNumber(val, false),
				z.union([
					z.null().refine(() => false, { message: 'This field is required' }),
					z.undefined().refine(() => false, { message: 'This field is required' }),
					z.number()
				])
			).pipe(
				z.number().refine((val) => {
					if (field.validate) {
						if (typeof field.validate.min === 'number' && val < field.validate.min) return false;
						if (typeof field.validate.max === 'number' && val > field.validate.max) return false;
					}
					return true;
				}, {
					message: field.validate?.min !== undefined && field.validate?.max !== undefined
						? `Must be between ${field.validate.min} and ${field.validate.max}`
						: field.validate?.min !== undefined
							? `Must be at least ${field.validate.min}`
							: field.validate?.max !== undefined
								? `Must be at most ${field.validate.max}`
								: 'Invalid value'
				})
			);
		}

		case 'bool':
			return z.preprocess(
				(val) => {
					if (typeof val === 'boolean') return val;
					if (val === 'true' || val === '1') return true;
					if (val === 'false' || val === '0' || isEmpty(val)) return false;
					return val;
				},
				z.boolean()
			);

		case 'timestamp': {
			const validateDate = (val: string) => ISO_DATE_REGEX.test(val) || !isNaN(Date.parse(val));

			if (isOptional) {
				return z.preprocess(
					(val) => {
						if (isEmpty(val)) return null;
						if (val instanceof Date) return val.toISOString();
						return val;
					},
					z.string().refine(validateDate, { message: 'Invalid date format' }).nullable()
				);
			}

			return z.preprocess(
				(val) => {
					if (val instanceof Date) return val.toISOString();
					return val;
				},
				z.string().min(1, 'This field is required').refine(validateDate, { message: 'Invalid date format' })
			);
		}

		case 'json':
			if (isNullable) {
				return z.preprocess(
					(val) => (isEmpty(val) ? null : val),
					z.any().nullable()
				);
			}
			return z.any();

		case 'blob':
			if (isNullable) {
				return z.preprocess(
					(val) => (isEmpty(val) ? null : val),
					z.any().nullable()
				);
			}
			return z.any();

		default:
			return z.any();
	}
}
