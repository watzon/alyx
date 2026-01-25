<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Editor, type Extensions } from '@tiptap/core';
  import StarterKit from '@tiptap/starter-kit';
  import Placeholder from '@tiptap/extension-placeholder';
  import { Button } from '$lib/components/ui/button';
  import BoldIcon from 'lucide-svelte/icons/bold';
  import ItalicIcon from 'lucide-svelte/icons/italic';
  import UnderlineIcon from 'lucide-svelte/icons/underline';
  import StrikethroughIcon from 'lucide-svelte/icons/strikethrough';
  import CodeIcon from 'lucide-svelte/icons/code';
  import ListIcon from 'lucide-svelte/icons/list';
  import ListOrderedIcon from 'lucide-svelte/icons/list-ordered';
  import QuoteIcon from 'lucide-svelte/icons/quote';
  import Heading2Icon from 'lucide-svelte/icons/heading-2';
  import LinkIcon from 'lucide-svelte/icons/link';
  import MinusIcon from 'lucide-svelte/icons/minus';
  import type { Field, RichTextConfig, RichTextFormat } from '$lib/api/client';

  interface Props {
    field: Field;
    value: string;
    errors?: string[];
    disabled?: boolean;
    richText?: boolean;
  }

  let { field, value = $bindable(), errors, disabled = false, richText = true }: Props = $props();

  let editorElement: HTMLDivElement;
  let editor: Editor | null = $state(null);

  const presetFormats: Record<string, RichTextFormat[]> = {
    minimal: ['bold', 'italic'],
    basic: ['bold', 'italic', 'link', 'bulletlist'],
    standard: ['bold', 'italic', 'underline', 'strike', 'link', 'heading', 'bulletlist', 'orderedlist', 'blockquote'],
    full: ['bold', 'italic', 'underline', 'strike', 'code', 'link', 'heading', 'blockquote', 'codeblock', 'bulletlist', 'orderedlist', 'horizontalrule'],
  };

  function getAllowedFormats(config?: RichTextConfig): Set<RichTextFormat> {
    const preset = config?.preset || 'basic';
    const baseFormats = new Set<RichTextFormat>(presetFormats[preset] || presetFormats.basic);
    
    if (config?.allow) {
      for (const f of config.allow) {
        baseFormats.add(f);
      }
    }
    
    if (config?.deny) {
      for (const f of config.deny) {
        baseFormats.delete(f);
      }
    }
    
    return baseFormats;
  }

  const allowedFormats = $derived(richText ? getAllowedFormats(field.richtext) : new Set<RichTextFormat>());

  function isAllowed(format: RichTextFormat): boolean {
    return allowedFormats.has(format);
  }

  onMount(() => {
    // StarterKit now includes Link and Underline by default
    // Configure all extensions through StarterKit.configure()
    const extensions: Extensions = [
      StarterKit.configure({
        bold: isAllowed('bold') ? {} : false,
        italic: isAllowed('italic') ? {} : false,
        strike: isAllowed('strike') ? {} : false,
        code: isAllowed('code') ? {} : false,
        heading: isAllowed('heading') ? { levels: [1, 2, 3] } : false,
        blockquote: isAllowed('blockquote') ? {} : false,
        codeBlock: isAllowed('codeblock') ? {} : false,
        bulletList: isAllowed('bulletlist') ? {} : false,
        orderedList: isAllowed('orderedlist') ? {} : false,
        horizontalRule: isAllowed('horizontalrule') ? {} : false,
        link: isAllowed('link') ? { openOnClick: false } : false,
        underline: isAllowed('underline') ? {} : false,
        // Keep listItem enabled when either list type is enabled
        listItem: (isAllowed('bulletlist') || isAllowed('orderedlist')) ? {} : false,
      }),
      Placeholder.configure({ placeholder: 'Write something...' }),
    ];

    editor = new Editor({
      element: editorElement,
      extensions,
      content: value || '',
      editable: !disabled,
      editorProps: {
        attributes: {
          class: 'prose prose-sm dark:prose-invert max-w-none flex-1 overflow-y-auto px-3 py-2 outline-none min-h-0',
        },
      },
      onUpdate: ({ editor }) => {
        value = editor.getHTML();
      },
    });
  });

  onDestroy(() => {
    editor?.destroy();
  });

  function handleCommand(e: MouseEvent, command: () => void) {
    e.preventDefault();
    command();
  }

  function toggleBold() {
    editor?.chain().focus().toggleBold().run();
  }

  function toggleItalic() {
    editor?.chain().focus().toggleItalic().run();
  }

  function toggleUnderline() {
    editor?.chain().focus().toggleUnderline().run();
  }

  function toggleStrike() {
    editor?.chain().focus().toggleStrike().run();
  }

  function toggleCode() {
    editor?.chain().focus().toggleCode().run();
  }

  function toggleHeading() {
    editor?.chain().focus().toggleHeading({ level: 2 }).run();
  }

  function toggleBlockquote() {
    editor?.chain().focus().toggleBlockquote().run();
  }

  function toggleBulletList() {
    editor?.chain().focus().toggleBulletList().run();
  }

  function toggleOrderedList() {
    editor?.chain().focus().toggleOrderedList().run();
  }

  function addHorizontalRule() {
    editor?.chain().focus().setHorizontalRule().run();
  }

  function setLink() {
    const url = window.prompt('Enter URL:');
    if (url) {
      editor?.chain().focus().setLink({ href: url }).run();
    }
  }
