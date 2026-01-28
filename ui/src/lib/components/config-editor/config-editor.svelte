<script lang="ts">
	import { Button } from '$ui/button';
	import * as Card from '$ui/card';
	import { Input } from '$ui/input';
	import { Label } from '$ui/label';
	import { Switch } from '$ui/switch';
	import { Badge } from '$ui/badge';
	import * as Collapsible from '$ui/collapsible';
	import * as Select from '$ui/select';
	import { YamlEditor } from '$lib/components/yaml-editor';
	import {
		DurationInput,
		SecretInput,
		ArrayInput,
		KeyValueEditor
	} from '$lib/components/config-editor';
	import {
		type ConfigSchemaResponse,
		type ConfigFieldMeta,
		type ConfigFieldType,
		type EditableConfig,
		toEditableConfig,
		toYamlString,
		getSortedSections,
		isFieldModified,
		updateFieldValue
	} from './types';
	import LayoutGridIcon from '@lucide/svelte/icons/layout-grid';
	import CodeIcon from '@lucide/svelte/icons/code';
	import ServerIcon from '@lucide/svelte/icons/server';
	import DatabaseIcon from '@lucide/svelte/icons/database';
	import ShieldIcon from '@lucide/svelte/icons/shield';
	import Code2Icon from '@lucide/svelte/icons/code-2';
	import RadioIcon from '@lucide/svelte/icons/radio';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import BugIcon from '@lucide/svelte/icons/bug';
	import BookOpenIcon from '@lucide/svelte/icons/book-open';
	import LayoutIcon from '@lucide/svelte/icons/layout';
	import HardDriveIcon from '@lucide/svelte/icons/hard-drive';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';

	interface Props {
		schema: ConfigSchemaResponse;
		onchange: (yaml: string) => void;
		disabled?: boolean;
	}

	let { schema, onchange, disabled = false }: Props = $props();

	let viewMode: 'visual' | 'yaml' = $state('visual');
	let activeSection = $state<string>('');

	// Track schema changes to reinitialize editable config
	let lastSchemaJson = $state<string>('');
	let editableConfig = $state<EditableConfig>({ sections: {}, isDirty: false });

	// Track new entry names for stringMap fields (key = "sectionKey-fieldKey")
	let newEntryNames = $state<Record<string, string>>({});

	// Initialize/update editable config when schema changes
	$effect(() => {
		const schemaJson = JSON.stringify(schema.sections);
		if (schemaJson !== lastSchemaJson) {
			lastSchemaJson = schemaJson;
			editableConfig = toEditableConfig(schema);
		}
	});

	const sortedSections = $derived(getSortedSections(schema));

	// Set initial active section
	$effect(() => {
		if (sortedSections.length > 0 && !activeSection) {
			activeSection = sortedSections[0].key;
		}
	});

	const yamlPreview = $derived(toYamlString(editableConfig, schema));

	// Notify parent of changes when config changes
	$effect(() => {
		if (editableConfig.isDirty) {
			onchange(yamlPreview);
		}
	});

	function handleFieldChange(sectionKey: string, fieldKey: string, value: unknown) {
		editableConfig = updateFieldValue(editableConfig, sectionKey, fieldKey, value);
	}

	function getIconComponent(iconName: string) {
		const icons: Record<string, typeof ServerIcon> = {
			Server: ServerIcon,
			Database: DatabaseIcon,
			Shield: ShieldIcon,
			Code: Code2Icon,
			Radio: RadioIcon,
			FileText: FileTextIcon,
			Bug: BugIcon,
			BookOpen: BookOpenIcon,
			Layout: LayoutIcon,
			HardDrive: HardDriveIcon,
			Settings: SettingsIcon
		};
		return icons[iconName] || SettingsIcon;
	}
</script>


