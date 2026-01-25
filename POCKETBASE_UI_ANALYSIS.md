# PocketBase Admin UI Implementation Patterns & Design System

**Analysis Date:** January 24, 2026  
**PocketBase Version:** v0.36.1  
**Repository:** [pocketbase/pocketbase](https://github.com/pocketbase/pocketbase)  
**Commit:** 9b036fb10fe2bf3c0e905417873ea93edf7de729

---

## 1. ARCHITECTURE OVERVIEW

### Tech Stack
- **Framework:** Svelte 4 + Vite
- **Styling:** SCSS with CSS variables
- **Icons:** Remixicon (ri-*)
- **Date Picker:** svelte-flatpickr
- **Code Editor:** CodeMirror v6
- **Charts:** Chart.js + Leaflet
- **Router:** svelte-spa-router

### Project Structure
```
ui/
├── src/
│   ├── components/
│   │   ├── base/              # Reusable UI components
│   │   ├── records/           # Record CRUD operations
│   │   │   └── fields/        # Field type components
│   │   ├── collections/       # Collection management
│   │   ├── settings/          # Admin settings
│   │   └── logs/              # Activity logs
│   ├── scss/                  # Design system
│   ├── stores/                # Svelte stores (state)
│   ├── utils/                 # Helpers & API client
│   └── App.svelte             # Root component
└── package.json
```

---

## 2. DESIGN SYSTEM & COLOR PALETTE

### CSS Variables (Root)
```css
/* Typography */
--baseFontFamily: 'Source Sans 3', sans-serif
--monospaceFontFamily: 'Ubuntu Mono', monospace
--iconFontFamily: 'remixicon'

/* Colors */
--txtPrimaryColor: #1a1a24        /* Main text */
--txtHintColor: #617079           /* Secondary text */
--txtDisabledColor: #a0a6ac       /* Disabled text */
--primaryColor: #1a1a24           /* Brand color */
--bodyColor: #f8f9fa              /* Page background */
--baseColor: #ffffff              /* Component background */

/* Status Colors */
--infoColor: #5499e8              /* Blue */
--successColor: #32ad84           /* Green */
--dangerColor: #e34562            /* Red */
--warningColor: #ff944d           /* Orange */

/* Spacing Scale */
--baseSpacing: 30px
--xsSpacing: 15px
--smSpacing: 20px
--lgSpacing: 50px
--xlSpacing: 60px

/* Component Dimensions */
--inputHeight: 34px
--btnHeight: 40px
--appSidebarWidth: 75px
--pageSidebarWidth: 235px

/* Timing */
--baseAnimationSpeed: 150ms
--activeAnimationSpeed: 70ms
--entranceAnimationSpeed: 250ms

/* Radius */
--baseRadius: 4px
--lgRadius: 12px
```

**Key Design Principles:**
- Minimal, professional aesthetic
- Generous whitespace
- Subtle shadows and borders
- Smooth transitions (150ms default)
- High contrast for accessibility

---

## 3. LAYOUT PATTERNS

### 3.1 Main Application Layout
**Source:** [App.svelte](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/App.svelte)

```svelte
<div class="app-layout">
  <!-- Vertical sidebar (75px wide) -->
  <aside class="app-sidebar">
    <a href="/" class="logo logo-sm">
      <img src="logo.svg" alt="PocketBase logo" />
    </a>
    
    <nav class="main-menu">
      <a href="/collections" class="menu-item">
        <i class="ri-database-2-line" />
      </a>
      <a href="/logs" class="menu-item">
        <i class="ri-line-chart-line" />
      </a>
      <a href="/settings" class="menu-item">
        <i class="ri-tools-line" />
      </a>
    </nav>
    
    <!-- User menu with dropdown -->
    <div class="thumb thumb-circle">
      <span class="initials">AB</span>
      <Toggler class="dropdown dropdown-upside">
        <!-- Dropdown items -->
      </Toggler>
    </div>
  </aside>
  
  <!-- Main content area -->
  <div class="app-body">
    <Router {routes} />
    <Toasts />
  </div>
</div>
```

**CSS Implementation:**
```scss
.app-layout {
  display: flex;
  width: 100%;
  height: 100vh;
  
  .app-sidebar {
    width: var(--appSidebarWidth);  // 75px
    padding: var(--smSpacing) 0;
    background: var(--baseColor);
    border-right: 1px solid var(--baseAlt2Color);
    display: flex;
    flex-direction: column;
    align-items: center;
  }
  
  .app-body {
    flex-grow: 1;
    min-width: 0;
    height: 100%;
  }
}
```

**Key Features:**
- Fixed vertical sidebar (icon-only navigation)
- Flex layout for responsive behavior
- Sticky positioning for header/footer
- Full-height viewport

---

### 3.2 Page Wrapper Pattern
**Source:** [PageWrapper.svelte](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/components/base/PageWrapper.svelte)

```svelte
<div class="page-wrapper {classes}" class:center-content={center}>
  <main class="page-content">
    <slot />
  </main>
  
  <footer class="page-footer">
    <slot name="footer" />
    
    {#if $superuser?.id}
      <a href={import.meta.env.PB_DOCS_URL} target="_blank">
        <i class="ri-book-open-line txt-sm" />
        <span class="txt">Docs</span>
      </a>
      <span class="delimiter">|</span>
      <a href={import.meta.env.PB_RELEASES} target="_blank">
        <span class="txt">PocketBase {import.meta.env.PB_VERSION}</span>
      </a>
    {/if}
  </footer>
</div>
```

---

## 4. SIDE DRAWER / OVERLAY PANEL IMPLEMENTATION

### 4.1 OverlayPanel Component
**Source:** [OverlayPanel.svelte](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/components/base/OverlayPanel.svelte) (275 lines)

**Key Features:**
- Slides in from right side
- Stacked z-index management for multiple panels
- Escape key to close
- Click overlay to close (optional)
- Smooth fade/fly transitions
- Scroll detection for shadow effects

```svelte
<OverlayPanel
  bind:this={recordPanel}
  class="record-preview-panel overlay-panel-lg"
  on:hide
  on:show
>
  <svelte:fragment slot="header">
    <h4><strong>{collection?.name}</strong> record preview</h4>
  </svelte:fragment>
  
  <!-- Content scrolls independently -->
  <table class="table-border preview-table">
    <!-- ... -->
  </table>
  
  <svelte:fragment slot="footer">
    <button type="button" class="btn btn-transparent">Close</button>
  </svelte:fragment>
</OverlayPanel>
```

**CSS Structure:**
```scss
.overlay-panel {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-self: flex-end;
  margin-left: auto;  // Push to right edge
  background: var(--baseColor);
  height: 100%;
  width: 580px;
  max-width: 100%;
  
  .overlay-panel-section {
    position: relative;
    width: 100%;
    padding: var(--baseSpacing);
    
    &.panel-header {
      z-index: 2;
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      column-gap: 10px;
      row-gap: var(--baseSpacing);
    }
    
    &.panel-content {
      flex-grow: 1;
      overflow-y: auto;
      overflow-y: overlay;
    }
    
    &.panel-footer {
      display: flex;
      gap: 10px;
      justify-content: flex-end;
    }
  }
  
  // Animations
  in:fly={{ duration: 150, x: 50 }}
  out:fly={{ duration: 150, x: 50 }}
}

// Size variants
.overlay-panel-lg {
  width: 580px;
}

.overlay-panel-xl {
  width: 800px;
}
```

**Scroll Detection:**
```javascript
// Adds shadow when content scrolls
contentScrollClass = "scrollable scroll-top-reached"
// Updates as user scrolls
if (panel.scrollTop == 0) {
  contentScrollClass += " scroll-top-reached"
} else if (panel.scrollTop + panel.offsetHeight == panel.scrollHeight) {
  contentScrollClass += " scroll-bottom-reached"
}
```

---

## 5. TABLE DESIGN & PATTERNS

### 5.1 Table Structure
**Source:** [_table.scss](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/scss/_table.scss) (330 lines)

```html
<div class="table-wrapper h-scroll v-scroll">
  <table class="table-border">
    <thead>
      <tr>
        <th class="bulk-select-col">
          <input type="checkbox" />
        </th>
        <th class="col-sort col-type-text">
          <div class="col-header-content">
            <span class="txt">Name</span>
            <i class="ri-arrow-up-down-line"></i>
          </div>
        </th>
        <th class="col-sort col-type-email">Email</th>
        <th class="col-type-action">Actions</th>
      </tr>
    </thead>
    <tbody>
      <tr class="row-handle">
        <td class="bulk-select-col">
          <input type="checkbox" />
        </td>
        <td class="col-type-text">John Doe</td>
        <td class="col-type-email">john@example.com</td>
        <td class="col-type-action">
          <button class="btn btn-sm">Edit</button>
        </td>
      </tr>
    </tbody>
  </table>
</div>
```

**CSS Styling:**
```scss
table {
  border-collapse: separate;
  min-width: 100%;
  
  td, th {
    vertical-align: middle;
    padding: 10px;
    border-bottom: 1px solid var(--baseAlt2Color);
    
    &:first-child {
      padding-left: 20px;
    }
    &:last-child {
      padding-right: 20px;
    }
  }
  
  th {
    color: var(--txtHintColor);
    font-weight: 600;
    height: 50px;
  }
  
  td {
    height: 56px;
    word-break: break-word;
  }
  
  // Sortable column header
  .col-sort {
    cursor: pointer;
    padding-right: 35px;
    transition: background 150ms;
    
    &:after {
      content: '\ea4c';  // Remixicon sort icon
      position: absolute;
      right: 10px;
      opacity: 0;
      transition: opacity 150ms;
    }
    
    &.sort-active:after {
      opacity: 1;
    }
    
    &:hover {
      background: var(--baseAlt1Color);
      &:after {
        opacity: 1;
      }
    }
  }
  
  // Column type specific widths
  .col-type-text {
    max-width: 300px;
  }
  .col-type-editor {
    min-width: 300px;
  }
  .col-type-json {
    font-family: var(--monospaceFontFamily);
    max-width: 300px;
  }
  .col-type-relation {
    min-width: 120px;
  }
  .col-type-action {
    width: 1% !important;
    white-space: nowrap;
    text-align: right;
  }
}

// Bordered table variant
table.table-border {
  border: 1px solid var(--baseAlt2Color);
  border-radius: var(--baseRadius);
  
  tr {
    background: var(--baseColor);
  }
  
  th {
    background: var(--baseAlt1Color);
  }
}

// Sticky headers & columns
.table-wrapper {
  thead {
    position: sticky;
    top: 0;
    z-index: 100;
  }
  
  .bulk-select-col,
  .col-type-action {
    position: sticky;
    z-index: 99;
  }
  
  .bulk-select-col {
    left: 0;
  }
  
  .col-type-action {
    right: 0;
  }
  
  // Scroll shadows
  &.h-scroll {
    .bulk-select-col {
      box-shadow: 3px 0px 5px 0px var(--shadowColor);
    }
    .col-type-action {
      box-shadow: -3px 0px 5px 0px var(--shadowColor);
    }
  }
}
```

**Row Interactions:**
```scss
tr.row-handle {
  cursor: pointer;
  user-select: none;
  
  &:hover,
  &:focus-visible {
    background: var(--baseAlt1Color);
    
    .action-col {
      color: var(--txtPrimaryColor);
      i {
        transform: translateX(3px);  // Slide icon on hover
      }
    }
  }
}
```

---

## 6. FORM FIELD PATTERNS

### 6.1 Field Wrapper Component
**Source:** [Field.svelte](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/components/base/Field.svelte)

```svelte
<Field class="form-field required" name="email" let:uniqueId>
  <label for={uniqueId}>Email</label>
  <input id={uniqueId} type="email" bind:value={email} />
  
  {#if fieldErrors.length}
    <div class="help-block help-block-error">
      <pre>{fieldErrors[0].message}</pre>
    </div>
  {/if}
</Field>
```

**Features:**
- Automatic error display
- Unique ID generation
- Error state styling
- Inline or block error messages

---

### 6.2 Field Type Components

#### Text Field
**Source:** [TextField.svelte](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/components/records/fields/TextField.svelte)

```svelte
<Field class="form-field {isRequired ? 'required' : ''}" name={field.name} let:uniqueId>
  <FieldLabel {uniqueId} {field} />
  
  <AutoExpandTextarea
    id={uniqueId}
    required={isRequired}
    placeholder={hasAutogenerate ? "Leave empty to autogenerate..." : ""}
    bind:value
  />
</Field>
```

#### JSON Field
**Source:** [JsonField.svelte](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/components/records/fields/JsonField.svelte)

```svelte
<Field class="form-field {field.required ? 'required' : ''}" name={field.name} let:uniqueId>
  <FieldLabel {uniqueId} {field}>
    <!-- Validation indicator -->
    <span class="json-state" use:tooltip={{ text: isValid ? "Valid JSON" : "Invalid JSON" }}>
      {#if isValid}
        <i class="ri-checkbox-circle-fill txt-success" />
      {:else}
        <i class="ri-error-warning-fill txt-danger" />
      {/if}
    </span>
  </FieldLabel>
  
  <!-- CodeMirror editor for JSON -->
  <svelte:component
    this={editorComponent}
    id={uniqueId}
    maxHeight="500"
    language="json"
    value={serialized}
    on:change={(e) => { value = e.detail.trim() }}
  />
</Field>
```

#### Boolean Field
**Source:** [BoolField.svelte](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/components/records/fields/BoolField.svelte)

```svelte
<Field class="form-field form-field-toggle {field.required ? 'required' : ''}" name={field.name} let:uniqueId>
  <input type="checkbox" id={uniqueId} bind:checked={value} />
  <FieldLabel {uniqueId} {field} icon={false} />
</Field>
```

**CSS for Toggle:**
```scss
.form-field-toggle {
  display: flex;
  align-items: center;
  gap: 10px;
  
  input[type="checkbox"] {
    width: 18px;
    height: 18px;
    cursor: pointer;
    accent-color: var(--primaryColor);
  }
  
  label {
    margin: 0;
    cursor: pointer;
  }
}
```

#### Date Field
Uses `svelte-flatpickr` with custom styling

#### Editor Field
Uses TinyMCE for rich text editing

#### File Field
Custom file picker with drag-and-drop

#### Relation Field
Dropdown/autocomplete for related records

---

### 6.3 Form Grid Layout
**Source:** [_grid.scss](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/scss/_grid.scss)

```html
<div class="grid">
  <div class="col-12 col-md-6">
    <Field name="firstName">
      <input type="text" />
    </Field>
  </div>
  <div class="col-12 col-md-6">
    <Field name="lastName">
      <input type="text" />
    </Field>
  </div>
</div>
```

**CSS:**
```scss
.grid {
  display: flex;
  flex-wrap: wrap;
  row-gap: var(--baseSpacing);
  margin: 0 calc(-0.5 * var(--baseSpacing));
  
  > * {
    margin: 0 calc(0.5 * var(--baseSpacing));
  }
}

// 12-column grid
.col-1 { width: calc(100% / 12 - var(--gridGap)); }
.col-2 { width: calc(200% / 12 - var(--gridGap)); }
.col-6 { width: calc(600% / 12 - var(--gridGap)); }
.col-12 { width: calc(1200% / 12 - var(--gridGap)); }

// Responsive breakpoints
@media (min-width: 768px) {
  .col-md-6 { width: calc(600% / 12 - var(--gridGap)); }
}
```

---

## 7. RECORD UPSERT PANEL

### 7.1 RecordUpsertPanel Component
**Source:** [RecordUpsertPanel.svelte](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/components/records/RecordUpsertPanel.svelte) (600+ lines)

**Key Features:**
- Create/Edit mode detection
- File upload handling
- Draft auto-save
- Tab-based interface (Form + Auth Providers)
- Unsaved changes warning
- Bulk field operations

```svelte
<OverlayPanel
  bind:this={recordPanel}
  class="record-upsert-panel {hasEditorField ? 'overlay-panel-xl' : 'overlay-panel-lg'}"
  on:hide
>
  <svelte:fragment slot="header">
    <h4>
      {#if isNew}
        <strong>New</strong> {collection?.name}
      {:else}
        <strong>Edit</strong> {collection?.name}
      {/if}
    </h4>
  </svelte:fragment>
  
  <!-- Tabs for form and auth providers -->
  <div class="tabs">
    <button class:active={activeTab === 'form'}>Form</button>
    {#if isAuthCollection}
      <button class:active={activeTab === 'providers'}>Auth Providers</button>
    {/if}
  </div>
  
  {#if activeTab === 'form'}
    <form id={formId} on:submit|preventDefault={save}>
      <!-- Regular fields -->
      {#each regularFields as field}
        <svelte:component
          this={getFieldComponent(field.type)}
          {field}
          {original}
          bind:value={record[field.name]}
        />
      {/each}
      
      <!-- Auth-specific fields -->
      {#if isAuthCollection}
        <AuthFields bind:record {original} />
      {/if}
    </form>
  {/if}
  
  <svelte:fragment slot="footer">
    <button type="button" class="btn btn-transparent" on:click={() => hide()}>
      Cancel
    </button>
    <button
      type="submit"
      form={formId}
      class="btn btn-expanded"
      disabled={!canSave || isSaving}
    >
      {isSaving ? 'Saving...' : 'Save'}
    </button>
  </svelte:fragment>
</OverlayPanel>
```

---

## 8. BUTTON STYLES & INTERACTIONS

### 8.1 Button Variants
**Source:** [_form.scss](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/scss/_form.scss) (200+ lines)

```html
<!-- Primary button -->
<button class="btn">Save</button>

<!-- Secondary button -->
<button class="btn btn-secondary">Cancel</button>

<!-- Transparent button -->
<button class="btn btn-transparent">Close</button>

<!-- Danger button -->
<button class="btn btn-danger">Delete</button>

<!-- Small button -->
<button class="btn btn-sm">Edit</button>

<!-- Expanded button (full width) -->
<button class="btn btn-expanded">Submit</button>

<!-- Circle button -->
<button class="btn btn-circle">+</button>
```

**CSS Implementation:**
```scss
.btn {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  outline: 0;
  border: 0;
  margin: 0;
  cursor: pointer;
  padding: 5px 20px;
  column-gap: 7px;
  user-select: none;
  min-width: var(--btnHeight);
  min-height: var(--btnHeight);
  font-weight: 600;
  color: #fff;
  border-radius: var(--btnRadius);
  transition: color 150ms;
  
  // Background layer for smooth transitions
  &:before {
    content: '';
    position: absolute;
    left: 0;
    top: 0;
    z-index: -1;
    width: 100%;
    height: 100%;
    border-radius: inherit;
    background: var(--primaryColor);
    transition: opacity 150ms, transform 150ms;
  }
  
  // Hover state
  &:hover,
  &:focus-visible {
    &:before {
      opacity: 0.9;
    }
  }
  
  // Active state
  &:active {
    &:before {
      opacity: 0.8;
      transition-duration: 70ms;
    }
  }
  
  // Size variants
  &.btn-sm {
    min-height: var(--smBtnHeight);
    padding: 3px 12px;
    font-size: var(--smFontSize);
  }
  
  &.btn-lg {
    min-height: var(--lgBtnHeight);
    padding: 10px 30px;
    font-size: var(--lgFontSize);
  }
  
  // Color variants
  &.btn-info {
    &:before {
      background: var(--infoColor);
    }
  }
  
  &.btn-success {
    &:before {
      background: var(--successColor);
    }
  }
  
  &.btn-danger {
    &:before {
      background: var(--dangerColor);
    }
  }
  
  // Transparent variant
  &.btn-transparent {
    color: var(--txtPrimaryColor);
    &:before {
      background: var(--baseAlt3Color);
      opacity: 0;
    }
    &:hover {
      &:before {
        opacity: 0.3;
      }
    }
  }
  
  // Expanded (full width)
  &.btn-expanded {
    flex-grow: 1;
  }
  
  // Circle button
  &.btn-circle {
    border-radius: 50%;
    padding: 0;
    min-width: var(--btnHeight);
  }
}
```

---

## 9. ANIMATIONS & TRANSITIONS

### 9.1 Transition Timings
```scss
--baseAnimationSpeed: 150ms      /* Default transitions */
--activeAnimationSpeed: 70ms     /* Click feedback */
--entranceAnimationSpeed: 250ms  /* Page/modal entrance */
```

### 9.2 Common Transitions
```svelte
<!-- Fade overlay -->
<div transition:fade={{ duration: 150, opacity: 0 }} />

<!-- Slide panel from right -->
<div in:fly={{ duration: 150, x: 50 }} out:fly={{ duration: 150, x: 50 }} />

<!-- Slide error message -->
<div transition:slide={{ duration: 150 }} />

<!-- Scale icon -->
<i transition:scale={{ duration: 150, start: 0.7 }} />
```

### 9.3 Entrance Animations
```scss
@keyframes entranceTop {
  from {
    opacity: 0;
    transform: translateY(-10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

table.table-animate {
  tr {
    animation: entranceTop 250ms;
  }
}
```

---

## 10. RESPONSIVE DESIGN

### 10.1 Breakpoints
```scss
$gridSizesMap: (
  "sm":  576px,
  "md":  768px,
  "lg":  992px,
  "xl":  1200px,
  "xxl": 1400px,
);
```

### 10.2 Responsive Grid
```html
<!-- Full width on mobile, half width on tablet+ -->
<div class="col-12 col-md-6">
  <Field name="email">
    <input type="email" />
  </Field>
</div>
```

### 10.3 Mobile Considerations
- Sidebar collapses to icon-only on small screens
- Overlay panels take full width on mobile
- Tables become scrollable horizontally
- Touch-friendly button sizes (40px minimum)

---

## 11. ACCESSIBILITY FEATURES

### 11.1 ARIA Labels
```svelte
<a
  href="/collections"
  class="menu-item"
  aria-label="Collections"
  use:link
>
  <i class="ri-database-2-line" />
</a>
```

### 11.2 Keyboard Navigation
- Escape key closes modals
- Tab navigation through form fields
- Enter to submit forms
- Arrow keys in dropdowns

### 11.3 Focus Indicators
```scss
&:focus-visible {
  background: var(--baseAlt1Color);
  outline: 2px solid var(--primaryColor);
}
```

---

## 12. REAL-WORLD USAGE EXAMPLES

### Example 1: Create Record Form
```svelte
<RecordUpsertPanel bind:this={upsertPanel} {collection}>
  <!-- Automatically renders all field types -->
  <!-- Handles file uploads, validation, drafts -->
</RecordUpsertPanel>

<!-- Trigger -->
<button on:click={() => upsertPanel.show()}>
  <i class="ri-add-line" />
  New Record
</button>
```

### Example 2: Records Table with Bulk Actions
```svelte
<RecordsList
  {collection}
  bind:sort
  bind:filter
  on:select={(e) => upsertPanel.show(e.detail)}
/>

<!-- Features: -->
<!-- - Sortable columns -->
<!-- - Filterable rows -->
<!-- - Bulk select/delete -->
<!-- - Sticky headers -->
<!-- - Horizontal scroll with shadows -->
```

### Example 3: Settings Panel
```svelte
<OverlayPanel class="overlay-panel-lg">
  <svelte:fragment slot="header">
    <h4>Application Settings</h4>
  </svelte:fragment>
  
  <div class="grid">
    <div class="col-12">
      <Field name="appName">
        <input type="text" bind:value={settings.meta.appName} />
      </Field>
    </div>
  </div>
  
  <svelte:fragment slot="footer">
    <button class="btn btn-transparent">Cancel</button>
    <button class="btn btn-expanded" on:click={save}>Save</button>
  </svelte:fragment>
</OverlayPanel>
```

---

## 13. KEY TAKEAWAYS FOR ALYX ADMIN UI

### Design Principles to Adopt
1. **Minimal, professional aesthetic** - Clean whitespace, subtle shadows
2. **Icon-based navigation** - Vertical sidebar with tooltips
3. **Right-sliding panels** - For create/edit operations
4. **Sticky table headers** - For long lists
5. **Smooth transitions** - 150ms default, 70ms for active states
6. **Type-specific field components** - Dedicated components for each field type
7. **Consistent spacing** - Use CSS variables for all spacing
8. **Accessible by default** - ARIA labels, keyboard navigation, focus indicators

### Components to Implement
- [ ] OverlayPanel (right-sliding drawer)
- [ ] Table with sticky headers and horizontal scroll
- [ ] Field wrapper with error display
- [ ] Field type components (Text, JSON, Bool, Date, etc.)
- [ ] Button variants (primary, secondary, danger, etc.)
- [ ] Form grid layout system
- [ ] Sidebar navigation
- [ ] Toast notifications
- [ ] Confirmation dialogs
- [ ] Dropdown menus

### CSS Architecture
- Use CSS variables for all colors, spacing, timing
- Implement 12-column grid system
- Create SCSS mixins for common patterns
- Use BEM naming convention
- Separate concerns: layout, components, utilities

---

## 14. GITHUB PERMALINKS

All code references in this document link to specific commits:

- **Repository:** https://github.com/pocketbase/pocketbase
- **Commit:** 9b036fb10fe2bf3c0e905417873ea93edf7de729
- **UI Directory:** https://github.com/pocketbase/pocketbase/tree/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui

### Key Files
- [App.svelte](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/App.svelte) - Root layout
- [OverlayPanel.svelte](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/components/base/OverlayPanel.svelte) - Side drawer
- [RecordUpsertPanel.svelte](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/components/records/RecordUpsertPanel.svelte) - Create/edit form
- [RecordsList.svelte](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/components/records/RecordsList.svelte) - Table component
- [_table.scss](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/scss/_table.scss) - Table styles
- [_form.scss](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/scss/_form.scss) - Form styles
- [_vars.scss](https://github.com/pocketbase/pocketbase/blob/9b036fb10fe2bf3c0e905417873ea93edf7de729/ui/src/scss/_vars.scss) - Design tokens

