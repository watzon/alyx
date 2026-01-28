<script lang="ts">
  import { FieldInput } from '$lib/components/fields';
  import type { Field } from '$lib/api/client';

  let stringValue = $state('');
  let textValue = $state('');
  let numberValue = $state<number | null>(null);
  let boolValue = $state(false);
  let dateValue = $state<string | null>(null);
  let jsonValue = $state(null);
  let fileValue = $state<File | null>(null);

  const fields: Field[] = [
    { name: 'email', type: 'string', nullable: false, validate: { format: 'email', maxLength: 100 } },
    { name: 'description', type: 'text', nullable: true },
    { name: 'age', type: 'int', nullable: false, validate: { min: 0, max: 120 } },
    { name: 'published', type: 'bool', nullable: false },
    { name: 'createdAt', type: 'timestamp', nullable: false },
    { name: 'metadata', type: 'json', nullable: true },
    { name: 'avatar', type: 'file', nullable: true },
  ];
</script>

<div class="space-y-8 p-8">
  <h1 class="text-2xl font-semibold">Field Components Test</h1>

  <div class="grid grid-cols-2 gap-6">
    <FieldInput field={fields[0]} bind:value={stringValue} />
    <FieldInput field={fields[1]} bind:value={textValue} />
    <FieldInput field={fields[2]} bind:value={numberValue} />
    <FieldInput field={fields[3]} bind:value={boolValue} />
    <FieldInput field={fields[4]} bind:value={dateValue} />
    <FieldInput field={fields[5]} bind:value={jsonValue} />
    <FieldInput field={fields[6]} bind:value={fileValue} />
  </div>

  <div class="mt-8">
    <h2 class="text-lg font-semibold mb-2">Values:</h2>
    <pre class="bg-muted p-4 rounded text-xs">{JSON.stringify({
      stringValue,
      textValue,
      numberValue,
      boolValue,
      dateValue,
      jsonValue,
      fileValue: fileValue?.name,
    }, null, 2)}</pre>
  </div>
</div>
