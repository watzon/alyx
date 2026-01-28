<script lang="ts">
	import CodeIcon from 'lucide-svelte/icons/code';
	import MousePointerClickIcon from 'lucide-svelte/icons/mouse-pointer-click';
</script>

<div class="flex h-full flex-col items-center justify-center text-center">
	<div class="rounded-full bg-muted p-4">
		<CodeIcon class="size-8 text-muted-foreground" />
	</div>
	<h2 class="mt-4 text-lg font-medium">Select a function</h2>
	<p class="mt-1 max-w-sm text-sm text-muted-foreground">
		Choose a function from the sidebar to view details, test it, or see its configuration.
	</p>
	<div class="mt-6 flex items-center gap-2 text-xs text-muted-foreground/70">
		<MousePointerClickIcon class="size-4" />
		<span>Click a function on the left to get started</span>
	</div>
</div>
