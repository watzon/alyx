package schema

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// Marshal serializes a Schema to YAML bytes.
// Collections, Buckets, and Functions are sorted alphabetically by name.
// Field order within collections is preserved using Collection.FieldOrder().
func Marshal(s *Schema) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("schema is nil")
	}

	// Build the raw schema structure for serialization
	raw := &rawSchemaWriter{
		Version:     s.Version,
		Buckets:     make(map[string]*rawBucketWriter),
		Collections: make(map[string]*rawCollectionWriter),
		Functions:   make(map[string]*rawFunctionWriter),
	}

	// Convert buckets (sorted alphabetically)
	bucketNames := make([]string, 0, len(s.Buckets))
	for name := range s.Buckets {
		bucketNames = append(bucketNames, name)
	}
	sort.Strings(bucketNames)

	for _, name := range bucketNames {
		bucket := s.Buckets[name]
		raw.Buckets[name] = &rawBucketWriter{
			Backend:      bucket.Backend,
			MaxFileSize:  bucket.MaxFileSize,
			MaxTotalSize: bucket.MaxTotalSize,
			AllowedTypes: bucket.AllowedTypes,
			Compression:  bucket.Compression,
			Rules:        bucket.Rules,
		}
	}

	// Convert collections (sorted alphabetically)
	collectionNames := make([]string, 0, len(s.Collections))
	for name := range s.Collections {
		collectionNames = append(collectionNames, name)
	}
	sort.Strings(collectionNames)

	for _, name := range collectionNames {
		col := s.Collections[name]
		rawCol := &rawCollectionWriter{
			Indexes: col.Indexes,
			Rules:   col.Rules,
		}

		// Use yaml.Node to preserve field order
		fieldsNode := &yaml.Node{
			Kind: yaml.MappingNode,
		}

		// Add fields in the order specified by FieldOrder()
		for _, fieldName := range col.FieldOrder() {
			if field, ok := col.Fields[fieldName]; ok {
				// Create key node
				keyNode := &yaml.Node{
					Kind:  yaml.ScalarNode,
					Value: fieldName,
				}

				// Create value node by encoding the field
				valueNode := &yaml.Node{}
				if err := valueNode.Encode(marshalField(field)); err != nil {
					return nil, fmt.Errorf("encoding field %s.%s: %w", name, fieldName, err)
				}

				fieldsNode.Content = append(fieldsNode.Content, keyNode, valueNode)
			}
		}

		rawCol.Fields = fieldsNode
		raw.Collections[name] = rawCol
	}

	// Convert functions (sorted alphabetically)
	if len(s.Functions) > 0 {
		functionNames := make([]string, 0, len(s.Functions))
		for name := range s.Functions {
			functionNames = append(functionNames, name)
		}
		sort.Strings(functionNames)

		for _, name := range functionNames {
			fn := s.Functions[name]
			raw.Functions[name] = &rawFunctionWriter{
				Runtime:      fn.Runtime,
				Entrypoint:   fn.Entrypoint,
				Path:         fn.Path,
				Description:  fn.Description,
				SampleInput:  fn.SampleInput,
				Timeout:      fn.Timeout,
				Memory:       fn.Memory,
				Env:          fn.Env,
				Dependencies: fn.Dependencies,
				Hooks:        fn.Hooks,
				Schedules:    fn.Schedules,
				Routes:       fn.Routes,
				Build:        fn.Build,
				Rules:        fn.Rules,
			}
		}
	}

	// Use yaml.v3 Node API to control field ordering
	node := &yaml.Node{}
	if err := node.Encode(raw); err != nil {
		return nil, fmt.Errorf("encoding schema: %w", err)
	}

	// Marshal with proper indentation
	data, err := yaml.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("marshaling YAML: %w", err)
	}

	return data, nil
}

