package schema

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	identifierRegex = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

func ParseFile(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading schema file: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (*Schema, error) {
	var raw rawSchema
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing schema YAML: %w", err)
	}

	schema := &Schema{
		Version:     raw.Version,
		Collections: make(map[string]*Collection),
	}

	for name, rawCol := range raw.Collections {
		col, err := parseCollection(name, rawCol)
		if err != nil {
			return nil, fmt.Errorf("collection %q: %w", name, err)
		}
		schema.Collections[name] = col
	}

	if err := Validate(schema); err != nil {
		return nil, err
	}

	return schema, nil
}

type rawSchema struct {
	Version     int                       `yaml:"version"`
	Collections map[string]*rawCollection `yaml:"collections"`
}

type rawCollection struct {
	Fields  yaml.Node `yaml:"fields"`
	Indexes []*Index  `yaml:"indexes"`
	Rules   *Rules    `yaml:"rules"`
}

func parseCollection(name string, raw *rawCollection) (*Collection, error) {
	col := &Collection{
		Name:    name,
		Fields:  make(map[string]*Field),
		Indexes: raw.Indexes,
		Rules:   raw.Rules,
	}

	if raw.Fields.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("fields must be a mapping")
	}

	fieldOrder := make([]string, 0, len(raw.Fields.Content)/2)
	for i := 0; i < len(raw.Fields.Content); i += 2 {
		keyNode := raw.Fields.Content[i]
		valueNode := raw.Fields.Content[i+1]

		fieldName := keyNode.Value
		fieldOrder = append(fieldOrder, fieldName)

		var field Field
		if err := valueNode.Decode(&field); err != nil {
			return nil, fmt.Errorf("field %q: %w", fieldName, err)
		}
		field.Name = fieldName

		if field.Validate != nil {
			if field.MinLength == nil && field.Validate.MinLength != nil {
				field.MinLength = field.Validate.MinLength
			}
			if field.MaxLength == nil && field.Validate.MaxLength != nil {
				field.MaxLength = field.Validate.MaxLength
			}
		}

		col.Fields[fieldName] = &field
	}

	col.SetFieldOrder(fieldOrder)
	return col, nil
}

type ValidationError struct {
	Path    string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

type ValidationErrors []*ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("schema validation failed:\n")
	for _, err := range e {
		sb.WriteString("  - ")
		sb.WriteString(err.Error())
		sb.WriteString("\n")
	}
	return sb.String()
}

