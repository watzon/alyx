package schema

import (
	"fmt"
	"sync"
)

// Manager provides centralized, thread-safe schema management with CRUD operations
// for collections and buckets, validation, and change notifications.
type Manager struct {
	path     string
	schema   *Schema
	mu       sync.RWMutex
	onChange func(*Schema)
}

// NewManager creates a new schema manager for the given file path.
func NewManager(path string) *Manager {
	return &Manager{
		path: path,
	}
}

// Load reads and parses the schema from the configured file path.
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	schema, err := ParseFile(m.path)
	if err != nil {
		return fmt.Errorf("loading schema: %w", err)
	}
	m.schema = schema
	return nil
}

// Save writes the current schema to disk after validation.
// Calls the onChange callback if set.
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := Validate(m.schema); err != nil {
		return fmt.Errorf("validating schema: %w", err)
	}

	if err := WriteFile(m.path, m.schema); err != nil {
		return fmt.Errorf("saving schema: %w", err)
	}

	if m.onChange != nil {
		m.onChange(m.schema)
	}
	return nil
}

// GetSchema returns a deep copy of the current schema to prevent external modification.
func (m *Manager) GetSchema() *Schema {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.schema == nil {
		return nil
	}

	schemaCopy := &Schema{
		Version:     m.schema.Version,
		Collections: make(map[string]*Collection, len(m.schema.Collections)),
		Buckets:     make(map[string]*Bucket, len(m.schema.Buckets)),
		Functions:   make(map[string]*Function, len(m.schema.Functions)),
	}

	for name, col := range m.schema.Collections {
		colCopy := &Collection{
			Name:       col.Name,
			Fields:     make(map[string]*Field, len(col.Fields)),
			Indexes:    make([]*Index, len(col.Indexes)),
			Rules:      col.Rules,
			fieldOrder: make([]string, len(col.fieldOrder)),
		}
		for fname, field := range col.Fields {
			fieldCopy := *field
			colCopy.Fields[fname] = &fieldCopy
		}
		for i, idx := range col.Indexes {
			idxCopy := *idx
			colCopy.Indexes[i] = &idxCopy
		}
		copy(colCopy.fieldOrder, col.fieldOrder)
		schemaCopy.Collections[name] = colCopy
	}

	for name, bucket := range m.schema.Buckets {
		bucketCopy := *bucket
		if bucket.AllowedTypes != nil {
			bucketCopy.AllowedTypes = make([]string, len(bucket.AllowedTypes))
			copy(bucketCopy.AllowedTypes, bucket.AllowedTypes)
		}
		schemaCopy.Buckets[name] = &bucketCopy
	}

	for name, fn := range m.schema.Functions {
		fnCopy := *fn
		if fn.Env != nil {
			fnCopy.Env = make(map[string]string, len(fn.Env))
			for k, v := range fn.Env {
				fnCopy.Env[k] = v
			}
		}
		if fn.Dependencies != nil {
			fnCopy.Dependencies = make([]string, len(fn.Dependencies))
			copy(fnCopy.Dependencies, fn.Dependencies)
		}
		if fn.Hooks != nil {
			fnCopy.Hooks = make([]FunctionHook, len(fn.Hooks))
			copy(fnCopy.Hooks, fn.Hooks)
		}
		if fn.Schedules != nil {
			fnCopy.Schedules = make([]FunctionSchedule, len(fn.Schedules))
			copy(fnCopy.Schedules, fn.Schedules)
		}
		if fn.Routes != nil {
			fnCopy.Routes = make([]FunctionRoute, len(fn.Routes))
			copy(fnCopy.Routes, fn.Routes)
		}
		schemaCopy.Functions[name] = &fnCopy
	}

	return schemaCopy
}

// UpdateFromYAML parses raw YAML content and updates the schema.
func (m *Manager) UpdateFromYAML(content []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	schema, err := Parse(content)
	if err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}

	m.schema = schema
	return nil
}

// SetOnChange sets a callback function to be invoked after successful schema saves.
func (m *Manager) SetOnChange(callback func(*Schema)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = callback
}

