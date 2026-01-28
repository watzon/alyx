import type {
	ConfigFieldType,
	ConfigFieldMeta,
	ConfigSectionMeta,
	ConfigSchemaResponse
} from '$lib/api/client';

// Re-export API types for convenience
export type { ConfigFieldType, ConfigFieldMeta, ConfigSectionMeta, ConfigSchemaResponse };

/**
 * Section metadata with display info
 */
export interface ConfigSection {
	key: string; // e.g., "server", "auth"
	name: string; // e.g., "Server", "Authentication"
	description?: string;
	icon: string; // Lucide icon name
	fields: Record<string, ConfigFieldMeta>;
}

/**
 * Editable config state (tracks current values for each field)
 */
export interface EditableConfig {
	sections: Record<string, Record<string, unknown>>; // section -> field -> value
	isDirty: boolean;
}

/**
 * Section display info for navigation
 */
export const SECTION_INFO: Record<string, { icon: string; order: number }> = {
	server: { icon: 'Server', order: 1 },
	database: { icon: 'Database', order: 2 },
	auth: { icon: 'Shield', order: 3 },
	functions: { icon: 'Code', order: 4 },
	realtime: { icon: 'Radio', order: 5 },
	logging: { icon: 'FileText', order: 6 },
	dev: { icon: 'Bug', order: 7 },
	docs: { icon: 'BookOpen', order: 8 },
	admin_ui: { icon: 'Layout', order: 9 },
	storage: { icon: 'HardDrive', order: 10 }
};

/**
 * Convert API schema response to editable config
 */
export function toEditableConfig(schema: ConfigSchemaResponse): EditableConfig {
	const sections: Record<string, Record<string, unknown>> = {};

	for (const [sectionKey, sectionMeta] of Object.entries(schema.sections)) {
		sections[sectionKey] = {};
		for (const [fieldKey, fieldMeta] of Object.entries(sectionMeta.fields)) {
			// Use current value if available, otherwise use default
			sections[sectionKey][fieldKey] =
				fieldMeta.current !== undefined ? fieldMeta.current : fieldMeta.default;
		}
	}

	return {
		sections,
		isDirty: false
	};
}

/**
 * Convert editable config back to YAML string
 */
export function toYamlString(
	config: EditableConfig,
	schema: ConfigSchemaResponse
): string {
	const lines: string[] = [];

	for (const [sectionKey, sectionMeta] of Object.entries(schema.sections)) {
		lines.push(`${sectionKey}:`);

		const sectionValues = config.sections[sectionKey] || {};

		for (const [fieldKey, fieldMeta] of Object.entries(sectionMeta.fields)) {
			const value = sectionValues[fieldKey];

			if (value === undefined || value === null) {
				continue;
			}

			// Skip if value equals default
			if (JSON.stringify(value) === JSON.stringify(fieldMeta.default)) {
				continue;
			}

			const yamlValue = formatYamlValue(value, fieldMeta.type);
			lines.push(`  ${fieldKey}: ${yamlValue}`);
		}

		lines.push('');
	}

	return lines.join('\n');
}

/**
 * Format a value for YAML output
 */
function formatYamlValue(value: unknown, type: ConfigFieldType): string {
	if (value === null || value === undefined) {
		return 'null';
	}

	switch (type) {
		case 'string':
		case 'secret':
		case 'duration':
			// Quote strings that need it
			if (typeof value === 'string') {
				if (value.includes(':') || value.includes('#') || value.includes('\n')) {
					return `"${value.replace(/"/g, '\\"')}"`;
				}
				return value;
			}
			return String(value);

		case 'bool':
			return value ? 'true' : 'false';

		case 'int':
		case 'int64':
			return String(value);

		case 'stringArray':
			if (Array.isArray(value)) {
				if (value.length === 0) return '[]';
				return '\n' + value.map((v) => `    - ${v}`).join('\n');
			}
			return '[]';

		case 'stringMap':
			if (typeof value === 'object' && value !== null) {
				const entries = Object.entries(value);
				if (entries.length === 0) return '{}';
				return '\n' + entries.map(([k, v]) => `    ${k}: ${v}`).join('\n');
			}
			return '{}';

		case 'object':
			// Objects are handled recursively in the calling code
			return '{}';

		default:
			return String(value);
	}
}

/**
 * Get display info for a config field type
 */
export function getFieldTypeInfo(type: ConfigFieldType): { label: string; description: string } {
	const info: Record<ConfigFieldType, { label: string; description: string }> = {
		string: { label: 'Text', description: 'Single line text value' },
		int: { label: 'Integer', description: 'Whole number' },
		int64: { label: 'Integer (64-bit)', description: 'Large whole number' },
		bool: { label: 'Boolean', description: 'True/false value' },
		duration: { label: 'Duration', description: 'Time duration (e.g., 5m, 1h30s)' },
		stringArray: { label: 'String Array', description: 'List of text values' },
		stringMap: { label: 'Key-Value Map', description: 'Dictionary of string keys and values' },
		object: { label: 'Object', description: 'Nested configuration object' },
		secret: { label: 'Secret', description: 'Sensitive value (masked in UI)' }
	};
	return info[type];
}

/**
 * Check if a field value differs from its default
 */
export function isFieldModified(
	value: unknown,
	fieldMeta: ConfigFieldMeta
): boolean {
	if (value === undefined || value === null) {
		return fieldMeta.default !== undefined && fieldMeta.default !== null;
	}
	return JSON.stringify(value) !== JSON.stringify(fieldMeta.default);
}

/**
 * Create a new editable config from schema with all defaults
 */
export function createEmptyConfig(schema: ConfigSchemaResponse): EditableConfig {
	return toEditableConfig(schema);
}

/**
 * Update a field value in the config
 */
export function updateFieldValue(
	config: EditableConfig,
	sectionKey: string,
	fieldKey: string,
	value: unknown
): EditableConfig {
	return {
		...config,
		sections: {
			...config.sections,
			[sectionKey]: {
				...config.sections[sectionKey],
				[fieldKey]: value
			}
		},
		isDirty: true
	};
}

/**
 * Get sorted sections by order
 */
export function getSortedSections(
	schema: ConfigSchemaResponse
): Array<{ key: string; name: string; description?: string; icon: string }> {
	return Object.entries(schema.sections)
		.map(([key, meta]) => ({
			key,
			name: meta.name,
			description: meta.description,
			icon: SECTION_INFO[key]?.icon || 'Settings',
			order: SECTION_INFO[key]?.order || 999
		}))
		.sort((a, b) => a.order - b.order);
}