</script>

<div class="flex flex-col rounded-md border bg-background {errors?.length ? 'border-destructive' : ''} min-h-[200px]">
  {#if richText && !disabled && allowedFormats.size > 0}
    <div class="flex flex-wrap gap-1 border-b bg-muted/30 p-1 shrink-0">
      {#if isAllowed('bold')}
        <Button variant="ghost" size="sm" class="h-7 w-7 p-0" onmousedown={(e) => handleCommand(e, toggleBold)} title="Bold">
          <BoldIcon class="h-3.5 w-3.5" />
        </Button>
      {/if}
      {#if isAllowed('italic')}
        <Button variant="ghost" size="sm" class="h-7 w-7 p-0" onmousedown={(e) => handleCommand(e, toggleItalic)} title="Italic">
          <ItalicIcon class="h-3.5 w-3.5" />
        </Button>
      {/if}
      {#if isAllowed('underline')}
        <Button variant="ghost" size="sm" class="h-7 w-7 p-0" onmousedown={(e) => handleCommand(e, toggleUnderline)} title="Underline">
          <UnderlineIcon class="h-3.5 w-3.5" />
        </Button>
      {/if}
      {#if isAllowed('strike')}
        <Button variant="ghost" size="sm" class="h-7 w-7 p-0" onmousedown={(e) => handleCommand(e, toggleStrike)} title="Strikethrough">
          <StrikethroughIcon class="h-3.5 w-3.5" />
        </Button>
      {/if}
      {#if isAllowed('code')}
        <Button variant="ghost" size="sm" class="h-7 w-7 p-0" onmousedown={(e) => handleCommand(e, toggleCode)} title="Code">
          <CodeIcon class="h-3.5 w-3.5" />
        </Button>
      {/if}
      {#if isAllowed('heading')}
        <Button variant="ghost" size="sm" class="h-7 w-7 p-0" onmousedown={(e) => handleCommand(e, toggleHeading)} title="Heading">
          <Heading2Icon class="h-3.5 w-3.5" />
        </Button>
      {/if}
      {#if isAllowed('bulletlist')}
        <Button variant="ghost" size="sm" class="h-7 w-7 p-0" onmousedown={(e) => handleCommand(e, toggleBulletList)} title="Bullet List">
          <ListIcon class="h-3.5 w-3.5" />
        </Button>
      {/if}
      {#if isAllowed('orderedlist')}
        <Button variant="ghost" size="sm" class="h-7 w-7 p-0" onmousedown={(e) => handleCommand(e, toggleOrderedList)} title="Numbered List">
          <ListOrderedIcon class="h-3.5 w-3.5" />
        </Button>
      {/if}
      {#if isAllowed('blockquote')}
        <Button variant="ghost" size="sm" class="h-7 w-7 p-0" onmousedown={(e) => handleCommand(e, toggleBlockquote)} title="Quote">
          <QuoteIcon class="h-3.5 w-3.5" />
        </Button>
      {/if}
      {#if isAllowed('link')}
        <Button variant="ghost" size="sm" class="h-7 w-7 p-0" onmousedown={(e) => handleCommand(e, setLink)} title="Link">
          <LinkIcon class="h-3.5 w-3.5" />
        </Button>
      {/if}
      {#if isAllowed('horizontalrule')}
        <Button variant="ghost" size="sm" class="h-7 w-7 p-0" onmousedown={(e) => handleCommand(e, addHorizontalRule)} title="Horizontal Rule">
          <MinusIcon class="h-3.5 w-3.5" />
        </Button>
      {/if}
    </div>
  {/if}

  <div
    bind:this={editorElement}
    class="flex flex-col flex-1 min-h-0 focus-within:ring-2 focus-within:ring-ring focus-within:ring-inset rounded-b-md"
  ></div>
</div>

{#if errors?.length}
  <p class="text-sm text-destructive mt-2">{errors[0]}</p>
{/if}

<style>
  :global(.ProseMirror) {
    flex: 1;
    min-height: 0;
    height: 100%;
  }
  
  :global(.ProseMirror p.is-editor-empty:first-child::before) {
    color: hsl(var(--muted-foreground));
    content: attr(data-placeholder);
    float: left;
    height: 0;
    pointer-events: none;
    opacity: 0.5;
  }
  
  :global(.ProseMirror.is-editor-empty::before) {
    color: hsl(var(--muted-foreground));
    content: attr(data-placeholder);
    float: left;
    height: 0;
    pointer-events: none;
    opacity: 0.5;
  }

  /* List styles */
  :global(.ProseMirror ul) {
    list-style-type: disc;
    padding-left: 1.5rem;
    margin: 0.5rem 0;
  }

  :global(.ProseMirror ol) {
    list-style-type: decimal;
    padding-left: 1.5rem;
    margin: 0.5rem 0;
  }

  :global(.ProseMirror li) {
    margin: 0.25rem 0;
  }

  :global(.ProseMirror li p) {
    margin: 0;
  }

  /* Heading styles */
  :global(.ProseMirror h1) {
    font-size: 1.875rem;
    font-weight: 700;
    margin: 1rem 0 0.5rem;
    line-height: 1.2;
  }

  :global(.ProseMirror h2) {
    font-size: 1.5rem;
    font-weight: 600;
    margin: 0.875rem 0 0.5rem;
    line-height: 1.25;
  }

  :global(.ProseMirror h3) {
    font-size: 1.25rem;
    font-weight: 600;
    margin: 0.75rem 0 0.5rem;
    line-height: 1.3;
  }

  /* Blockquote styles */
  :global(.ProseMirror blockquote) {
    border-left: 3px solid var(--border);
    padding-left: 1rem;
    margin: 0.75rem 0;
    color: var(--muted-foreground);
    font-style: italic;
  }

  /* Horizontal rule */
  :global(.ProseMirror hr) {
    border: none;
    border-top: 1px solid var(--border);
    margin: 1rem 0;
  }

  /* Code block */
  :global(.ProseMirror pre) {
    background: var(--muted);
    border-radius: 0.375rem;
    padding: 0.75rem 1rem;
    margin: 0.75rem 0;
    overflow-x: auto;
    font-family: ui-monospace, monospace;
    font-size: 0.875rem;
  }

  :global(.ProseMirror pre code) {
    background: none;
    padding: 0;
    border-radius: 0;
    font-size: inherit;
  }

  /* Inline code */
  :global(.ProseMirror code) {
    background: var(--muted);
    padding: 0.125rem 0.25rem;
    border-radius: 0.25rem;
    font-family: ui-monospace, monospace;
    font-size: 0.875em;
  }

  /* Links */
  :global(.ProseMirror a) {
    color: var(--primary);
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  :global(.ProseMirror a:hover) {
    text-decoration-thickness: 2px;
  }
</style>
