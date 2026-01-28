<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import Play from '@lucide/svelte/icons/play';
	import RotateCcw from '@lucide/svelte/icons/rotate-ccw';
	import Loader2 from '@lucide/svelte/icons/loader-2';
	import { toast } from 'svelte-sonner';

	interface Props {
		functionName: string;
		onExecute: (input: unknown) => Promise<void>;
		isExecuting: boolean;
	}

	let { functionName, onExecute, isExecuting }: Props = $props();

	const DEFAULT_INPUT = '{ "name": "World" }';

	let inputValue = $state(DEFAULT_INPUT);
	let parseError = $state('');

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
		inputValue = DEFAULT_INPUT;
		parseError = '';
	}

	async function handleExecute() {
		if (!validateJSON(inputValue)) {
			toast.error('Invalid JSON', { description: parseError });
			return;
		}

		try {
			const parsed = JSON.parse(inputValue);
			await onExecute(parsed);
		} catch (e) {
			toast.error('Execution failed', {
				description: e instanceof Error ? e.message : 'Unknown error'
			});
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
				<Badge variant="destructive">Invalid JSON</Badge>
			{/if}
		</div>
		<Button variant="ghost" size="sm" class="h-7 gap-1.5" onclick={handleReset}>
			<RotateCcw class="size-3.5" />
			Reset
		</Button>
	</div>

	<div class="flex-1 p-4">
		<textarea
			bind:value={inputValue}
			class="bg-muted/50 border-input focus-visible:ring-ring h-full w-full resize-none rounded-md border p-3 font-mono text-sm focus-visible:ring-1 focus-visible:outline-none"
			placeholder="Enter JSON input..."
			spellcheck="false"
		></textarea>
	</div>

	<div class="border-t p-4">
		<Button
			class="w-full gap-2"
			disabled={isExecuting || !!parseError}
			onclick={handleExecute}
		>
			{#if isExecuting}
				<Loader2 class="size-4 animate-spin" />
				Executing...
			{:else}
				<Play class="size-4" />
				Execute {functionName}
			{/if}
		</Button>
	</div>
</div>
