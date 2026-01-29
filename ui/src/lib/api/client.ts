const BASE_URL = '/api';

export interface ApiError {
	code: string;
	error: string;
	message: string;
	details?: { field: string; code: string; message: string }[];
}

interface ApiResponse<T> {
	data?: T;
	error?: ApiError;
}

class ApiClient {
	private accessToken: string | null = null;
	private refreshPromise: Promise<boolean> | null = null;
	private onAuthFailure: (() => void) | null = null;
	private refreshTimer: ReturnType<typeof setTimeout> | null = null;

	setToken(token: string | null) {
		this.accessToken = token;
		if (token) {
			localStorage.setItem('alyx_access_token', token);
			// Schedule token refresh 5 minutes before it expires
			// Default: 1 hour token, refresh at 55 minutes
			this.scheduleTokenRefresh(55 * 60 * 1000);
		} else {
			localStorage.removeItem('alyx_access_token');
			this.clearRefreshTimer();
		}
	}

	getToken(): string | null {
		if (this.accessToken) return this.accessToken;
		if (typeof localStorage !== 'undefined') {
			this.accessToken = localStorage.getItem('alyx_access_token');
			if (this.accessToken && !this.refreshTimer) {
				this.scheduleTokenRefresh(55 * 60 * 1000);
			}
		}
		return this.accessToken;
	}

	getRefreshToken(): string | null {
		if (typeof localStorage !== 'undefined') {
			return localStorage.getItem('alyx_refresh_token');
		}
		return null;
	}

	setRefreshToken(token: string | null) {
		if (typeof localStorage !== 'undefined') {
			if (token) {
				localStorage.setItem('alyx_refresh_token', token);
			} else {
				localStorage.removeItem('alyx_refresh_token');
			}
		}
	}

	/**
	 * Register a callback to be invoked when authentication fails
	 * (i.e., both access token and refresh token are invalid/expired)
	 */
	onAuthenticationFailure(callback: () => void) {
		this.onAuthFailure = callback;
	}

	private scheduleTokenRefresh(delayMs: number) {
		this.clearRefreshTimer();
		this.refreshTimer = setTimeout(async () => {
			const refreshed = await this.refreshAccessToken();
			if (!refreshed) {
				this.onAuthFailure?.();
			}
		}, delayMs);
	}

	private clearRefreshTimer() {
		if (this.refreshTimer) {
			clearTimeout(this.refreshTimer);
			this.refreshTimer = null;
		}
	}

	/**
	 * Attempt to refresh the access token using the stored refresh token.
	 * Returns true if refresh succeeded, false otherwise.
	 * Uses a single promise to prevent concurrent refresh attempts.
	 */
	private async refreshAccessToken(): Promise<boolean> {
		if (this.refreshPromise) {
			return this.refreshPromise;
		}

		const refreshToken = this.getRefreshToken();
		if (!refreshToken) {
			return false;
		}

		this.refreshPromise = (async () => {
			try {
				const response = await fetch(`${BASE_URL}/auth/refresh`, {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ refresh_token: refreshToken })
				});

				if (!response.ok) {
					this.setToken(null);
					this.setRefreshToken(null);
					return false;
				}

				const data = await response.json();
				const tokens = data.tokens;
				this.setToken(tokens.access_token);
				if (tokens.refresh_token) {
					this.setRefreshToken(tokens.refresh_token);
				}
				return true;
			} catch {
				return false;
			} finally {
				this.refreshPromise = null;
			}
		})();

		return this.refreshPromise;
	}

	async request<T>(
		method: string,
		path: string,
		body?: unknown,
		isRetry = false
	): Promise<ApiResponse<T>> {
		const headers: Record<string, string> = {
			'Content-Type': 'application/json'
		};

		const token = this.getToken();
		if (token) {
			headers['Authorization'] = `Bearer ${token}`;
		}

		try {
			const response = await fetch(`${BASE_URL}${path}`, {
				method,
				headers,
				body: body ? JSON.stringify(body) : undefined
			});

			if (response.status === 401 && !isRetry && !path.startsWith('/auth/')) {
				const refreshed = await this.refreshAccessToken();
				if (refreshed) {
					return this.request<T>(method, path, body, true);
				}
				this.onAuthFailure?.();
			}

			const data = await response.json();

			if (!response.ok) {
				const apiError = data as ApiError;
				if (apiError.error && !apiError.message) {
					apiError.message = apiError.error;
				}
				return { error: apiError };
			}

			return { data: data as T };
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Network request failed';
			return {
				error: {
					code: 'NETWORK_ERROR',
					error: message,
					message: message
				}
			};
		}
	}

	get<T>(path: string) {
		return this.request<T>('GET', path);
	}

	post<T>(path: string, body?: unknown) {
		return this.request<T>('POST', path, body);
	}

	patch<T>(path: string, body?: unknown) {
		return this.request<T>('PATCH', path, body);
	}

	put<T>(path: string, body?: unknown) {
		return this.request<T>('PUT', path, body);
	}

	delete<T>(path: string) {
		return this.request<T>('DELETE', path);
	}

	async uploadFile<T>(path: string, file: File): Promise<ApiResponse<T>> {
		const url = `${BASE_URL}${path}`;
		const formData = new FormData();
		formData.append('file', file);

		const headers: Record<string, string> = {};
		const token = this.getToken();
		if (token) {
			headers['Authorization'] = `Bearer ${token}`;
		}

		try {
			const response = await fetch(url, {
				method: 'POST',
				headers,
				body: formData
			});

			const data = await response.json();

			if (!response.ok) {
				return { error: data as ApiError };
			}

			return { data: data as T };
		} catch (error) {
			return {
				error: {
					code: 'NETWORK_ERROR',
					error: 'Network error',
					message: error instanceof Error ? error.message : 'Unknown error'
				}
			};
		}
	}
}

