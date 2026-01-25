import type { Schema, Collection, Field, Index, Rules, RichTextConfig } from '$lib/api/client';

/**
 * Field types supported by Alyx schema
 */
export const FIELD_TYPES = [
	'uuid',
	'string',
	'text',
	'richtext',
	'int',
	'float',
	'bool',
	'timestamp',
	'json',
	'blob',
	'email',
	'url',
	'date',
	'select',
	'relation'
] as const;

export type FieldType = (typeof FIELD_TYPES)[number];

/**
 * On-delete actions for foreign key references
 */
export const ON_DELETE_ACTIONS = ['restrict', 'cascade', 'set null'] as const;
export type OnDeleteAction = (typeof ON_DELETE_ACTIONS)[number];

/**
 * Validation formats for string fields
 */
export const VALIDATION_FORMATS = ['email', 'url', 'uuid', 'date', 'datetime'] as const;
export type ValidationFormat = (typeof VALIDATION_FORMATS)[number];

/**
 * Rich text presets
 */
export const RICHTEXT_PRESETS = ['minimal', 'basic', 'standard', 'full'] as const;
export type RichTextPreset = (typeof RICHTEXT_PRESETS)[number];

/**
 * Select field configuration
 */
export interface SelectConfig {
	values: string[];
	maxSelect?: number;
}

/**
 * Relation field configuration
 */
export interface RelationConfig {
	collection: string;
	field?: string;
	onDelete?: OnDeleteAction;
	displayName?: string;
}

/**
 * Editable version of Field with a unique ID for tracking
 */
export interface EditableField {
	_id: string;
	name: string;
	type: FieldType;
	primary?: boolean;
	unique?: boolean;
	nullable?: boolean;
	index?: boolean;
	default?: string;
	references?: string;
	onDelete?: OnDeleteAction;
	validate?: {
		minLength?: number;
		maxLength?: number;
		min?: number;
		max?: number;
		format?: ValidationFormat;
		pattern?: string;
		enum?: string[];
	};
	richtext?: RichTextConfig;
	select?: SelectConfig;
	relation?: RelationConfig;
}

/**
 * Editable version of Index with a unique ID for tracking
 */
export interface EditableIndex {
	_id: string;
	name: string;
	fields: string[];
	unique?: boolean;
}

/**
 * Editable version of Rules
 */
export interface EditableRules {
	create?: string;
	read?: string;
	update?: string;
	delete?: string;
}

/**
 * Editable version of Collection with unique ID
 */
export interface EditableCollection {
	_id: string;
	name: string;
	fields: EditableField[];
	indexes: EditableIndex[];
	rules: EditableRules;
}

/**
 * Editable version of Schema
 */
export interface EditableSchema {
	version: number;
	collections: EditableCollection[];
}

export interface SchemaValidationError {
	path: string;
	message: string;
	collectionId?: string;
	fieldId?: string;
}

const IDENTIFIER_REGEX = /^[a-z][a-z0-9_]*$/;

export function validateSchema(schema: EditableSchema): SchemaValidationError[] {
	const errors: SchemaValidationError[] = [];

	if (schema.version < 1) {
		errors.push({ path: 'version', message: 'Version must be at least 1' });
	}

	if (schema.collections.length === 0) {
		errors.push({ path: 'collections', message: 'At least one collection is required' });
	}

	for (const collection of schema.collections) {
		const colPath = `collections.${collection.name || '(unnamed)'}`;

		if (!collection.name) {
			errors.push({
				path: colPath,
				message: 'Collection name is required',
				collectionId: collection._id
			});
		} else if (!IDENTIFIER_REGEX.test(collection.name)) {
			errors.push({
				path: colPath,
				message: 'Name must start with a lowercase letter and contain only lowercase letters, numbers, and underscores',
				collectionId: collection._id
			});
		}

		if (collection.fields.length === 0) {
			errors.push({
				path: `${colPath}.fields`,
				message: 'At least one field is required',
				collectionId: collection._id
			});
		}

		const hasPrimary = collection.fields.some((f) => f.primary);
		if (!hasPrimary) {
			errors.push({
				path: `${colPath}.fields`,
				message: 'Collection must have a primary key field',
				collectionId: collection._id
			});
		}

		for (const field of collection.fields) {
			const fieldPath = `${colPath}.fields.${field.name || '(unnamed)'}`;

			if (!field.name) {
				errors.push({
					path: fieldPath,
					message: 'Field name is required',
					collectionId: collection._id,
					fieldId: field._id
				});
			} else if (!IDENTIFIER_REGEX.test(field.name)) {
				errors.push({
					path: fieldPath,
					message: 'Name must start with a lowercase letter and contain only lowercase letters, numbers, and underscores',
					collectionId: collection._id,
					fieldId: field._id
				});
			}

			if (field.primary && field.nullable) {
				errors.push({
					path: fieldPath,
					message: 'Primary key cannot be nullable',
					collectionId: collection._id,
					fieldId: field._id
				});
			}

			if (field.type === 'select' && (!field.select?.values || field.select.values.length === 0)) {
				errors.push({
					path: fieldPath,
					message: 'Select field must have at least one option value',
					collectionId: collection._id,
					fieldId: field._id
				});
			}

			if (field.type === 'relation' && !field.relation?.collection) {
				errors.push({
					path: fieldPath,
					message: 'Relation field must specify a target collection',
					collectionId: collection._id,
					fieldId: field._id
				});
			}
		}
	}

	return errors;
}

