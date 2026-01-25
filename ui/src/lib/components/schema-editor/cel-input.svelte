<script lang="ts">
	import { Input } from '$ui/input';
	import * as Popover from '$ui/popover';
	import { admin } from '$lib/api/client';
	import { cn } from '$lib/utils';
	import { Check, X, AlertCircle, HelpCircle, ChevronRight } from 'lucide-svelte';

	interface Props {
		value: string;
		onchange: (value: string) => void;
		placeholder?: string;
		disabled?: boolean;
		label?: string;
		fieldNames?: string[];
	}

	let { value, onchange, placeholder, disabled = false, label, fieldNames = [] }: Props = $props();

	let validationState = $state<'idle' | 'validating' | 'valid' | 'invalid'>('idle');
	let validationError = $state<string | null>(null);
	let validationHints = $state<string[]>([]);
	let showAutocomplete = $state(false);
	let showHelp = $state(false);
	let inputRef = $state<HTMLInputElement | null>(null);
	let selectedIndex = $state(0);
	let debounceTimer: ReturnType<typeof setTimeout> | null = null;

	const AUTH_FIELDS = [
		{ name: 'id', type: 'string', description: 'User ID' },
		{ name: 'email', type: 'string', description: 'User email' },
		{ name: 'verified', type: 'bool', description: 'Email verified' },
		{ name: 'role', type: 'string', description: 'User role' },
		{ name: 'metadata', type: 'map', description: 'Custom metadata' }
	];

	const REQUEST_FIELDS = [
		{ name: 'method', type: 'string', description: 'HTTP method' },
		{ name: 'ip', type: 'string', description: 'Client IP' }
	];

	const celContext = $derived({
		auth: {
			description: 'Current user authentication context',
			fields: AUTH_FIELDS
		},
		doc: {
			description: 'Document being accessed',
			fields: fieldNames.map((name) => ({ name, type: 'any', description: 'Collection field' }))
		},
		request: {
			description: 'HTTP request context',
			fields: REQUEST_FIELDS
		}
	});

	const COMMON_PATTERNS = [
		{ pattern: 'true', description: 'Allow all (public)' },
		{ pattern: 'false', description: 'Deny all' },
		{ pattern: 'auth.id != ""', description: 'Require auth' },
		{ pattern: 'auth.role == "admin"', description: 'Admin only' },
		{ pattern: 'auth.id == doc.owner_id', description: 'Owner only' },
		{ pattern: 'auth.role in ["admin", "moderator"]', description: 'Multiple roles' }
	];

	type CelContextKey = 'auth' | 'doc' | 'request';
	type FieldInfo = { name: string; type: string; description: string };

	function getCurrentContext(): { variable: string; fields: FieldInfo[] } | null {
		if (!inputRef) return null;
		const cursorPos = inputRef.selectionStart || 0;
		const beforeCursor = value.slice(0, cursorPos);
		const match = beforeCursor.match(/(auth|doc|request)\.$/);
		if (!match) return null;
		const variable = match[1] as CelContextKey;
		return { variable, fields: celContext[variable].fields };
	}

	let currentContext = $derived(getCurrentContext());
	let currentFields = $derived(currentContext?.fields ?? []);

	async function validateExpression(expr: string) {
		if (!expr.trim()) {
			validationState = 'idle';
			validationError = null;
			validationHints = [];
			return;
		}

		validationState = 'validating';

		const result = await admin.validateRule(expr, fieldNames);
		if (result.error) {
			validationState = 'invalid';
			validationError = result.error.message;
			validationHints = [];
			return;
		}

		if (result.data?.valid) {
			validationState = 'valid';
			validationError = null;
			validationHints = [];
		} else {
			validationState = 'invalid';
			validationError = result.data?.error || 'Invalid expression';
			validationHints = result.data?.hints || [];
		}
	}

	function handleInput(e: Event) {
		const target = e.target as HTMLInputElement;
		const newValue = target.value;
		onchange(newValue);

		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			validateExpression(newValue);
		}, 500);

		checkForAutocomplete(newValue, target.selectionStart || 0);
	}

	function checkForAutocomplete(text: string, cursorPos: number) {
		const beforeCursor = text.slice(0, cursorPos);
		const match = beforeCursor.match(/(auth|doc|request)\.$/);
		if (match) {
			showAutocomplete = true;
			selectedIndex = 0;
		} else {
			showAutocomplete = false;
		}
	}

	function insertSuggestion(suggestion: string) {
		if (!inputRef) return;
		const cursorPos = inputRef.selectionStart || 0;
		const beforeCursor = value.slice(0, cursorPos);
		const afterCursor = value.slice(cursorPos);
		const newValue = beforeCursor + suggestion + afterCursor;
		onchange(newValue);
		showAutocomplete = false;

		setTimeout(() => {
			if (inputRef) {
				const newPos = cursorPos + suggestion.length;
				inputRef.setSelectionRange(newPos, newPos);
				inputRef.focus();
			}
		}, 0);
	}

	function insertPattern(pattern: string) {
		onchange(pattern);
		showHelp = false;
		validateExpression(pattern);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (!showAutocomplete || currentFields.length === 0) return;

		switch (e.key) {
			case 'ArrowDown':
				e.preventDefault();
				selectedIndex = (selectedIndex + 1) % currentFields.length;
				break;
			case 'ArrowUp':
				e.preventDefault();
				selectedIndex = (selectedIndex - 1 + currentFields.length) % currentFields.length;
				break;
			case 'Enter':
			case 'Tab':
				e.preventDefault();
				insertSuggestion(currentFields[selectedIndex].name);
				break;
			case 'Escape':
				e.preventDefault();
				showAutocomplete = false;
				break;
		}
	}

	function handleBlur() {
		setTimeout(() => {
			showAutocomplete = false;
		}, 150);
	}
