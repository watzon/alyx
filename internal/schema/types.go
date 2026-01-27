package schema

import (
	"fmt"
	"strings"
)

type FieldType string

const (
	FieldTypeUUID      FieldType = "uuid"
	FieldTypeString    FieldType = "string"
	FieldTypeText      FieldType = "text"
	FieldTypeRichText  FieldType = "richtext"
	FieldTypeInt       FieldType = "int"
	FieldTypeFloat     FieldType = "float"
	FieldTypeBool      FieldType = "bool"
	FieldTypeTimestamp FieldType = "timestamp"
	FieldTypeJSON      FieldType = "json"
	FieldTypeBlob      FieldType = "blob"
	FieldTypeEmail     FieldType = "email"
	FieldTypeURL       FieldType = "url"
	FieldTypeDate      FieldType = "date"
	FieldTypeSelect    FieldType = "select"
	FieldTypeRelation  FieldType = "relation"
	FieldTypeFile      FieldType = "file"
)

func (t FieldType) IsValid() bool {
	switch t {
	case FieldTypeUUID, FieldTypeString, FieldTypeText, FieldTypeRichText, FieldTypeInt,
		FieldTypeFloat, FieldTypeBool, FieldTypeTimestamp, FieldTypeJSON, FieldTypeBlob,
		FieldTypeEmail, FieldTypeURL, FieldTypeDate, FieldTypeSelect, FieldTypeRelation, FieldTypeFile:
		return true
	}
	return false
}

func (t FieldType) SQLiteType() string {
	switch t {
	case FieldTypeUUID, FieldTypeString, FieldTypeText, FieldTypeRichText, FieldTypeTimestamp,
		FieldTypeJSON, FieldTypeEmail, FieldTypeURL, FieldTypeDate, FieldTypeSelect, FieldTypeRelation, FieldTypeFile:
		return "TEXT"
	case FieldTypeInt, FieldTypeBool:
		return "INTEGER"
	case FieldTypeFloat:
		return "REAL"
	case FieldTypeBlob:
		return "BLOB"
	}
	return "TEXT"
}

func (t FieldType) GoType(nullable bool) string {
	prefix := ""
	if nullable {
		prefix = "*"
	}
	switch t {
	case FieldTypeUUID, FieldTypeString, FieldTypeText, FieldTypeRichText,
		FieldTypeEmail, FieldTypeURL, FieldTypeDate, FieldTypeRelation, FieldTypeFile:
		return prefix + "string"
	case FieldTypeInt:
		return prefix + "int64"
	case FieldTypeFloat:
		return prefix + "float64"
	case FieldTypeBool:
		return prefix + "bool"
	case FieldTypeTimestamp:
		return prefix + "time.Time"
	case FieldTypeJSON:
		return "any"
	case FieldTypeBlob:
		return "[]byte"
	case FieldTypeSelect:
		// Select can be single (string) or multi ([]string), but Go type is always string/[]string
		// Multi-select stored as JSON array in DB
		return "any" // Can be string or []string
	default:
		return prefix + "string"
	}
}

func (t FieldType) TypeScriptType(nullable bool) string {
	var base string
	switch t {
	case FieldTypeUUID, FieldTypeString, FieldTypeText, FieldTypeRichText,
		FieldTypeEmail, FieldTypeURL, FieldTypeDate, FieldTypeRelation, FieldTypeFile:
		base = "string"
	case FieldTypeInt, FieldTypeFloat:
		base = "number"
	case FieldTypeBool:
		base = "boolean"
	case FieldTypeTimestamp:
		base = "Date"
	case FieldTypeJSON:
		base = "unknown"
	case FieldTypeBlob:
		base = "Uint8Array"
	case FieldTypeSelect:
		base = "string | string[]"
	default:
		base = "string"
	}
	if nullable {
		return base + " | null"
	}
	return base
}