export function getFieldErrors(errors: SchemaValidationError[], fieldId: string): SchemaValidationError[] {
	return errors.filter((e) => e.fieldId === fieldId);
}

export function getCollectionErrors(errors: SchemaValidationError[], collectionId: string): SchemaValidationError[] {
	return errors.filter((e) => e.collectionId === collectionId && !e.fieldId);
}

export function generateId(): string {
	return crypto.randomUUID();
}

/**
 * Create a new empty field with defaults
 */
export function createEmptyField(): EditableField {
	return {
		_id: generateId(),
		name: '',
		type: 'string',
		nullable: true
	};
}

/**
 * Create a new empty index
 */
export function createEmptyIndex(): EditableIndex {
	return {
		_id: generateId(),
		name: '',
		fields: [],
		unique: false
	};
}

/**
 * Create a new empty collection
 */
export function createEmptyCollection(): EditableCollection {
	return {
		_id: generateId(),
		name: '',
		fields: [
			{
				_id: generateId(),
				name: 'id',
				type: 'uuid',
				primary: true,
				default: 'auto'
			}
		],
		indexes: [],
		rules: {}
	};
}

/**
 * Convert API Schema to EditableSchema
 */
export function toEditableSchema(schema: Schema): EditableSchema {
	return {
		version: schema.version,
		collections: (schema.collections ?? []).map((c) => toEditableCollection(c))
	};
}

/**
 * Convert API Collection to EditableCollection
 */
export function toEditableCollection(collection: Collection): EditableCollection {
	return {
		_id: generateId(),
		name: collection.name,
		fields: (collection.fields ?? []).map((f) => toEditableField(f)),
		indexes: (collection.indexes ?? []).map((i) => toEditableIndex(i)),
		rules: collection.rules ?? {}
	};
}

/**
 * Convert API Field to EditableField
 */
export function toEditableField(field: Field): EditableField {
	return {
		_id: generateId(),
		name: field.name,
		type: field.type as FieldType,
		primary: field.primary,
		unique: field.unique,
		nullable: field.nullable,
		index: field.index,
		default: field.default as string | undefined,
		references: field.references,
		onDelete: field.onDelete as OnDeleteAction | undefined,
		validate: field.validate as EditableField['validate'],
		richtext: field.richtext,
		select: field.select as SelectConfig | undefined,
		relation: field.relation as RelationConfig | undefined
	};
}

/**
 * Convert API Index to EditableIndex
 */
export function toEditableIndex(index: Index): EditableIndex {
	return {
		_id: generateId(),
		name: index.name,
		fields: index.fields ?? [],
		unique: index.unique
	};
}

/**
 * Convert EditableSchema to YAML string
 */