</script>

<div class="space-y-1.5">
	{#if label}
		<div class="flex items-center justify-between">
			<span class="text-xs text-muted-foreground">{label}</span>
			<div class="flex items-center gap-1">
				{#if validationState === 'valid'}
					<Check class="h-3.5 w-3.5 text-green-500" />
				{:else if validationState === 'invalid'}
					<X class="h-3.5 w-3.5 text-destructive" />
				{/if}
				<Popover.Root bind:open={showHelp}>
					<Popover.Trigger>
						{#snippet child({ props })}
							<button
								type="button"
								class="h-5 w-5 p-0 inline-flex items-center justify-center rounded hover:bg-muted"
								{...props}
							>
								<HelpCircle class="h-3.5 w-3.5 text-muted-foreground" />
							</button>
						{/snippet}
					</Popover.Trigger>
					<Popover.Content class="w-80 p-0" align="end">
						<div class="border-b px-3 py-2 bg-muted/30">
							<h4 class="font-medium text-sm">CEL Expression Reference</h4>
						</div>
						<div class="max-h-72 overflow-y-auto">
							<div class="p-3 border-b">
								<h5 class="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">Variables</h5>
								<div class="space-y-2">
									{#each Object.entries(celContext) as [varName, ctx]}
										<div class="group">
											<div class="flex items-center gap-2">
												<code class="text-xs font-mono font-semibold text-primary">{varName}</code>
												<span class="text-[10px] text-muted-foreground">{ctx.description}</span>
											</div>
											<div class="ml-3 mt-1 flex flex-wrap gap-1">
												{#each ctx.fields as field}
													<code class="text-[10px] font-mono bg-muted px-1.5 py-0.5 rounded text-muted-foreground">.{field.name}</code>
												{/each}
											</div>
										</div>
									{/each}
								</div>
							</div>

							<div class="p-3">
								<h5 class="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">Quick Insert</h5>
								<div class="grid gap-1">
									{#each COMMON_PATTERNS as { pattern, description }}
										<button
											type="button"
											class="flex items-center justify-between gap-2 px-2 py-1.5 rounded text-left hover:bg-accent transition-colors group"
											onclick={() => insertPattern(pattern)}
										>
											<code class="text-xs font-mono truncate">{pattern}</code>
											<span class="text-[10px] text-muted-foreground shrink-0">{description}</span>
										</button>
									{/each}
								</div>
							</div>
						</div>
					</Popover.Content>
				</Popover.Root>
			</div>
		</div>
	{/if}

	<div class="relative">
		<Input
			bind:ref={inputRef}
			{value}
			{placeholder}
			{disabled}
			class={cn(
				'font-mono text-sm h-9',
				validationState === 'valid' && 'border-green-500/50 focus-visible:ring-green-500/20',
				validationState === 'invalid' && 'border-destructive/50 focus-visible:ring-destructive/20'
			)}
			oninput={handleInput}
			onkeydown={handleKeydown}
			onblur={handleBlur}
		/>

		{#if showAutocomplete && currentContext && currentFields.length > 0}
			<div
				class="absolute left-0 right-0 top-full mt-1 z-50 bg-popover border rounded-lg shadow-lg overflow-hidden"
				role="listbox"
			>
				<div class="px-3 py-1.5 border-b bg-muted/30 flex items-center justify-between">
					<span class="text-xs font-medium text-muted-foreground">{currentContext.variable}</span>
					<span class="text-[10px] text-muted-foreground/70">↑↓ navigate · ↵ select · esc close</span>
				</div>
				<div class="py-1 max-h-44 overflow-y-auto">
					{#each currentFields as field, i}
						<button
							type="button"
							role="option"
							aria-selected={i === selectedIndex}
							class={cn(
								'w-full flex items-center gap-3 px-3 py-1.5 text-left transition-colors',
								i === selectedIndex ? 'bg-accent' : 'hover:bg-accent/50'
							)}
							onclick={() => insertSuggestion(field.name)}
							onmouseenter={() => (selectedIndex = i)}
						>
							<code class="text-sm font-mono font-medium min-w-[80px]">{field.name}</code>
							<span class="text-xs text-muted-foreground">{field.description}</span>
							<span class="text-[10px] text-muted-foreground/60 ml-auto font-mono">{field.type}</span>
						</button>
					{/each}
				</div>
			</div>
		{/if}
	</div>

	{#if validationState === 'invalid' && validationError}
		<div class="flex items-start gap-1.5 text-destructive">
			<AlertCircle class="h-3.5 w-3.5 mt-0.5 shrink-0" />
			<div class="space-y-0.5">
				<p class="text-xs">{validationError}</p>
				{#if validationHints.length > 0}
					<div class="text-[11px] text-muted-foreground">
						{#each validationHints as hint}
							<p>{hint}</p>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>