func Validate(s *Schema) error {
	var errs ValidationErrors

	if s.Version < 1 {
		errs = append(errs, &ValidationError{
			Path:    "version",
			Message: "must be at least 1",
		})
	}

	if len(s.Collections) == 0 {
		errs = append(errs, &ValidationError{
			Path:    "collections",
			Message: "at least one collection is required",
		})
	}

	for name, col := range s.Collections {
		colErrs := validateCollection(name, col, s)
		errs = append(errs, colErrs...)
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func validateCollection(name string, col *Collection, s *Schema) ValidationErrors {
	var errs ValidationErrors
	path := fmt.Sprintf("collections.%s", name)

	if !identifierRegex.MatchString(name) {
		errs = append(errs, &ValidationError{
			Path:    path,
			Message: "name must start with lowercase letter and contain only lowercase letters, numbers, and underscores",
		})
	}

	if strings.HasPrefix(name, "_alyx") {
		errs = append(errs, &ValidationError{
			Path:    path,
			Message: "collection names starting with '_alyx' are reserved",
		})
	}

	if len(col.Fields) == 0 {
		errs = append(errs, &ValidationError{
			Path:    path + ".fields",
			Message: "at least one field is required",
		})
	}

	hasPrimary := false
	for fieldName, field := range col.Fields {
		fieldErrs := validateField(path+".fields."+fieldName, fieldName, field, s)
		errs = append(errs, fieldErrs...)

		if field.Primary {
			if hasPrimary {
				errs = append(errs, &ValidationError{
					Path:    path + ".fields." + fieldName,
					Message: "only one primary key field is allowed",
				})
			}
			hasPrimary = true
		}
	}

	if !hasPrimary {
		errs = append(errs, &ValidationError{
			Path:    path,
			Message: "collection must have a primary key field",
		})
	}

	for i, idx := range col.Indexes {
		idxPath := fmt.Sprintf("%s.indexes[%d]", path, i)
		if idx.Name == "" {
			errs = append(errs, &ValidationError{
				Path:    idxPath + ".name",
				Message: "index name is required",
			})
		}
		if len(idx.Fields) == 0 {
			errs = append(errs, &ValidationError{
				Path:    idxPath + ".fields",
				Message: "index must have at least one field",
			})
		}
		for _, f := range idx.Fields {
			if _, ok := col.Fields[f]; !ok {
				errs = append(errs, &ValidationError{
					Path:    idxPath + ".fields",
					Message: fmt.Sprintf("field %q does not exist in collection", f),
				})
			}
		}
	}

	return errs
}

func validateField(path, name string, f *Field, s *Schema) ValidationErrors {
	var errs ValidationErrors

	errs = append(errs, validateFieldBasics(path, name, f)...)
	errs = append(errs, validateFieldReferences(path, f, s)...)
	errs = append(errs, validateFieldTimestamps(path, f)...)
	errs = append(errs, validateFieldLength(path, f)...)
	errs = append(errs, validateFieldRichText(path, f)...)

	if f.Validate != nil {
		errs = append(errs, validateFieldValidation(path+".validate", f)...)
	}

	return errs
}

func validateFieldBasics(path, name string, f *Field) ValidationErrors {
	var errs ValidationErrors

	if !identifierRegex.MatchString(name) {
		errs = append(errs, &ValidationError{
			Path:    path,
			Message: "name must start with lowercase letter and contain only lowercase letters, numbers, and underscores",
		})
	}

	if !f.Type.IsValid() {
		errs = append(errs, &ValidationError{
			Path:    path + ".type",
			Message: fmt.Sprintf("invalid type %q; must be one of: uuid, string, text, int, float, bool, timestamp, json, blob", f.Type),
		})
	}

	if f.Primary && f.Nullable {
		errs = append(errs, &ValidationError{
			Path:    path,
			Message: "primary key cannot be nullable",
		})
	}

	return errs
}

func validateFieldReferences(path string, f *Field, s *Schema) ValidationErrors {
	var errs ValidationErrors

	if f.References == "" {
		return errs
	}

	table, field, ok := f.ParseReference()
	if !ok {
		errs = append(errs, &ValidationError{
			Path:    path + ".references",
			Message: "must be in format 'table.field'",
		})
	} else if refCol, ok := s.Collections[table]; !ok {
		errs = append(errs, &ValidationError{
			Path:    path + ".references",
			Message: fmt.Sprintf("referenced collection %q does not exist", table),
		})
	} else if _, ok := refCol.Fields[field]; !ok {
		errs = append(errs, &ValidationError{
			Path:    path + ".references",
			Message: fmt.Sprintf("referenced field %q does not exist in collection %q", field, table),
		})
	}

	if !f.OnDelete.IsValid() {
		errs = append(errs, &ValidationError{
			Path:    path + ".onDelete",
			Message: "must be one of: restrict, cascade, set null",
		})
	}

	if f.OnDelete == OnDeleteSetNull && !f.Nullable {
		errs = append(errs, &ValidationError{
			Path:    path + ".onDelete",
			Message: "cannot use 'set null' on non-nullable field",
		})
	}

	return errs
}

func validateFieldTimestamps(path string, f *Field) ValidationErrors {
	var errs ValidationErrors

	if f.OnUpdate != "" && f.OnUpdate != string(DefaultNow) {
		errs = append(errs, &ValidationError{
			Path:    path + ".onUpdate",
			Message: "only 'now' is supported for onUpdate",
		})
	}

	if f.OnUpdate == string(DefaultNow) && f.Type != FieldTypeTimestamp {
		errs = append(errs, &ValidationError{
			Path:    path + ".onUpdate",
			Message: "onUpdate: now can only be used with timestamp type",
		})
	}

	return errs
}

func validateFieldLength(path string, f *Field) ValidationErrors {
	var errs ValidationErrors

	if f.MinLength == nil && f.MaxLength == nil {
		return errs
	}

	if f.Type != FieldTypeString && f.Type != FieldTypeText && f.Type != FieldTypeRichText {
		errs = append(errs, &ValidationError{
			Path:    path,
			Message: "minLength/maxLength can only be used with string, text, or richtext types",
		})
	}

	if f.MinLength != nil && *f.MinLength < 0 {
		errs = append(errs, &ValidationError{
			Path:    path + ".minLength",
			Message: "must be non-negative",
		})
	}

	if f.MaxLength != nil && *f.MaxLength < 1 {
		errs = append(errs, &ValidationError{
			Path:    path + ".maxLength",
			Message: "must be at least 1",
		})
	}

	if f.MinLength != nil && f.MaxLength != nil && *f.MinLength > *f.MaxLength {
		errs = append(errs, &ValidationError{
			Path:    path,
			Message: "minLength cannot be greater than maxLength",
		})
	}

	return errs
}

func validateFieldRichText(path string, f *Field) ValidationErrors {
	var errs ValidationErrors

	if f.RichText != nil && f.Type != FieldTypeRichText {
		errs = append(errs, &ValidationError{
			Path:    path + ".richtext",
			Message: "richtext config can only be used with richtext field type",
		})
		return errs
	}

	if f.RichText == nil {
		return errs
	}

	if f.RichText.Preset != "" && !IsValidRichTextPreset(f.RichText.Preset) {
		errs = append(errs, &ValidationError{
			Path:    path + ".richtext.preset",
			Message: "must be one of: minimal, basic, standard, full",
		})
	}

	for i, format := range f.RichText.Allow {
		if !IsValidRichTextFormat(format) {
			errs = append(errs, &ValidationError{
				Path:    fmt.Sprintf("%s.richtext.allow[%d]", path, i),
				Message: fmt.Sprintf("invalid format: %s", format),
			})
		}
	}

	for i, format := range f.RichText.Deny {
		if !IsValidRichTextFormat(format) {
			errs = append(errs, &ValidationError{
				Path:    fmt.Sprintf("%s.richtext.deny[%d]", path, i),
				Message: fmt.Sprintf("invalid format: %s", format),
			})
		}
	}

	return errs
}

func validateFieldValidation(path string, f *Field) ValidationErrors {
	var errs ValidationErrors
	v := f.Validate

	if v.Format != "" {
		validFormats := map[string]bool{"email": true, "url": true, "uuid": true}
		if !validFormats[v.Format] {
			errs = append(errs, &ValidationError{
				Path:    path + ".format",
				Message: "must be one of: email, url, uuid",
			})
		}
	}

	if v.Pattern != "" {
		if _, err := regexp.Compile(v.Pattern); err != nil {
			errs = append(errs, &ValidationError{
				Path:    path + ".pattern",
				Message: fmt.Sprintf("invalid regex pattern: %v", err),
			})
		}
	}

	if v.Min != nil && v.Max != nil && *v.Min > *v.Max {
		errs = append(errs, &ValidationError{
			Path:    path,
			Message: "min cannot be greater than max",
		})
	}

	if (v.Min != nil || v.Max != nil) && f.Type != FieldTypeInt && f.Type != FieldTypeFloat {
		errs = append(errs, &ValidationError{
			Path:    path,
			Message: "min/max can only be used with int or float types",
		})
	}

	return errs
}
