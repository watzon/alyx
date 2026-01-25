import { config as configApi, type ServerConfig } from '$lib/api/client';

class ConfigStore {
	config = $state<ServerConfig | null>(null);
	isLoading = $state(false);
	error = $state<string | null>(null);

	async load() {
		this.isLoading = true;
		this.error = null;

		const result = await configApi.get();

		if (result.error) {
			this.error = result.error.message;
			this.isLoading = false;
			return;
		}

		this.config = result.data ?? null;
		this.isLoading = false;
	}

	get docsEnabled(): boolean {
		return this.config?.Docs?.Enabled ?? false;
	}

	get docsUrl(): string {
		return '/api/docs';
	}

	get docsUI(): string {
		return this.config?.Docs?.UI?.toLowerCase() ?? 'scalar';
	}

	getCollectionDocsUrl(collectionName: string): string | null {
		if (!this.docsEnabled) return null;

		const ui = this.docsUI;
		const base = this.docsUrl;

		switch (ui) {
			case 'scalar':
			case 'redoc':
				return `${base}#tag/${collectionName}`;
			case 'swagger':
				return `${base}#operations-tag-${collectionName}`;
			case 'stoplight':
				return `${base}#/operations/list${capitalize(collectionName)}`;
			default:
				return `${base}#tag/${collectionName}`;
		}
	}
}

function capitalize(str: string): string {
	return str.charAt(0).toUpperCase() + str.slice(1);
}

export const configStore = new ConfigStore();