// AddBucket adds a new bucket to the schema.
// Returns an error if a bucket with the same name already exists.
func (m *Manager) AddBucket(name string, bucket *Bucket) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.schema == nil {
		return fmt.Errorf("schema not loaded")
	}

	if _, exists := m.schema.Buckets[name]; exists {
		return fmt.Errorf("bucket %q already exists", name)
	}

	if m.schema.Buckets == nil {
		m.schema.Buckets = make(map[string]*Bucket)
	}

	bucket.Name = name
	m.schema.Buckets[name] = bucket

	if err := Validate(m.schema); err != nil {
		delete(m.schema.Buckets, name)
		return fmt.Errorf("validating schema: %w", err)
	}

	return nil
}

// UpdateBucket updates an existing bucket in the schema.
// Returns an error if the bucket does not exist.
func (m *Manager) UpdateBucket(name string, bucket *Bucket) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.schema == nil {
		return fmt.Errorf("schema not loaded")
	}

	if _, exists := m.schema.Buckets[name]; !exists {
		return fmt.Errorf("bucket %q not found", name)
	}

	oldBucket := m.schema.Buckets[name]
	bucket.Name = name
	m.schema.Buckets[name] = bucket

	if err := Validate(m.schema); err != nil {
		m.schema.Buckets[name] = oldBucket
		return fmt.Errorf("validating schema: %w", err)
	}

	return nil
}

// DeleteBucket removes a bucket from the schema.
// Returns an error if the bucket is referenced by any file fields.
func (m *Manager) DeleteBucket(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.schema == nil {
		return fmt.Errorf("schema not loaded")
	}

	if _, exists := m.schema.Buckets[name]; !exists {
		return fmt.Errorf("bucket %q not found", name)
	}

	// Check for references in file fields
	for colName, col := range m.schema.Collections {
		for fieldName, field := range col.Fields {
			if field.Type == FieldTypeFile && field.File != nil && field.File.Bucket == name {
				return fmt.Errorf("bucket %q is referenced by field %s.%s", name, colName, fieldName)
			}
		}
	}

	delete(m.schema.Buckets, name)
	return nil
}

// AddCollection adds a new collection to the schema.
// Returns an error if a collection with the same name already exists.
func (m *Manager) AddCollection(name string, col *Collection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.schema == nil {
		return fmt.Errorf("schema not loaded")
	}

	if _, exists := m.schema.Collections[name]; exists {
		return fmt.Errorf("collection %q already exists", name)
	}

	if m.schema.Collections == nil {
		m.schema.Collections = make(map[string]*Collection)
	}

	col.Name = name
	m.schema.Collections[name] = col

	if err := Validate(m.schema); err != nil {
		delete(m.schema.Collections, name)
		return fmt.Errorf("validating schema: %w", err)
	}

	return nil
}

// UpdateCollection updates an existing collection in the schema.
// Returns an error if the collection does not exist.
func (m *Manager) UpdateCollection(name string, col *Collection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.schema == nil {
		return fmt.Errorf("schema not loaded")
	}

	if _, exists := m.schema.Collections[name]; !exists {
		return fmt.Errorf("collection %q not found", name)
	}

	oldCol := m.schema.Collections[name]
	col.Name = name
	m.schema.Collections[name] = col

	if err := Validate(m.schema); err != nil {
		m.schema.Collections[name] = oldCol
		return fmt.Errorf("validating schema: %w", err)
	}

	return nil
}

// DeleteCollection removes a collection from the schema.
// Returns an error if the collection is referenced by any relation fields.
func (m *Manager) DeleteCollection(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.schema == nil {
		return fmt.Errorf("schema not loaded")
	}

	if _, exists := m.schema.Collections[name]; !exists {
		return fmt.Errorf("collection %q not found", name)
	}

	// Check for references in relation fields
	for colName, col := range m.schema.Collections {
		for fieldName, field := range col.Fields {
			if field.Type == FieldTypeRelation && field.Relation != nil && field.Relation.Collection == name {
				return fmt.Errorf("collection %q is referenced by relation in %s.%s", name, colName, fieldName)
			}
		}
	}

	delete(m.schema.Collections, name)
	return nil
}
