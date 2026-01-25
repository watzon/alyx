<script lang="ts">
  import { CollectionDrawer } from '$lib/components/collections';
  import { Button } from '$ui/button';
  import type { Collection } from '$lib/api/client';

  let createOpen = $state(false);
  let editOpen = $state(false);

  // Mock collection schema
  const mockCollection: Collection = {
    name: 'posts',
    fields: [
      { name: 'id', type: 'uuid', primary: true },
      { name: 'title', type: 'string', nullable: false, validate: { maxLength: 100 } },
      { name: 'content', type: 'text', nullable: true },
      { name: 'published', type: 'bool', nullable: false, default: false },
      { name: 'publishedAt', type: 'timestamp', nullable: true },
      { name: 'tags', type: 'json', nullable: true },
    ],
  };

  // Mock document for edit mode
  const mockDocument = {
    id: 'test-123',
    title: 'Sample Post',
    content: '<p>This is sample <strong>content</strong>.</p>',
    published: true,
    publishedAt: '2024-01-15T10:30:00Z',
    tags: { featured: true, category: 'tech' },
  };

  function handleSuccess() {
    console.log('Document saved successfully!');
  }
</script>

<div class="p-8 space-y-4">
  <h1 class="text-2xl font-semibold">Collection Drawer Test</h1>

  <div class="flex gap-4">
    <Button onclick={() => (createOpen = true)}>
      Create New Document
    </Button>

    <Button variant="outline" onclick={() => (editOpen = true)}>
      Edit Existing Document
    </Button>
  </div>

  <CollectionDrawer
    collection={mockCollection}
    bind:open={createOpen}
    onSuccess={handleSuccess}
  />

  <CollectionDrawer
    collection={mockCollection}
    document={mockDocument}
    bind:open={editOpen}
    onSuccess={handleSuccess}
  />
</div>
