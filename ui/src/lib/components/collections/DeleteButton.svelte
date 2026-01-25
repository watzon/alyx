<script lang="ts">
  import { createMutation, useQueryClient } from '@tanstack/svelte-query';
  import * as AlertDialog from '$ui/alert-dialog';
  import { Button, buttonVariants } from '$ui/button';
  import { Trash2Icon, Loader2Icon } from 'lucide-svelte';
  import { collections } from '$lib/api/client';
  import { toast } from 'svelte-sonner';

  interface Props {
    collectionName: string;
    documentId: string;
    onSuccess?: () => void;
  }

  let { collectionName, documentId, onSuccess }: Props = $props();

  const queryClient = useQueryClient();
  let open = $state(false);

  const deleteMutation = createMutation(() => ({
    mutationFn: async () => {
      const result = await collections.delete(collectionName, documentId);
      if (result.error) throw result.error;
      return result;
    },
    onSuccess: () => {
      toast.success('Document deleted successfully');
      queryClient.invalidateQueries({ queryKey: ['collections', collectionName] });
      open = false;
      onSuccess?.();
    },
    onError: (error: any) => {
      toast.error('Failed to delete document', {
        description: error.message || 'Unknown error',
      });
    },
  }));

  function handleDelete() {
    deleteMutation.mutate();
  }
</script>

<AlertDialog.Root bind:open>
  <AlertDialog.Trigger
    class={buttonVariants({ variant: "ghost", size: "icon" }) + " h-7 w-7 text-muted-foreground hover:text-destructive"}
  >
    <Trash2Icon class="h-3.5 w-3.5" />
    <span class="sr-only">Delete document</span>
  </AlertDialog.Trigger>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>Delete document?</AlertDialog.Title>
      <AlertDialog.Description>
        This action cannot be undone. This will permanently delete the document from the collection.
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel disabled={deleteMutation.isPending}>Cancel</AlertDialog.Cancel>
      <Button
        variant="destructive"
        onclick={handleDelete}
        disabled={deleteMutation.isPending}
      >
        {#if deleteMutation.isPending}
          <Loader2Icon class="mr-2 h-4 w-4 animate-spin" />
          Deleting...
        {:else}
          Delete
        {/if}
      </Button>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
