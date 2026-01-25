import { z } from 'zod';
import type { Collection } from '$lib/api/client';
import { fieldToZod } from './field-schema';

export function collectionToZod(collection: Collection): z.ZodObject<any> {
	const shape: Record<string, z.ZodType> = {};

	for (const field of collection.fields) {
		if (field.primary) continue; // Skip primary keys

		let schema = fieldToZod(field);

		// If the field has a default value, we can make it optional in the Zod schema
		// so that if it's missing from the form data, it doesn't fail validation.
		// Alternatively, we could use .default(field.default) but that might try to send the default value
		// to the API, whereas the API might handle defaults itself.
		// Usually, for a create form, if there's a default, the user doesn't *have* to provide it.
		if (field.default !== undefined) {
			schema = schema.optional().or(schema); 
            // .optional() makes it T | undefined.
		}
        
        // If the field is nullable, fieldToZod already added .nullable()
        // If the field is NOT nullable and has NO default, it is required.

		shape[field.name] = schema;
	}

	return z.object(shape);
}