export const api = new ApiClient();

export interface AuthTokens {
	access_token: string;
	refresh_token: string;
	expires_in: number;
}

export interface AuthResponse {
	user: User;
	tokens: AuthTokens;
}

export interface User {
	id: string;
	email: string;
	role: string;
	verified: boolean;
	created_at: string;
	updated_at: string;
	metadata?: Record<string, unknown>;
}

export interface Collection {
	name: string;
	fields: Field[];
	indexes?: Index[];
	rules?: Rules;
}

export type RichTextPreset = 'minimal' | 'basic' | 'standard' | 'full';
export type RichTextFormat =
	| 'bold'
	| 'italic'
	| 'underline'
	| 'strike'
	| 'code'
	| 'link'
	| 'heading'
	| 'blockquote'
	| 'codeblock'
	| 'bulletlist'
	| 'orderedlist'
	| 'horizontalrule';

export interface RichTextConfig {
	preset?: RichTextPreset;
	allow?: RichTextFormat[];
	deny?: RichTextFormat[];
}

export interface SelectConfig {
	values: string[];
	maxSelect?: number;
}

export interface RelationConfig {
	collection: string;
	field?: string;
	onDelete?: string;
	displayName?: string;
}

export interface FileConfig {
	bucket: string;
	maxSize?: number;
	allowedTypes?: string[];
	onDelete?: string;
}

export interface Field {
	name: string;
	type: string;
	primary?: boolean;
	unique?: boolean;
	nullable?: boolean;
	index?: boolean;
	default?: unknown;
	references?: string;
	onDelete?: string;
	validate?: Record<string, unknown>;
	richtext?: RichTextConfig;
	select?: SelectConfig;
	relation?: RelationConfig;
	file?: FileConfig;
}

export interface Index {
	name: string;
	fields: string[];
	unique?: boolean;
}

export interface Rules {
	create?: string;
	read?: string;
	update?: string;
	delete?: string;
}

export interface Bucket {
	name: string;
	backend: string;
	maxFileSize?: number;
	allowedTypes?: string[];
}

export interface BucketInput {
	name: string;
	backend: string;
	max_file_size?: number;
	max_total_size?: number;
	allowed_types?: string[];
	compression?: boolean;
}

export interface Schema {
	version: number;
	collections: Collection[];
	buckets?: Bucket[];
}

export interface Stats {
	uptime: number;
	collections: number;
	documents: number;
	users: number;
	functions: number;
}

export interface StorageBucketStats {
	bucket: string;
	fileCount: number;
	totalBytes: number;
}

export interface StorageStats {
	buckets: StorageBucketStats[];
}

export interface FunctionInfo {
	name: string;
	runtime: string;
	path: string;
	enabled: boolean;
}

export interface FunctionDetail {
	name: string;
	runtime: string;
	path: string;
	entrypoint: string;
	description?: string;
	sample_input?: unknown;
	timeout?: string;
	memory?: string;
	enabled: boolean;
	env?: Record<string, string>;
	dependencies?: string[];
}

export interface FunctionInvokeResponse {
	success: boolean;
	output?: unknown;
	error?: string;
	duration_ms?: number;
}