export function toYamlString(schema: EditableSchema): string {
	const lines: string[] = [];
	lines.push(`version: ${schema.version}`);
	lines.push('');
	lines.push('collections:');

	for (const collection of schema.collections) {
		lines.push(`  ${collection.name}:`);
		lines.push('    fields:');

		for (const field of collection.fields) {
			lines.push(`      ${field.name}:`);
			lines.push(`        type: ${field.type}`);

			if (field.primary) lines.push('        primary: true');
			if (field.unique) lines.push('        unique: true');
			if (field.nullable) lines.push('        nullable: true');
			if (field.index) lines.push('        index: true');
			if (field.default) lines.push(`        default: ${field.default}`);
			if (field.references) lines.push(`        references: ${field.references}`);
			if (field.onDelete) lines.push(`        onDelete: ${field.onDelete}`);

			if (field.validate) {
				const v = field.validate;
				const hasValidation =
					v.minLength !== undefined ||
					v.maxLength !== undefined ||
					v.min !== undefined ||
					v.max !== undefined ||
					v.format ||
					v.pattern ||
					(v.enum && v.enum.length > 0);

				if (hasValidation) {
					lines.push('        validate:');
					if (v.minLength !== undefined) lines.push(`          minLength: ${v.minLength}`);
					if (v.maxLength !== undefined) lines.push(`          maxLength: ${v.maxLength}`);
					if (v.min !== undefined) lines.push(`          min: ${v.min}`);
					if (v.max !== undefined) lines.push(`          max: ${v.max}`);
					if (v.format) lines.push(`          format: ${v.format}`);
					if (v.pattern) lines.push(`          pattern: "${v.pattern}"`);
					if (v.enum && v.enum.length > 0) {
						lines.push('          enum:');
						for (const e of v.enum) {
							lines.push(`            - ${e}`);
						}
					}
				}
			}

			if (field.richtext) {
				const rt = field.richtext;
				lines.push('        richtext:');
				if (rt.preset) lines.push(`          preset: ${rt.preset}`);
				if (rt.allow && rt.allow.length > 0) {
					lines.push('          allow:');
					for (const a of rt.allow) {
						lines.push(`            - ${a}`);
					}
				}
				if (rt.deny && rt.deny.length > 0) {
					lines.push('          deny:');
					for (const d of rt.deny) {
						lines.push(`            - ${d}`);
					}
				}
			}

			if (field.select) {
				const s = field.select;
				lines.push('        select:');
				if (s.values && s.values.length > 0) {
					lines.push('          values:');
					for (const v of s.values) {
						lines.push(`            - ${v}`);
					}
				}
				if (s.maxSelect !== undefined) lines.push(`          maxSelect: ${s.maxSelect}`);
			}

			if (field.relation) {
				const r = field.relation;
				lines.push('        relation:');
				lines.push(`          collection: ${r.collection}`);
				if (r.field) lines.push(`          field: ${r.field}`);
				if (r.onDelete) lines.push(`          onDelete: ${r.onDelete}`);
				if (r.displayName) lines.push(`          displayName: ${r.displayName}`);
			}
		}

		if (collection.indexes.length > 0) {
			lines.push('    indexes:');
			for (const index of collection.indexes) {
				lines.push(`      - name: ${index.name}`);
				lines.push(`        fields: [${index.fields.join(', ')}]`);
				if (index.unique) lines.push('        unique: true');
			}
		}

		const hasRules =
			collection.rules.create ||
			collection.rules.read ||
			collection.rules.update ||
			collection.rules.delete;

		if (hasRules) {
			lines.push('    rules:');
			if (collection.rules.create) lines.push(`      create: "${collection.rules.create}"`);
			if (collection.rules.read) lines.push(`      read: "${collection.rules.read}"`);
			if (collection.rules.update) lines.push(`      update: "${collection.rules.update}"`);
			if (collection.rules.delete) lines.push(`      delete: "${collection.rules.delete}"`);
		}

		lines.push('');
	}

	return lines.join('\n');
}

/**
 * Get display info for a field type
 */
export function getFieldTypeInfo(type: FieldType): { label: string; description: string } {
	const info: Record<FieldType, { label: string; description: string }> = {
		uuid: { label: 'UUID', description: 'Unique identifier (auto-generated)' },
		string: { label: 'String', description: 'Short text (up to 255 chars)' },
		text: { label: 'Text', description: 'Long text (unlimited)' },
		richtext: { label: 'Rich Text', description: 'Formatted HTML content' },
		int: { label: 'Integer', description: 'Whole number' },
		float: { label: 'Float', description: 'Decimal number' },
		bool: { label: 'Boolean', description: 'True/false value' },
		timestamp: { label: 'Timestamp', description: 'Date and time' },
		json: { label: 'JSON', description: 'Arbitrary JSON data' },
		blob: { label: 'Blob', description: 'Binary data' },
		email: { label: 'Email', description: 'Email address' },
		url: { label: 'URL', description: 'Web URL address' },
		date: { label: 'Date', description: 'Date only (YYYY-MM-DD)' },
		select: { label: 'Select', description: 'Single or multi-select from options' },
		relation: { label: 'Relation', description: 'Link to another collection' }
	};
	return info[type];
}

/**
 * Check if a field type supports validation options
 */
export function supportsValidation(type: FieldType): {
	minLength: boolean;
	maxLength: boolean;
	min: boolean;
	max: boolean;
	format: boolean;
	pattern: boolean;
	enum: boolean;
} {
	switch (type) {
		case 'string':
		case 'text':
			return {
				minLength: true,
				maxLength: true,
				min: false,
				max: false,
				format: true,
				pattern: true,
				enum: true
			};
		case 'email':
		case 'url':
			return {
				minLength: true,
				maxLength: true,
				min: false,
				max: false,
				format: false,
				pattern: true,
				enum: false
			};
		case 'int':
		case 'float':
			return {
				minLength: false,
				maxLength: false,
				min: true,
				max: true,
				format: false,
				pattern: false,
				enum: true
			};
		default:
			return {
				minLength: false,
				maxLength: false,
				min: false,
				max: false,
				format: false,
				pattern: false,
				enum: false
			};
	}
}
