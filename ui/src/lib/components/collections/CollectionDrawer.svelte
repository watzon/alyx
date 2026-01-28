<script lang="ts">
  import { createMutation, useQueryClient } from '@tanstack/svelte-query';
  import { superForm } from 'sveltekit-superforms';
  import { zodClient } from 'sveltekit-superforms/adapters';
  import * as Sheet from '$ui/sheet';
  import { Button } from '$ui/button';
  import { Separator } from '$ui/separator';
  import { FieldInput } from '$lib/components/fields';
  import { collections, files, type Collection } from '$lib/api/client';
  import { collectionToZod } from '$lib/validation/collection-schema';
  import { toast } from 'svelte-sonner';
  import { XIcon, SaveIcon, Loader2Icon } from 'lucide-svelte';

  interface Props {
    collection: Collection;
    document?: Record<string, any> | null;
    open?: boolean;
    onSuccess?: () => void;
  }

  let { collection, document = null, open = $bindable(false), onSuccess }: Props = $props();

  const queryClient = useQueryClient();
  const isEditMode = $derived(!!document);

  // Generate Zod schema from collection fields (reactive to collection changes)
  const schema = $derived(collectionToZod(collection));

  // Get the primary key field (usually 'id')
  const primaryKeyField = $derived((collection.fields || []).find((f) => f.primary));
  
  // Get editable fields (exclude primary keys for regular fields section)
  const editableFields = $derived((collection.fields || []).filter((f) => !f.primary));

  // Compute initial form data (reactive to document/collection changes)
  const initialData = $derived.by(() => {
    if (document) {
      // Edit mode: pre-populate with document data
      return { ...document };
    } else {
      // Create mode: use default values
      const defaults: Record<string, any> = {};
      
      // Include primary key field with empty value (user can optionally set it)
      if (primaryKeyField) {
        defaults[primaryKeyField.name] = '';
      }
      
      for (const field of editableFields) {
        if (field.default !== undefined) {
          defaults[field.name] = field.default;
        } else if (field.type === 'bool') {
          defaults[field.name] = false;
        } else {
          defaults[field.name] = field.nullable ? null : '';
        }
      }
      return defaults;
    }
  });

  // Superform setup - use empty object initially, reset with initialData via $effect
  // Use getter function for validators to access current $derived schema value
  const form = superForm<Record<string, any>, Record<string, any>>(
    { data: {} as Record<string, any> },
    {
      validators: () => zodClient(schema as any),
      SPA: true,
      dataType: 'json',
      resetForm: false,
      onUpdate({ form }) {
        if (form.valid) {
          if (isEditMode && document?.id) {
            updateDocMutation.mutate({ id: document.id, data: form.data });
          } else {
            createDocMutation.mutate(form.data);
          }
        } else {
          // Validation failed - show toast with error count
          const errorCount = Object.keys(form.errors).length;
          toast.error('Validation failed', {
            description: `Please fix ${errorCount} field${errorCount === 1 ? '' : 's'} with errors`,
          });
        }
      },
      onError({ result }) {
        toast.error('Form error', {
          description: result.error?.message || 'An unexpected error occurred',
        });
      },
    }
  );

  const { form: formData, enhance, errors, submitting, reset, allErrors } = form;

  // Reset form when drawer opens or document changes
  $effect(() => {
    if (open) {
      reset({ data: initialData });
    }
  });

  async function processFileFields(data: Record<string, any>): Promise<Record<string, any>> {
    const processed = { ...data };
    const fileFields = (collection.fields || []).filter(f => f.type === 'file');
    
    for (const field of fileFields) {
      const value = processed[field.name];
      if (value instanceof File) {
        const bucket = field.file?.bucket;
        if (!bucket) {
          throw new Error(`No bucket configured for file field: ${field.name}`);
        }
        const uploadResult = await files.upload(bucket, value);
        if (uploadResult.error) {
          throw new Error(`Failed to upload file for ${field.name}: ${uploadResult.error.message}`);
        }
        processed[field.name] = uploadResult.data!.id;
      } else if (value === null || value === undefined || value === '') {
        processed[field.name] = null;
      }
    }
    
    return processed;
  }

  const createDocMutation = createMutation(() => ({
    mutationFn: async (data: Record<string, any>) => {
      const processedData = await processFileFields(data);
      const result = await collections.create(collection.name, processedData);
      if (result.error) throw result.error;
      return result.data;
    },
    onSuccess: () => {
      toast.success('Document created successfully');
      queryClient.invalidateQueries({ queryKey: ['collections', collection.name] });
      open = false;
      onSuccess?.();
    },
    onError: (error: any) => {
      toast.error('Failed to create document', {
        description: error.message || 'Unknown error',
      });
    },
  }));

  const updateDocMutation = createMutation(() => ({
    mutationFn: async ({ id, data }: { id: string; data: Record<string, any> }) => {
      const processedData = await processFileFields(data);
      const result = await collections.update(collection.name, id, processedData);
      if (result.error) throw result.error;
      return result.data;
    },
    onSuccess: () => {
      toast.success('Document updated successfully');
      queryClient.invalidateQueries({ queryKey: ['collections', collection.name] });
      open = false;
      onSuccess?.();
    },
    onError: (error: any) => {
      toast.error('Failed to update document', {
        description: error.message || 'Unknown error',
      });
    },
  }));

  const isLoading = $derived($submitting || createDocMutation.isPending || updateDocMutation.isPending);

  // Handle drawer close with unsaved changes warning
  function handleOpenChange(newOpen: boolean) {
    if (!newOpen && form.tainted && !isLoading) {
      const confirm = window.confirm('You have unsaved changes. Are you sure you want to close?');
      if (!confirm) return;
    }
    open = newOpen;
  }

  // Handle keyboard shortcuts
  function handleKeyDown(event: KeyboardEvent) {
    if ((event.metaKey || event.ctrlKey) && event.key === 's') {
      event.preventDefault();
      if (!isLoading) {
        form.submit();
      }
    }
  }