export interface AuthStatus {
	needs_setup: boolean;
	allow_registration: boolean;
}

export interface ServerConfig {
	Docs?: {
		Enabled: boolean;
		UI: string;
		Title?: string;
		Description?: string;
		Version?: string;
	};
	AdminUI?: {
		Enabled: boolean;
		Path: string;
	};
	Realtime?: {
		Enabled: boolean;
	};
	Functions?: {
		Enabled: boolean;
	};
	Dev?: {
		Enabled: boolean;
		Watch: boolean;
		AutoMigrate: boolean;
		AutoGenerate: boolean;
		GenerateLanguages: string[];
		GenerateOutput: string;
	};
}

export interface RequestLogEntry {
	id: string;
	timestamp: string;
	method: string;
	path: string;
	query?: string;
	status: number;
	duration: number;
	duration_ms: number;
	bytes_in: number;
	bytes_out: number;
	client_ip: string;
	user_agent?: string;
	user_id?: string;
	error?: string;
	error_code?: string;
}

export interface RequestLogListResponse {
	entries: RequestLogEntry[];
	total: number;
	limit: number;
	offset: number;
}

export interface RequestLogStats {
	capacity: number;
	count: number;
}

export const auth = {
	status: () => api.get<AuthStatus>('/auth/status'),

	login: (email: string, password: string) =>
		api.post<AuthResponse>('/auth/login', { email, password }),

	register: (email: string, password: string) =>
		api.post<AuthResponse>('/auth/register', { email, password }),

	logout: () => api.post('/auth/logout'),

	refresh: (refreshToken: string) =>
		api.post<AuthTokens>('/auth/refresh', { refresh_token: refreshToken }),

	me: () => api.get<User>('/auth/me'),

	providers: () => api.get<{ providers: string[] }>('/auth/providers')
};

export interface SchemaRaw {
	content: string;
	path: string;
}

export interface ValidateRuleResponse {
	valid: boolean;
	error?: string;
	message?: string;
	hints?: string[];
}

export interface ConfigRaw {
	content: string;
	path: string;
}

// Config schema types for visual editor
export type ConfigFieldType = 'string' | 'int' | 'int64' | 'bool' | 'duration' | 'stringArray' | 'stringMap' | 'object' | 'secret';

export interface ConfigFieldMeta {
	type: ConfigFieldType;
	description?: string;
	default?: unknown;
	current?: unknown;
	sensitive?: boolean;
	required?: boolean;
	options?: string[];
	fields?: Record<string, ConfigFieldMeta>;
}

export interface ConfigSectionMeta {
	name: string;
	description?: string;
	fields: Record<string, ConfigFieldMeta>;
}

export interface ConfigSchemaResponse {
	sections: Record<string, ConfigSectionMeta>;
	path: string;
}

export interface PendingChange {
	id: string;
	type: string;
	collection: string;
	field?: string;
	description: string;
	safe: boolean;
	created_at: string;
}

export interface PendingChangesResponse {
	pending: boolean;
	changes: PendingChange[];
	total: number;
}

export interface SchemaChange {
	Type: string;
	Collection: string;
	Field?: string;
	Description: string;
	Safe: boolean;
	RequiresManual?: boolean;
}

export interface SchemaDraftPreviewResponse {
	sessionId: string;
	valid: boolean;
	safeChanges: SchemaChange[];
	unsafeChanges: SchemaChange[];
	totalChanges: number;
}

export interface SchemaDraftApplyResponse {
	success: boolean;
	message: string;
	safeApplied: number;
	unsafeApplied: number;
}