func (t FieldType) PythonType(nullable bool) string {
	var base string
	switch t {
	case FieldTypeUUID, FieldTypeString, FieldTypeText, FieldTypeRichText,
		FieldTypeEmail, FieldTypeURL, FieldTypeRelation, FieldTypeFile:
		base = "str"
	case FieldTypeInt:
		base = "int"
	case FieldTypeFloat:
		base = "float"
	case FieldTypeBool:
		base = "bool"
	case FieldTypeTimestamp:
		base = "datetime"
	case FieldTypeJSON:
		base = "Any"
	case FieldTypeBlob:
		base = "bytes"
	case FieldTypeDate:
		base = "date"
	case FieldTypeSelect:
		base = "Union[str, List[str]]"
	default:
		base = "str"
	}
	if nullable {
		return "Optional[" + base + "]"
	}
	return base
}

type OnDeleteAction string

const (
	OnDeleteRestrict OnDeleteAction = "restrict"
	OnDeleteCascade  OnDeleteAction = "cascade"
	OnDeleteSetNull  OnDeleteAction = "set null"
)

func (a OnDeleteAction) IsValid() bool {
	switch a {
	case OnDeleteRestrict, OnDeleteCascade, OnDeleteSetNull, "":
		return true
	}
	return false
}

func (a OnDeleteAction) SQL() string {
	switch a {
	case OnDeleteCascade:
		return "CASCADE"
	case OnDeleteSetNull:
		return "SET NULL"
	default:
		return "RESTRICT"
	}
}

type DefaultValue string

const (
	DefaultAuto DefaultValue = "auto"
	DefaultNow  DefaultValue = "now"
)

type Schema struct {
	Version     int                    `yaml:"version"`
	Collections map[string]*Collection `yaml:"collections"`
	Buckets     map[string]*Bucket     `yaml:"buckets"`
}

type Collection struct {
	Name    string            `yaml:"-"`
	Fields  map[string]*Field `yaml:"fields"`
	Indexes []*Index          `yaml:"indexes"`
	Rules   *Rules            `yaml:"rules"`

	fieldOrder []string
}

func (c *Collection) FieldOrder() []string {
	return c.fieldOrder
}

func (c *Collection) SetFieldOrder(order []string) {
	c.fieldOrder = order
}

func (c *Collection) OrderedFields() []*Field {
	fields := make([]*Field, 0, len(c.fieldOrder))
	for _, name := range c.fieldOrder {
		if f, ok := c.Fields[name]; ok {
			fields = append(fields, f)
		}
	}
	return fields
}

func (c *Collection) PrimaryKeyField() *Field {
	for _, f := range c.Fields {
		if f.Primary {
			return f
		}
	}
	return nil
}

// FileConfig defines options for file field type.
type FileConfig struct {
	Bucket       string         `yaml:"bucket"`
	MaxSize      int64          `yaml:"max_size"`
	AllowedTypes []string       `yaml:"allowed_types"`
	OnDelete     OnDeleteAction `yaml:"on_delete"`
}

type Field struct {
	Name       string           `yaml:"-"`
	Type       FieldType        `yaml:"type"`
	Primary    bool             `yaml:"primary"`
	Unique     bool             `yaml:"unique"`
	Nullable   bool             `yaml:"nullable"`
	Index      bool             `yaml:"index"`
	Default    string           `yaml:"default"`
	References string           `yaml:"references"`
	OnDelete   OnDeleteAction   `yaml:"onDelete"`
	OnUpdate   string           `yaml:"onUpdate"`
	Internal   bool             `yaml:"internal"`
	Validate   *FieldValidation `yaml:"validate"`
	RichText   *RichTextConfig  `yaml:"richtext"`
	Select     *SelectConfig    `yaml:"select"`
	Relation   *RelationConfig  `yaml:"relation"`
	File       *FileConfig      `yaml:"file"`

	MinLength *int `yaml:"minLength"`
	MaxLength *int `yaml:"maxLength"`
}

// SelectConfig defines options for select field type.
type SelectConfig struct {
	Values    []string `yaml:"values"`
	MaxSelect int      `yaml:"maxSelect"`
}