<div class="space-y-4">
  <div class="flex items-center justify-between">
    <div class="flex items-center gap-2">
      <Button
        variant={viewMode === 'visual' ? 'default' : 'outline'}
        size="sm"
        onclick={() => (viewMode = 'visual')}
      >
        <LayoutGridIcon class="h-4 w-4 mr-2" />
        Visual
      </Button>
      <Button
        variant={viewMode === 'yaml' ? 'default' : 'outline'}
        size="sm"
        onclick={() => (viewMode = 'yaml')}
      >
        <CodeIcon class="h-4 w-4 mr-2" />
        YAML Preview
      </Button>
    </div>
  </div>


  {#if viewMode === 'visual'}
    {#if sortedSections.length === 0}
      <div class="rounded-lg border border-dashed border-border p-12 text-center">
        <p class="text-muted-foreground">No configuration sections available.</p>
      </div>
    {:else}
      <div class="grid gap-3 md:grid-cols-2 lg:grid-cols-4 mb-6">
        {#each sortedSections as section (section.key)}
          {@const IconComponent = getIconComponent(section.icon)}
          <button
            class="text-left p-4 rounded-lg border transition-colors bg-muted/10 backdrop-blur-lg backdrop-saturate-150 border-border/20 hover:bg-muted/20 hover:backdrop-blur-xl hover:border-border/30 {activeSection === section.key ? '!bg-muted/30 !backdrop-blur-xl !border-border/40' : ''}"
            onclick={() => (activeSection = section.key)}
          >
            <div class="flex items-center gap-3">
              <IconComponent class="h-5 w-5 text-muted-foreground" />
              <div class="flex-1 min-w-0">
                <span class="font-medium truncate block">{section.name}</span>
                {#if section.description}
                  <span class="text-xs text-muted-foreground truncate block">{section.description}</span>
                {/if}
              </div>
            </div>
          </button>
        {/each}
      </div>


      {#if activeSection && schema.sections[activeSection]}
        {@const sectionMeta = schema.sections[activeSection]}
        {@const sectionValues = editableConfig.sections[activeSection] || {}}
        <Card.Root>
          <Card.Header>
            <Card.Title>{sectionMeta.name}</Card.Title>
            {#if sectionMeta.description}
              <Card.Description>{sectionMeta.description}</Card.Description>
            {/if}
          </Card.Header>
          <Card.Content class="space-y-6">
            {#each Object.entries(sectionMeta.fields) as [fieldKey, fieldMeta] (fieldKey)}
              {@const typedFieldMeta = fieldMeta as ConfigFieldMeta}
              {@const value = sectionValues[fieldKey]}
              {@const isModified = isFieldModified(value, typedFieldMeta)}
              {@const isRequired = typedFieldMeta.required}
              {@const isSensitive = typedFieldMeta.sensitive || typedFieldMeta.type === 'secret'}
              <div class="space-y-2">
                <div class="flex items-center justify-between">
                  <Label class="text-sm font-medium flex items-center gap-2">
                    {fieldKey}
                    {#if isRequired}
                      <span class="text-destructive">*</span>
                    {/if}
                  </Label>
                  <div class="flex items-center gap-2">
                    {#if isModified}
                      <Badge variant="secondary" class="text-xs">Modified</Badge>
                    {/if}
                  </div>
                </div>
                {#if typedFieldMeta.description}
                  <p class="text-xs text-muted-foreground">{typedFieldMeta.description}</p>
                {/if}
                <div class={isSensitive ? 'opacity-90' : ''}>

                  {#if typedFieldMeta.type === 'string'}
                    {#if typedFieldMeta.options?.length}
                      <Select.Root
                        type="single"
                        value={(value as string) || ''}
                        onValueChange={(v) => handleFieldChange(activeSection, fieldKey, v)}
                      >
                        <Select.Trigger class="w-full" disabled={disabled}>
                          {value || 'Select...'}
                        </Select.Trigger>
                        <Select.Content>
                          {#each typedFieldMeta.options as option}
                            <Select.Item value={option}>{option}</Select.Item>
                          {/each}
                        </Select.Content>
                      </Select.Root>
                    {:else}
                      <Input
                        type="text"
                        value={(value as string) || ''}
                        onchange={(e) => handleFieldChange(activeSection, fieldKey, e.currentTarget.value)}
                        {disabled}
                        placeholder={String(typedFieldMeta.default || '')}
                      />
                    {/if}
                  {:else if typedFieldMeta.type === 'secret'}
                    <SecretInput
                      value={(value as string) || ''}
                      onchange={(v) => handleFieldChange(activeSection, fieldKey, v)}
                      {disabled}
                      isSet={!!typedFieldMeta.current}
                    />
                  {:else if typedFieldMeta.type === 'int' || typedFieldMeta.type === 'int64'}
                    <Input
                      type="number"
                      value={value !== undefined ? String(value) : ''}
                      onchange={(e) => {
                        const num = parseInt(e.currentTarget.value, 10);
                        handleFieldChange(activeSection, fieldKey, isNaN(num) ? null : num);
                      }}
                      {disabled}
                      placeholder={String(typedFieldMeta.default || '0')}
                    />
                  {:else if typedFieldMeta.type === 'bool'}
                    <div class="flex items-center gap-3">
                      <Switch
                        checked={!!value}
                        onCheckedChange={(checked) => handleFieldChange(activeSection, fieldKey, checked)}
                        {disabled}
                      />
                      <span class="text-sm text-muted-foreground">{value ? 'Enabled' : 'Disabled'}</span>
                    </div>
                  {:else if typedFieldMeta.type === 'duration'}
                    <DurationInput
                      value={(value as string) || ''}
                      onchange={(v) => handleFieldChange(activeSection, fieldKey, v)}
                      {disabled}
                    />
                  {:else if typedFieldMeta.type === 'stringArray'}
                    <ArrayInput
                      values={(value as string[]) || []}
                      onchange={(v) => handleFieldChange(activeSection, fieldKey, v)}
                      {disabled}
                    />
                  {:else if typedFieldMeta.type === 'stringMap'}
                    {@const mapEntries = (value as Record<string, Record<string, unknown>>) || {}}
                    {#if typedFieldMeta.fields}
                      {@const entryKey = `${activeSection}-${fieldKey}`}
                      <div class="space-y-4">
                        {#each Object.entries(mapEntries) as [entryName, entryValue] (entryName)}
                          <Collapsible.Root>
                            <Collapsible.Trigger class="flex items-center justify-between w-full p-3 rounded-lg border bg-muted/5 hover:bg-muted/10 transition-colors">
                              <div class="flex items-center gap-2">
                                <ChevronDownIcon class="h-4 w-4" />
                                <span class="font-medium">{entryName}</span>
                              </div>
                              <Button
                                variant="ghost"
                                size="sm"
                                onclick={() => {
                                  const newEntries = { ...mapEntries };
                                  delete newEntries[entryName];
                                  handleFieldChange(activeSection, fieldKey, newEntries);
                                }}
                                {disabled}
                                class="text-destructive hover:text-destructive hover:bg-destructive/10"
                              >
                                Delete
                              </Button>
                            </Collapsible.Trigger>
                            <Collapsible.Content class="pt-4 space-y-4">
                              {#each Object.entries(typedFieldMeta.fields) as [nestedKey, nestedMeta] (nestedKey)}
                                {@const typedNestedMeta = nestedMeta as ConfigFieldMeta}
                                <div class="pl-4 border-l-2 border-border space-y-2">
                                  <Label class="text-sm">{nestedKey}</Label>
                                  {#if typedNestedMeta.description}
                                    <p class="text-xs text-muted-foreground">{typedNestedMeta.description}</p>
                                  {/if}
                                  {#if typedNestedMeta.type === 'string'}
                                    {#if typedNestedMeta.options?.length}
                                      <Select.Root
                                        type="single"
                                        value={(entryValue[nestedKey] as string) || ''}
                                        onValueChange={(v) => {
                                          const newEntries = { ...mapEntries, [entryName]: { ...entryValue, [nestedKey]: v } };
                                          handleFieldChange(activeSection, fieldKey, newEntries);
                                        }}
                                      >
                                        <Select.Trigger class="w-full" disabled={disabled}>
                                          {entryValue[nestedKey] || 'Select...'}
                                        </Select.Trigger>
                                        <Select.Content>
                                          {#each typedNestedMeta.options as option}
                                            <Select.Item value={option}>{option}</Select.Item>
                                          {/each}
                                        </Select.Content>
                                      </Select.Root>
                                    {:else}
                                      <Input
                                        type="text"
                                        value={(entryValue[nestedKey] as string) || ''}
                                        onchange={(e) => {
                                          const newEntries = { ...mapEntries, [entryName]: { ...entryValue, [nestedKey]: e.currentTarget.value } };
                                          handleFieldChange(activeSection, fieldKey, newEntries);
                                        }}
                                        {disabled}
                                      />
                                    {/if}
                                  {:else if typedNestedMeta.type === 'bool'}
                                    <Switch
                                      checked={!!entryValue[nestedKey]}
                                      onCheckedChange={(checked) => {
                                        const newEntries = { ...mapEntries, [entryName]: { ...entryValue, [nestedKey]: checked } };
                                        handleFieldChange(activeSection, fieldKey, newEntries);
                                      }}
                                      {disabled}
                                    />
                                  {:else if typedNestedMeta.type === 'secret'}
                                    <SecretInput
                                      value={(entryValue[nestedKey] as string) || ''}
                                      onchange={(v) => {
                                        const newEntries = { ...mapEntries, [entryName]: { ...entryValue, [nestedKey]: v } };
                                        handleFieldChange(activeSection, fieldKey, newEntries);
                                      }}
                                      {disabled}
                                      isSet={!!typedNestedMeta.current}
                                    />
                                  {:else if typedNestedMeta.type === 'duration'}
                                    <DurationInput
                                      value={(entryValue[nestedKey] as string) || ''}
                                      onchange={(v) => {
                                        const newEntries = { ...mapEntries, [entryName]: { ...entryValue, [nestedKey]: v } };
                                        handleFieldChange(activeSection, fieldKey, newEntries);
                                      }}
                                      {disabled}
                                    />
                                  {:else if typedNestedMeta.type === 'stringArray'}
                                    <ArrayInput
                                      values={(entryValue[nestedKey] as string[]) || []}
                                      onchange={(v) => {
                                        const newEntries = { ...mapEntries, [entryName]: { ...entryValue, [nestedKey]: v } };
                                        handleFieldChange(activeSection, fieldKey, newEntries);
                                      }}
                                      {disabled}
                                    />
                                  {:else if typedNestedMeta.type === 'int' || typedNestedMeta.type === 'int64'}
                                    <Input
                                      type="number"
                                      value={entryValue[nestedKey] !== undefined ? String(entryValue[nestedKey]) : ''}
                                      onchange={(e) => {
                                        const num = parseInt(e.currentTarget.value, 10);
                                        const newEntries = { ...mapEntries, [entryName]: { ...entryValue, [nestedKey]: isNaN(num) ? null : num } };
                                        handleFieldChange(activeSection, fieldKey, newEntries);
                                      }}
                                      {disabled}
                                      placeholder={String(typedNestedMeta.default || '0')}
                                    />
                                  {/if}
                                </div>
                              {/each}
                            </Collapsible.Content>
                          </Collapsible.Root>
                        {/each}
                        <div class="flex items-center gap-2 pt-2">
                          <Input
                            value={newEntryNames[entryKey] || ''}
                            oninput={(e) => newEntryNames[entryKey] = e.currentTarget.value}
                            placeholder="New entry name..."
                            {disabled}
                            class="flex-1"
                            onkeydown={(e) => {
                              const name = (newEntryNames[entryKey] || '').trim();
                              if (e.key === 'Enter' && name && !mapEntries[name]) {
                                const newEntries = { ...mapEntries, [name]: {} };
                                handleFieldChange(activeSection, fieldKey, newEntries);
                                newEntryNames[entryKey] = '';
                              }
                            }}
                          />
                          <Button
                            variant="outline"
                            size="sm"
                            onclick={() => {
                              const name = (newEntryNames[entryKey] || '').trim();
                              if (name && !mapEntries[name]) {
                                const newEntries = { ...mapEntries, [name]: {} };
                                handleFieldChange(activeSection, fieldKey, newEntries);
                                newEntryNames[entryKey] = '';
                              }
                            }}
                            disabled={disabled || !(newEntryNames[entryKey] || '').trim() || !!mapEntries[(newEntryNames[entryKey] || '').trim()]}
                          >
                            Add Entry
                          </Button>
                        </div>
                      </div>
                    {:else}
                      <KeyValueEditor
                        entries={(value as Record<string, string>) || {}}
                        onchange={(v) => handleFieldChange(activeSection, fieldKey, v)}
                        {disabled}
                      />
                    {/if}
                  {:else if typedFieldMeta.type === 'object'}
                    <Collapsible.Root>
                      <Collapsible.Trigger class="flex items-center gap-2 text-sm font-medium hover:text-foreground/80 transition-colors">
                        <ChevronDownIcon class="h-4 w-4" />
                        Configure Object
                      </Collapsible.Trigger>
                      <Collapsible.Content class="pt-4 space-y-4">
                        {#if typedFieldMeta.fields}
                          {@const nestedValues = (value as Record<string, unknown>) || {}}
                          {#each Object.entries(typedFieldMeta.fields) as [nestedKey, nestedMeta] (nestedKey)}
                            {@const typedNestedMeta = nestedMeta as ConfigFieldMeta}
                            <div class="pl-4 border-l-2 border-border space-y-2">
                              <Label class="text-sm">{nestedKey}</Label>
                              {#if typedNestedMeta.description}
                                <p class="text-xs text-muted-foreground">{typedNestedMeta.description}</p>
                              {/if}
                              {#if typedNestedMeta.type === 'string'}
                                {#if typedNestedMeta.options?.length}
                                  <Select.Root
                                    type="single"
                                    value={(nestedValues[nestedKey] as string) || ''}
                                    onValueChange={(v) => {
                                      const newValue = { ...nestedValues, [nestedKey]: v };
                                      handleFieldChange(activeSection, fieldKey, newValue);
                                    }}
                                  >
                                    <Select.Trigger class="w-full" disabled={disabled}>
                                      {nestedValues[nestedKey] || 'Select...'}
                                    </Select.Trigger>
                                    <Select.Content>
                                      {#each typedNestedMeta.options as option}
                                        <Select.Item value={option}>{option}</Select.Item>
                                      {/each}
                                    </Select.Content>
                                  </Select.Root>
                                {:else}
                                  <Input
                                    type="text"
                                    value={(nestedValues[nestedKey] as string) || ''}
                                    onchange={(e) => {
                                      const newValue = { ...nestedValues, [nestedKey]: e.currentTarget.value };
                                      handleFieldChange(activeSection, fieldKey, newValue);
                                    }}
                                    {disabled}
                                  />
                                {/if}
                              {:else if typedNestedMeta.type === 'bool'}
                                <Switch
                                  checked={!!nestedValues[nestedKey]}
                                  onCheckedChange={(checked) => {
                                    const newValue = { ...nestedValues, [nestedKey]: checked };
                                    handleFieldChange(activeSection, fieldKey, newValue);
                                  }}
                                  {disabled}
                                />
                              {:else if typedNestedMeta.type === 'secret'}
                                <SecretInput
                                  value={(nestedValues[nestedKey] as string) || ''}
                                  onchange={(v) => {
                                    const newValue = { ...nestedValues, [nestedKey]: v };
                                    handleFieldChange(activeSection, fieldKey, newValue);
                                  }}
                                  {disabled}
                                  isSet={!!typedNestedMeta.current}
                                />
                              {:else if typedNestedMeta.type === 'duration'}
                                <DurationInput
                                  value={(nestedValues[nestedKey] as string) || ''}
                                  onchange={(v) => {
                                    const newValue = { ...nestedValues, [nestedKey]: v };
                                    handleFieldChange(activeSection, fieldKey, newValue);
                                  }}
                                  {disabled}
                                />
                              {:else if typedNestedMeta.type === 'stringArray'}
                                <ArrayInput
                                  values={(nestedValues[nestedKey] as string[]) || []}
                                  onchange={(v) => {
                                    const newValue = { ...nestedValues, [nestedKey]: v };
                                    handleFieldChange(activeSection, fieldKey, newValue);
                                  }}
                                  {disabled}
                                />
                              {:else if typedNestedMeta.type === 'int' || typedNestedMeta.type === 'int64'}
                                <Input
                                  type="number"
                                  value={nestedValues[nestedKey] !== undefined ? String(nestedValues[nestedKey]) : ''}
                                  onchange={(e) => {
                                    const num = parseInt(e.currentTarget.value, 10);
                                    const newValue = { ...nestedValues, [nestedKey]: isNaN(num) ? null : num };
                                    handleFieldChange(activeSection, fieldKey, newValue);
                                  }}
                                  {disabled}
                                  placeholder={String(typedNestedMeta.default || '0')}
                                />
                              {/if}
                            </div>
                          {/each}
                        {/if}
                      </Collapsible.Content>
                    </Collapsible.Root>
                  {/if}
                </div>
              </div>
            {/each}
          </Card.Content>
        </Card.Root>
      {/if}
    {/if}
  {:else}
    <div class="rounded-lg border border-border">
      <div class="px-4 py-2 border-b border-border bg-muted/50">
        <p class="text-sm text-muted-foreground">This is a preview of the YAML that will be saved.</p>
      </div>
      <YamlEditor value={yamlPreview} readonly />
    </div>
  {/if}
</div>