export const admin = {
	stats: () => api.get<Stats>('/admin/stats'),

	storageStats: () => api.get<StorageStats>('/admin/storage/stats'),

	schema: () => api.get<Schema>('/admin/schema'),

	schemaRaw: {
		get: () => api.get<SchemaRaw>('/admin/schema/raw'),
		update: (content: string) =>
			api.put<{ success: boolean; message?: string }>('/admin/schema/raw', { content })
	},

	schemaDraft: {
		preview: (content: string) =>
			api.put<SchemaDraftPreviewResponse>('/admin/schema', { content }),
		apply: () =>
			api.post<SchemaDraftApplyResponse>('/admin/schema/apply'),
		cancel: () =>
			api.delete<{ success: boolean; message: string }>('/admin/schema/draft')
	},

	validateRule: (expression: string, fields?: string[]) =>
		api.post<ValidateRuleResponse>('/admin/schema/validate-rule', { expression, fields }),

	pendingChanges: {
		list: () => api.get<PendingChangesResponse>('/admin/schema/pending-changes'),
		confirm: () => api.post<{ success: boolean; message: string; applied: number }>('/admin/schema/confirm-changes'),
		cancel: () => api.post<{ success: boolean; message: string }>('/admin/schema/cancel-changes')
	},

	configRaw: {
		get: () => api.get<ConfigRaw>('/admin/config/raw'),
		update: (content: string) =>
			api.put<{ success: boolean; message?: string }>('/admin/config/raw', { content })
	},

	configSchema: {
		get: () => api.get<ConfigSchemaResponse>('/admin/config/schema')
	},

	users: {
		list: (params?: {
			limit?: number;
			offset?: number;
			sort_by?: string;
			sort_dir?: 'asc' | 'desc';
			search?: string;
			role?: string;
		}) => {
			const query = new URLSearchParams();
			if (params?.limit) query.set('limit', String(params.limit));
			if (params?.offset) query.set('offset', String(params.offset));
			if (params?.sort_by) query.set('sort_by', params.sort_by);
			if (params?.sort_dir) query.set('sort_dir', params.sort_dir);
			if (params?.search) query.set('search', params.search);
			if (params?.role) query.set('role', params.role);
			const qs = query.toString();
			return api.get<{ users: User[]; total: number }>(`/admin/users${qs ? `?${qs}` : ''}`);
		},
		get: (id: string) => api.get<User>(`/admin/users/${id}`),
		create: (data: { email: string; password: string; role?: string; verified?: boolean }) =>
			api.post<User>('/admin/users', data),
		update: (id: string, data: { email?: string; role?: string; verified?: boolean }) =>
			api.patch<User>(`/admin/users/${id}`, data),
		delete: (id: string) => api.delete<{ deleted: boolean; id: string }>(`/admin/users/${id}`),
		setPassword: (id: string, password: string) =>
			api.post<{ success: boolean }>(`/admin/users/${id}/password`, { password })
	},

	buckets: {
		list: () => api.get<{ buckets: Bucket[] }>('/admin/buckets'),
		create: (input: BucketInput) => api.post<Bucket>('/admin/buckets', input),
		update: (name: string, input: Partial<BucketInput>) =>
			api.put<Bucket>(`/admin/buckets/${name}`, input),
		delete: (name: string) =>
			api.delete<{ deleted: boolean; name: string }>(`/admin/buckets/${name}`)
	},

	tokens: {
		list: () => api.get<{ tokens: { name: string; created_at: string }[] }>('/admin/tokens'),
		create: (name: string) => api.post<{ token: string }>('/admin/tokens', { name }),
		delete: (name: string) => api.delete(`/admin/tokens/${name}`)
	},

	functions: {
		list: () => api.get<{ functions: FunctionInfo[] }>('/functions'),
		get: (name: string) => api.get<FunctionDetail>(`/functions/${name}`),
		stats: () => api.get<Record<string, unknown>>('/functions/stats'),
		invoke: (name: string, input?: unknown) =>
			api.post<FunctionInvokeResponse>(`/functions/${name}`, input),
		invokeWithFiles: async (
			name: string,
			input: Record<string, unknown>,
			files: File[]
		): Promise<ApiResponse<FunctionInvokeResponse>> => {
			const formData = new FormData();

			for (const [key, value] of Object.entries(input)) {
				formData.append(key, typeof value === 'string' ? value : JSON.stringify(value));
			}

			for (const file of files) {
				formData.append('files', file);
			}

			const headers: Record<string, string> = {};
			const token = api.getToken();
			if (token) {
				headers['Authorization'] = `Bearer ${token}`;
			}

			try {
				const response = await fetch(`${BASE_URL}/functions/${name}`, {
					method: 'POST',
					headers,
					body: formData
				});

				const data = await response.json();

				if (!response.ok) {
					return { error: data as ApiError };
				}

				return { data: data as FunctionInvokeResponse };
			} catch (error) {
				return {
					error: {
						code: 'NETWORK_ERROR',
						error: 'Network error',
						message: error instanceof Error ? error.message : 'Unknown error'
					}
				};
			}
		},
		reload: () => api.post('/functions/reload')
	},

	logs: {
		list: (params?: {
			limit?: number;
			offset?: number;
			method?: string;
			path?: string;
			exclude_path_prefix?: string;
			status?: number;
			min_status?: number;
			max_status?: number;
			user_id?: string;
			since?: string;
			until?: string;
		}) => {
			const query = new URLSearchParams();
			if (params?.limit) query.set('limit', String(params.limit));
			if (params?.offset) query.set('offset', String(params.offset));
			if (params?.method) query.set('method', params.method);
			if (params?.path) query.set('path', params.path);
			if (params?.exclude_path_prefix) query.set('exclude_path_prefix', params.exclude_path_prefix);
			if (params?.status) query.set('status', String(params.status));
			if (params?.min_status) query.set('min_status', String(params.min_status));
			if (params?.max_status) query.set('max_status', String(params.max_status));
			if (params?.user_id) query.set('user_id', params.user_id);
			if (params?.since) query.set('since', params.since);
			if (params?.until) query.set('until', params.until);
			const qs = query.toString();
			return api.get<RequestLogListResponse>(`/admin/logs${qs ? `?${qs}` : ''}`);
		},
		stats: () => api.get<RequestLogStats>('/admin/logs/stats'),
		clear: () => api.post<{ message: string }>('/admin/logs/clear')
	}
};