// IsMultiple returns true if field allows multiple selections.
func (c *SelectConfig) IsMultiple() bool {
	return c != nil && c.MaxSelect != 1
}

// RelationConfig defines options for relation field type.
type RelationConfig struct {
	Collection  string         `yaml:"collection"`
	Field       string         `yaml:"field"`
	OnDelete    OnDeleteAction `yaml:"onDelete"`
	DisplayName string         `yaml:"displayName"`
}

func (f *Field) HasDefault() bool {
	return f.Default != ""
}

func (f *Field) IsAutoGenerated() bool {
	return f.Default == string(DefaultAuto)
}

func (f *Field) IsTimestampNow() bool {
	return f.Default == string(DefaultNow)
}

func (f *Field) IsAutoUpdateTimestamp() bool {
	return f.OnUpdate == string(DefaultNow)
}

func (f *Field) ParseReference() (table, field string, ok bool) {
	if f.References == "" {
		return "", "", false
	}
	parts := strings.SplitN(f.References, ".", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (f *Field) SQLDefault() string {
	if f.Default == "" {
		return ""
	}
	switch f.Default {
	case string(DefaultAuto):
		if f.Type == FieldTypeUUID {
			return ""
		}
		return ""
	case string(DefaultNow):
		return "(datetime('now'))"
	default:
		switch f.Type {
		case FieldTypeString, FieldTypeText, FieldTypeRichText, FieldTypeUUID:
			return fmt.Sprintf("'%s'", strings.ReplaceAll(f.Default, "'", "''"))
		case FieldTypeBool:
			if f.Default == "true" {
				return "1"
			}
			return "0"
		default:
			return f.Default
		}
	}
}

type FieldValidation struct {
	MinLength *int     `yaml:"minLength"`
	MaxLength *int     `yaml:"maxLength"`
	Min       *float64 `yaml:"min"`
	Max       *float64 `yaml:"max"`
	Format    string   `yaml:"format"`
	Pattern   string   `yaml:"pattern"`
	Enum      []string `yaml:"enum"`
}

type RichTextPreset string

const (
	RichTextPresetBasic    RichTextPreset = "basic"
	RichTextPresetStandard RichTextPreset = "standard"
	RichTextPresetFull     RichTextPreset = "full"
	RichTextPresetMinimal  RichTextPreset = "minimal"
)

type RichTextFormat string

const (
	RichTextFormatBold           RichTextFormat = "bold"
	RichTextFormatItalic         RichTextFormat = "italic"
	RichTextFormatUnderline      RichTextFormat = "underline"
	RichTextFormatStrike         RichTextFormat = "strike"
	RichTextFormatCode           RichTextFormat = "code"
	RichTextFormatLink           RichTextFormat = "link"
	RichTextFormatHeading        RichTextFormat = "heading"
	RichTextFormatBlockquote     RichTextFormat = "blockquote"
	RichTextFormatCodeBlock      RichTextFormat = "codeblock"
	RichTextFormatBulletList     RichTextFormat = "bulletlist"
	RichTextFormatOrderedList    RichTextFormat = "orderedlist"
	RichTextFormatHorizontalRule RichTextFormat = "horizontalrule"
)

var richTextPresetFormats = map[RichTextPreset][]RichTextFormat{
	RichTextPresetMinimal: {
		RichTextFormatBold,
		RichTextFormatItalic,
	},
	RichTextPresetBasic: {
		RichTextFormatBold,
		RichTextFormatItalic,
		RichTextFormatLink,
		RichTextFormatBulletList,
	},
	RichTextPresetStandard: {
		RichTextFormatBold,
		RichTextFormatItalic,
		RichTextFormatUnderline,
		RichTextFormatStrike,
		RichTextFormatLink,
		RichTextFormatHeading,
		RichTextFormatBulletList,
		RichTextFormatOrderedList,
		RichTextFormatBlockquote,
	},
	RichTextPresetFull: {
		RichTextFormatBold,
		RichTextFormatItalic,
		RichTextFormatUnderline,
		RichTextFormatStrike,
		RichTextFormatCode,
		RichTextFormatLink,
		RichTextFormatHeading,
		RichTextFormatBlockquote,
		RichTextFormatCodeBlock,
		RichTextFormatBulletList,
		RichTextFormatOrderedList,
		RichTextFormatHorizontalRule,
	},
}

var validRichTextFormats = map[RichTextFormat]bool{
	RichTextFormatBold:           true,
	RichTextFormatItalic:         true,
	RichTextFormatUnderline:      true,
	RichTextFormatStrike:         true,
	RichTextFormatCode:           true,
	RichTextFormatLink:           true,
	RichTextFormatHeading:        true,
	RichTextFormatBlockquote:     true,
	RichTextFormatCodeBlock:      true,
	RichTextFormatBulletList:     true,
	RichTextFormatOrderedList:    true,
	RichTextFormatHorizontalRule: true,
}

type RichTextConfig struct {
	Preset RichTextPreset   `yaml:"preset"`
	Allow  []RichTextFormat `yaml:"allow"`
	Deny   []RichTextFormat `yaml:"deny"`
}

func (c *RichTextConfig) GetAllowedFormats() []RichTextFormat {
	if c == nil {
		return richTextPresetFormats[RichTextPresetBasic]
	}

	var baseFormats []RichTextFormat
	switch {
	case c.Preset != "":
		baseFormats = richTextPresetFormats[c.Preset]
	default:
		baseFormats = richTextPresetFormats[RichTextPresetBasic]
	}

	formatSet := make(map[RichTextFormat]bool)
	for _, f := range baseFormats {
		formatSet[f] = true
	}

	for _, f := range c.Allow {
		formatSet[f] = true
	}

	for _, f := range c.Deny {
		delete(formatSet, f)
	}

	result := make([]RichTextFormat, 0, len(formatSet))
	for f := range formatSet {
		result = append(result, f)
	}
	return result
}

func (c *RichTextConfig) IsFormatAllowed(format RichTextFormat) bool {
	for _, f := range c.GetAllowedFormats() {
		if f == format {
			return true
		}
	}
	return false
}

func IsValidRichTextFormat(format RichTextFormat) bool {
	return validRichTextFormats[format]
}

func IsValidRichTextPreset(preset RichTextPreset) bool {
	_, ok := richTextPresetFormats[preset]
	return ok
}

type Index struct {
	Name   string   `yaml:"name"`
	Fields []string `yaml:"fields"`
	Unique bool     `yaml:"unique"`
	Order  string   `yaml:"order"`
}

func (i *Index) SQL(tableName string) string {
	uniqueStr := ""
	if i.Unique {
		uniqueStr = "UNIQUE "
	}

	orderStr := ""
	if i.Order != "" {
		orderStr = " " + strings.ToUpper(i.Order)
	}

	fieldList := make([]string, len(i.Fields))
	for idx, f := range i.Fields {
		fieldList[idx] = f + orderStr
	}

	return fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s (%s)",
		uniqueStr, i.Name, tableName, strings.Join(fieldList, ", "))
}

type Rules struct {
	Create string `yaml:"create"`
	Read   string `yaml:"read"`
	Update string `yaml:"update"`
	Delete string `yaml:"delete"`
}

func (r *Rules) HasRules() bool {
	return r != nil && (r.Create != "" || r.Read != "" || r.Update != "" || r.Delete != "")
}

type Bucket struct {
	Name         string   `yaml:"-"`
	Backend      string   `yaml:"backend"`
	MaxFileSize  int64    `yaml:"max_file_size"`
	MaxTotalSize int64    `yaml:"max_total_size"`
	AllowedTypes []string `yaml:"allowed_types"`
	Compression  bool     `yaml:"compression"`
	Rules        *Rules   `yaml:"rules"`
}

type BucketRules struct {
	Create string `yaml:"create"`
	Read   string `yaml:"read"`
	Update string `yaml:"update"`
	Delete string `yaml:"delete"`
}