// WriteFile writes a Schema to a file using atomic write pattern.
// It writes to a temporary file first, then renames it to the target path.
func WriteFile(path string, s *Schema) error {
	data, err := Marshal(s)
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // cleanup on failure
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// marshalField converts a Field to a fieldWriter for serialization.
func marshalField(f *Field) *fieldWriter {
	fw := &fieldWriter{
		Type:       f.Type,
		Primary:    f.Primary,
		Unique:     f.Unique,
		Nullable:   f.Nullable,
		Index:      f.Index,
		Default:    f.Default,
		References: f.References,
		OnDelete:   f.OnDelete,
		OnUpdate:   f.OnUpdate,
		Internal:   f.Internal,
		Validate:   f.Validate,
		RichText:   f.RichText,
		Select:     f.Select,
		Relation:   f.Relation,
		File:       f.File,
		MinLength:  f.MinLength,
		MaxLength:  f.MaxLength,
	}
	return fw
}

// rawSchemaWriter is the intermediate structure for YAML serialization.
type rawSchemaWriter struct {
	Version     int                             `yaml:"version"`
	Buckets     map[string]*rawBucketWriter     `yaml:"buckets,omitempty"`
	Collections map[string]*rawCollectionWriter `yaml:"collections"`
	Functions   map[string]*rawFunctionWriter   `yaml:"functions,omitempty"`
}

// rawCollectionWriter represents a collection for serialization.
type rawCollectionWriter struct {
	Fields  *yaml.Node `yaml:"fields"`
	Indexes []*Index   `yaml:"indexes,omitempty"`
	Rules   *Rules     `yaml:"rules,omitempty"`
}

// fieldWriter represents a field for serialization.
type fieldWriter struct {
	Type       FieldType        `yaml:"type"`
	Primary    bool             `yaml:"primary,omitempty"`
	Unique     bool             `yaml:"unique,omitempty"`
	Nullable   bool             `yaml:"nullable,omitempty"`
	Index      bool             `yaml:"index,omitempty"`
	Default    string           `yaml:"default,omitempty"`
	References string           `yaml:"references,omitempty"`
	OnDelete   OnDeleteAction   `yaml:"onDelete,omitempty"`
	OnUpdate   string           `yaml:"onUpdate,omitempty"`
	Internal   bool             `yaml:"internal,omitempty"`
	Validate   *FieldValidation `yaml:"validate,omitempty"`
	RichText   *RichTextConfig  `yaml:"richtext,omitempty"`
	Select     *SelectConfig    `yaml:"select,omitempty"`
	Relation   *RelationConfig  `yaml:"relation,omitempty"`
	File       *FileConfig      `yaml:"file,omitempty"`
	MinLength  *int             `yaml:"minLength,omitempty"`
	MaxLength  *int             `yaml:"maxLength,omitempty"`
}

// rawBucketWriter represents a bucket for serialization.
type rawBucketWriter struct {
	Backend      string   `yaml:"backend"`
	MaxFileSize  int64    `yaml:"max_file_size,omitempty"`
	MaxTotalSize int64    `yaml:"max_total_size,omitempty"`
	AllowedTypes []string `yaml:"allowed_types,omitempty"`
	Compression  bool     `yaml:"compression,omitempty"`
	Rules        *Rules   `yaml:"rules,omitempty"`
}

// rawFunctionWriter represents a function for serialization.
type rawFunctionWriter struct {
	Runtime      string             `yaml:"runtime"`
	Entrypoint   string             `yaml:"entrypoint"`
	Path         string             `yaml:"path,omitempty"`
	Description  string             `yaml:"description,omitempty"`
	SampleInput  any                `yaml:"sample_input,omitempty"`
	Timeout      string             `yaml:"timeout,omitempty"`
	Memory       string             `yaml:"memory,omitempty"`
	Env          map[string]string  `yaml:"env,omitempty"`
	Dependencies []string           `yaml:"dependencies,omitempty"`
	Hooks        []FunctionHook     `yaml:"hooks,omitempty"`
	Schedules    []FunctionSchedule `yaml:"schedules,omitempty"`
	Routes       []FunctionRoute    `yaml:"routes,omitempty"`
	Build        *FunctionBuild     `yaml:"build,omitempty"`
	Rules        *FunctionRules     `yaml:"rules,omitempty"`
}
