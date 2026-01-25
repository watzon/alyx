<script lang="ts">
  import { Button } from '$lib/components/ui/button';
  import UploadIcon from 'lucide-svelte/icons/upload';
  import XIcon from 'lucide-svelte/icons/x';
  import type { Field } from '$lib/api/client';

  interface Props {
    field: Field;
    value: File | null;
    errors?: string[];
    disabled?: boolean;
  }

  let { field, value = $bindable(), errors, disabled = false }: Props = $props();

  let fileInput = $state<HTMLInputElement>();
  let dragOver = $state(false);
  let previewUrl = $state<string | null>(null);

  function handleFileSelect(event: Event) {
    const target = event.target as HTMLInputElement;
    if (target.files?.[0]) {
      setFile(target.files[0]);
    }
  }

  function handleDrop(event: DragEvent) {
    event.preventDefault();
    dragOver = false;
    if (event.dataTransfer?.files[0]) {
      setFile(event.dataTransfer.files[0]);
    }
  }

  function setFile(file: File) {
    value = file;
    if (file.type.startsWith('image/')) {
      previewUrl = URL.createObjectURL(file);
    } else {
      previewUrl = null;
    }
  }

  function clearFile() {
    value = null;
    previewUrl = null;
    if (fileInput) fileInput.value = '';
  }
</script>

<div>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="border-2 border-dashed rounded-lg p-6 text-center transition-colors {dragOver ? 'border-primary bg-primary/5' : 'border-border'} {errors?.length ? 'border-destructive' : ''}"
    ondragover={(e) => {
      e.preventDefault();
      dragOver = true;
    }}
    ondragleave={() => (dragOver = false)}
    ondrop={handleDrop}
  >
    {#if value}
      <div class="space-y-2">
        {#if previewUrl}
          <img src={previewUrl} alt="Preview" class="max-h-48 mx-auto rounded" />
        {/if}
        <p class="text-sm font-medium">{value.name}</p>
        <p class="text-xs text-muted-foreground">{(value.size / 1024).toFixed(1)} KB</p>
        <Button variant="outline" size="sm" onclick={clearFile}>
          <XIcon class="h-3 w-3 mr-1" />
          Remove
        </Button>
      </div>
    {:else}
      <UploadIcon class="h-8 w-8 mx-auto mb-2 text-muted-foreground" />
      <p class="text-sm text-muted-foreground mb-2">
        Drag and drop a file, or click to browse
      </p>
      <Button variant="outline" size="sm" onclick={() => fileInput?.click()} {disabled}>
        Select File
      </Button>
      <input
        bind:this={fileInput}
        type="file"
        class="hidden"
        onchange={handleFileSelect}
        {disabled}
      />
    {/if}
  </div>

  {#if errors?.length}
    <p class="text-sm text-destructive mt-2">{errors[0]}</p>
  {/if}
</div>