export const config = {
	get: () => api.get<ServerConfig>('/config')
};

export const collections = {
	list: (collection: string, params?: { filter?: string; sort?: string; page?: number; perPage?: number; search?: string }) => {
		const query = new URLSearchParams();
		if (params?.filter) query.set('filter', params.filter);
		if (params?.sort) query.set('sort', params.sort);
		if (params?.page) query.set('page', String(params.page));
		if (params?.perPage) query.set('perPage', String(params.perPage));
		if (params?.search) query.set('search', params.search);
		const qs = query.toString();
		return api.get<{ docs: Record<string, unknown>[]; total: number; limit: number; offset: number }>(
			`/collections/${collection}${qs ? `?${qs}` : ''}`
		);
	},

	get: (collection: string, id: string) =>
		api.get<Record<string, unknown>>(`/collections/${collection}/${id}`),

	create: (collection: string, data: Record<string, unknown>) =>
		api.post<Record<string, unknown>>(`/collections/${collection}`, data),

	update: (collection: string, id: string, data: Record<string, unknown>) =>
		api.patch<Record<string, unknown>>(`/collections/${collection}/${id}`, data),

	delete: (collection: string, id: string) => api.delete(`/collections/${collection}/${id}`)
};

export interface FileMetadata {
	id: string;
	bucket: string;
	name: string;
	path: string;
	mime_type: string;
	size: number;
	checksum?: string;
	compressed: boolean;
	compression_type?: string;
	original_size?: number;
	metadata?: Record<string, string>;
	version: number;
	created_at: string;
	updated_at: string;
}

export const files = {
	upload: (bucket: string, file: File) =>
		api.uploadFile<FileMetadata>(`/files/${bucket}`, file),

	get: (bucket: string, id: string) =>
		api.get<FileMetadata>(`/files/${bucket}/${id}`),

	delete: (bucket: string, id: string) =>
		api.delete(`/files/${bucket}/${id}`),

	getDownloadUrl: (bucket: string, id: string) =>
		`${BASE_URL}/files/${bucket}/${id}/download`,

	getViewUrl: (bucket: string, id: string) =>
		`${BASE_URL}/files/${bucket}/${id}/view`,

	list: (
		bucket: string,
		params?: {
			offset?: number;
			limit?: number;
			search?: string;
			mime_type?: string;
		}
	) => {
		const query = new URLSearchParams();
		if (params?.offset) query.set('offset', String(params.offset));
		if (params?.limit) query.set('limit', String(params.limit));
		if (params?.search) query.set('search', params.search);
		if (params?.mime_type) query.set('mime_type', params.mime_type);
		const qs = query.toString();
		return api.get<{
			files: FileMetadata[];
			total: number;
			offset: number;
			limit: number;
		}>(`/files/${bucket}${qs ? `?${qs}` : ''}`);
	},

	deleteBatch: (bucket: string, ids: string[]) =>
		api.request<{
			deleted: number;
			failed: Array<{ id: string; error: string }>;
		}>('DELETE', `/files/${bucket}/batch`, { ids }),

	generateSignedUrl: (bucket: string, id: string, params?: { operation?: 'download' | 'view'; expiry?: string }) =>
		api.get<{ url: string; token: string; expires_at: string }>(`/files/${bucket}/${id}/sign${params ? '?' + new URLSearchParams(params as any).toString() : ''}`)
};
