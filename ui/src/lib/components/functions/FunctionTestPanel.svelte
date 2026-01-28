<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import Play from '@lucide/svelte/icons/play';
	import RotateCcw from '@lucide/svelte/icons/rotate-ccw';
	import Loader2 from '@lucide/svelte/icons/loader-2';
	import Upload from '@lucide/svelte/icons/upload';
	import X from '@lucide/svelte/icons/x';
	import FileIcon from '@lucide/svelte/icons/file';
	import { toast } from 'svelte-sonner';

	interface Props {
		functionName: string;
		sampleInput?: unknown;
		onExecute: (input: unknown, files?: File[]) => Promise<void>;
		isExecuting: boolean;
	}

	let { functionName, sampleInput, onExecute, isExecuting }: Props = $props();

	const defaultInput = $derived(
		sampleInput !== null && sampleInput !== undefined
			? JSON.stringify(sampleInput, null, 2)
			: '{}'
	);

	let inputValue = $state('');
	let parseError = $state('');
	let selectedFiles = $state<File[]>([]);
	let fileInput: HTMLInputElement;
	let lastFunctionName = $state('');

	$effect(() => {
		if (functionName !== lastFunctionName) {
			inputValue = defaultInput;
			parseError = '';
			selectedFiles = [];
			lastFunctionName = functionName;
		}
	});

	function validateJSON(json: string): boolean {
		try {
			JSON.parse(json);
			parseError = '';
			return true;
		} catch (e) {
			parseError = e instanceof Error ? e.message : 'Invalid JSON';
			return false;
		}
	}

	function handleReset() {
		inputValue = defaultInput;
		parseError = '';
		selectedFiles = [];
	}

	function handleFileSelect(e: Event) {
		const target = e.target as HTMLInputElement;
		if (target.files) {
			selectedFiles = [...selectedFiles, ...Array.from(target.files)];
		}
		target.value = '';
	}

	function removeFile(index: number) {
		selectedFiles = selectedFiles.filter((_, i) => i !== index);
	}

	function formatFileSize(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
	}

	async function handleExecute() {
		if (!validateJSON(inputValue)) {
			toast.error('Invalid JSON', { description: parseError });
			return;
		}

		try {
			const parsed = JSON.parse(inputValue);
			await onExecute(parsed, selectedFiles.length > 0 ? selectedFiles : undefined);
		} catch (e) {
			toast.error('Execution failed', {
				description: e instanceof Error ? e.message : 'Unknown error'
			});
		}
	}

	function handleKeyDown(e: KeyboardEvent) {
		if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
			e.preventDefault();
			if (!isExecuting && !parseError) {
				handleExecute();
			}
		}
	}

	$effect(() => {
		validateJSON(inputValue);
	});
</script>

<div class="flex h-full flex-col">
	<div class="flex items-center justify-between border-b px-4 py-3">
		<div class="flex items-center gap-3">
			<h3 class="text-sm font-medium">Request</h3>
			{#if parseError}
				<Badge variant="destructive" id="json-error" role="alert">Invalid JSON</Badge>
			{/if}
		</div>
		<Button variant="ghost" size="sm" class="h-7 gap-1.5" onclick={handleReset} aria-label="Reset to default input">
			<RotateCcw class="size-3.5" />
			Reset
		</Button>
	</div>

	<div class="flex-1 overflow-auto p-4">
		<textarea
			bind:value={inputValue}
			onkeydown={handleKeyDown}
			class="bg-muted/50 border-input focus-visible:ring-ring min-h-[200px] w-full resize-none rounded-md border p-3 font-mono text-sm focus-visible:ring-1 focus-visible:outline-none"
			placeholder="Enter JSON input..."
			spellcheck="false"
			aria-label="Function input JSON"
			aria-describedby={parseError ? 'json-error' : undefined}
		></textarea>

		<div class="mt-4 space-y-3">
			<div class="flex items-center gap-2">
				<span class="text-sm font-medium">Files</span>
				{#if selectedFiles.length > 0}
					<Badge variant="secondary">{selectedFiles.length}</Badge>
				{/if}
			</div>

			<input
				bind:this={fileInput}
				type="file"
				multiple
				class="hidden"
				onchange={handleFileSelect}
			/>

			{#if selectedFiles.length > 0}
				<div class="space-y-2">
					{#each selectedFiles as file, i}
						<div class="flex items-center gap-2 rounded-md border bg-muted/30 px-3 py-2 text-sm">
							<FileIcon class="size-4 shrink-0 text-muted-foreground" />
							<span class="min-w-0 flex-1 truncate">{file.name}</span>
							<span class="shrink-0 text-xs text-muted-foreground">{formatFileSize(file.size)}</span>
							<Button
								variant="ghost"
								size="sm"
								class="h-6 w-6 p-0"
								onclick={() => removeFile(i)}
								aria-label={`Remove ${file.name}`}
							>
								<X class="size-3.5" />
							</Button>
						</div>
					{/each}
				</div>
			{/if}

			<Button
				variant="outline"
				size="sm"
				class="w-full gap-2"
				onclick={() => fileInput.click()}
			>
				<Upload class="size-4" />
				Add Files
			</Button>
		</div>
	</div>

	<div class="border-t p-4">
		<Button
			class="w-full gap-2"
			disabled={isExecuting || !!parseError}
			onclick={handleExecute}
			aria-label={isExecuting ? 'Executing function...' : `Execute ${functionName}`}
		>
			{#if isExecuting}
				<Loader2 class="size-4 animate-spin" />
				Executing...
			{:else}
				<Play class="size-4" />
				Execute {functionName}
			{/if}
		</Button>
		<p class="text-muted-foreground mt-2 text-center text-xs">
			Press <kbd class="bg-muted rounded px-1 py-0.5 font-mono text-[10px]">⌘</kbd>+
			<kbd class="bg-muted rounded px-1 py-0.5 font-mono text-[10px]">Enter</kbd> to execute
		</p>
	</div>
</div>