</script>

<svelte:window onkeydown={handleKeyDown} />

<Sheet.Root bind:open onOpenChange={handleOpenChange}>
  <Sheet.Content class="w-full sm:max-w-2xl lg:max-w-3xl overflow-y-auto bg-muted/30">
    <Sheet.Header class="sticky top-0 z-10 bg-muted/30 backdrop-blur-sm pb-6 px-6">
      <Sheet.Title class="text-lg font-medium">
        New {collection.name} record
      </Sheet.Title>
    </Sheet.Header>

    <form method="POST" use:enhance class="pb-24 px-6">
      <div class="space-y-3">
        {#if primaryKeyField && !isEditMode}
          {@const fieldErrors = $errors[primaryKeyField.name]}
          <FieldInput
            field={primaryKeyField}
            bind:value={$formData[primaryKeyField.name]}
            errors={fieldErrors as string[] | undefined}
            disabled={isLoading}
          />
        {:else if primaryKeyField && isEditMode}
          <div class="rounded-lg bg-background border border-border p-4">
            <div class="flex items-center gap-2 mb-3">
              <span class="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                {primaryKeyField.name}
              </span>
            </div>
            <div class="font-mono text-sm text-muted-foreground bg-muted/50 rounded px-3 py-2">
              {$formData[primaryKeyField.name] || 'N/A'}
            </div>
          </div>
        {/if}

        {#each editableFields as field (field.name)}
          {@const fieldErrors = $errors[field.name]}
          <FieldInput
            {field}
            bind:value={$formData[field.name]}
            errors={fieldErrors as string[] | undefined}
            disabled={isLoading}
          />
        {/each}
      </div>
    </form>

    <Sheet.Footer class="sticky bottom-0 z-10 bg-muted/30 backdrop-blur-sm pt-4 px-6 border-t border-border/50">
      <div class="flex items-center justify-end w-full gap-3">
        <Button
          type="button"
          variant="outline"
          onclick={() => (open = false)}
          disabled={isLoading}
          class="min-w-[100px]"
        >
          Cancel
        </Button>
        <Button 
          type="submit" 
          disabled={isLoading}
          class="min-w-[100px] bg-foreground text-background hover:bg-foreground/90"
          onclick={() => form.submit()}
        >
          {#if isLoading}
            <Loader2Icon class="mr-2 h-4 w-4 animate-spin" />
            {isEditMode ? 'Saving...' : 'Creating...'}
          {:else}
            {isEditMode ? 'Save' : 'Create'}
          {/if}
        </Button>
      </div>
    </Sheet.Footer>
  </Sheet.Content>
</Sheet.Root>
