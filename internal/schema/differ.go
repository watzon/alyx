package schema

import (
	"fmt"
)

type ChangeType string

const (
	ChangeAddCollection  ChangeType = "add_collection"
	ChangeDropCollection ChangeType = "drop_collection"
	ChangeAddField       ChangeType = "add_field"
	ChangeDropField      ChangeType = "drop_field"
	ChangeModifyField    ChangeType = "modify_field"
	ChangeRenameField    ChangeType = "rename_field"
	ChangeAddIndex       ChangeType = "add_index"
	ChangeDropIndex      ChangeType = "drop_index"
	ChangeModifyRules    ChangeType = "modify_rules"
)

type Change struct {
	Type           ChangeType
	Collection     string
	Field          string
	OldField       *Field
	NewField       *Field
	Index          *Index
	Safe           bool
	RequiresManual bool
	Description    string
}

func (c *Change) String() string {
	switch c.Type {
	case ChangeAddCollection:
		return fmt.Sprintf("Add collection %q", c.Collection)
	case ChangeDropCollection:
		return fmt.Sprintf("Drop collection %q (DESTRUCTIVE)", c.Collection)
	case ChangeAddField:
		return fmt.Sprintf("Add field %q to collection %q", c.Field, c.Collection)
	case ChangeDropField:
		return fmt.Sprintf("Drop field %q from collection %q (DESTRUCTIVE)", c.Field, c.Collection)
	case ChangeModifyField:
		return fmt.Sprintf("Modify field %q in collection %q", c.Field, c.Collection)
	case ChangeRenameField:
		return fmt.Sprintf("Rename field in collection %q", c.Collection)
	case ChangeAddIndex:
		return fmt.Sprintf("Add index %q on collection %q", c.Index.Name, c.Collection)
	case ChangeDropIndex:
		return fmt.Sprintf("Drop index %q", c.Index.Name)
	case ChangeModifyRules:
		return fmt.Sprintf("Modify rules for collection %q", c.Collection)
	default:
		return c.Description
	}
}

type Differ struct{}

func NewDiffer() *Differ {
	return &Differ{}
}

func (d *Differ) Diff(old, new *Schema) []*Change {
	var changes []*Change

	for name := range old.Collections {
		if _, exists := new.Collections[name]; !exists {
			changes = append(changes, &Change{
				Type:           ChangeDropCollection,
				Collection:     name,
				Safe:           false,
				RequiresManual: true,
				Description:    fmt.Sprintf("Collection %q will be dropped", name),
			})
		}
	}

	for name, newCol := range new.Collections {
		oldCol, exists := old.Collections[name]
		if !exists {
			changes = append(changes, &Change{
				Type:        ChangeAddCollection,
				Collection:  name,
				Safe:        true,
				Description: fmt.Sprintf("Collection %q will be created", name),
			})
			continue
		}

		changes = append(changes, d.diffCollection(name, oldCol, newCol)...)
	}

	return changes
}

func (d *Differ) diffCollection(name string, old, new *Collection) []*Change {
	var changes []*Change

	for fieldName := range old.Fields {
		if _, exists := new.Fields[fieldName]; !exists {
			changes = append(changes, &Change{
				Type:           ChangeDropField,
				Collection:     name,
				Field:          fieldName,
				OldField:       old.Fields[fieldName],
				Safe:           false,
				RequiresManual: true,
				Description:    fmt.Sprintf("Field %q will be dropped from %q", fieldName, name),
			})
		}
	}

	for fieldName, newField := range new.Fields {
		oldField, exists := old.Fields[fieldName]
		if !exists {
			safe := newField.Nullable || newField.HasDefault()
			changes = append(changes, &Change{
				Type:           ChangeAddField,
				Collection:     name,
				Field:          fieldName,
				NewField:       newField,
				Safe:           safe,
				RequiresManual: !safe,
				Description:    fmt.Sprintf("Field %q will be added to %q", fieldName, name),
			})
			continue
		}

		if fieldChanges := d.diffField(name, fieldName, oldField, newField); len(fieldChanges) > 0 {
			changes = append(changes, fieldChanges...)
		}
	}

	changes = append(changes, d.diffIndexes(name, old, new)...)

	if d.rulesChanged(old.Rules, new.Rules) {
		changes = append(changes, &Change{
			Type:        ChangeModifyRules,
			Collection:  name,
			Safe:        true,
			Description: fmt.Sprintf("Rules for %q will be updated", name),
		})
	}

	return changes
}

func (d *Differ) diffField(collection, fieldName string, old, new *Field) []*Change {
	var changes []*Change

	if old.Type != new.Type {
		changes = append(changes, &Change{
			Type:           ChangeModifyField,
			Collection:     collection,
			Field:          fieldName,
			OldField:       old,
			NewField:       new,
			Safe:           false,
			RequiresManual: true,
			Description:    fmt.Sprintf("Field type change from %s to %s requires manual migration", old.Type, new.Type),
		})
	}

	if old.Nullable && !new.Nullable {
		changes = append(changes, &Change{
			Type:           ChangeModifyField,
			Collection:     collection,
			Field:          fieldName,
			OldField:       old,
			NewField:       new,
			Safe:           false,
			RequiresManual: true,
			Description:    "Making field non-nullable requires manual migration to handle existing NULL values",
		})
	} else if !old.Nullable && new.Nullable {
		changes = append(changes, &Change{
			Type:        ChangeModifyField,
			Collection:  collection,
			Field:       fieldName,
			OldField:    old,
			NewField:    new,
			Safe:        true,
			Description: "Making field nullable is safe",
		})
	}

	if !old.Unique && new.Unique {
		changes = append(changes, &Change{
			Type:           ChangeModifyField,
			Collection:     collection,
			Field:          fieldName,
			OldField:       old,
			NewField:       new,
			Safe:           false,
			RequiresManual: true,
			Description:    "Adding unique constraint requires manual verification of existing data",
		})
	}

	if old.References != new.References {
		changes = append(changes, &Change{
			Type:           ChangeModifyField,
			Collection:     collection,
			Field:          fieldName,
			OldField:       old,
			NewField:       new,
			Safe:           false,
			RequiresManual: true,
			Description:    "Changing foreign key reference requires manual migration",
		})
	}

	return changes
}

func (d *Differ) diffIndexes(collection string, old, new *Collection) []*Change {
	var changes []*Change

	oldIndexes := make(map[string]*Index)
	for _, idx := range old.Indexes {
		oldIndexes[idx.Name] = idx
	}

	newIndexes := make(map[string]*Index)
	for _, idx := range new.Indexes {
		newIndexes[idx.Name] = idx
	}

	for name, idx := range oldIndexes {
		if _, exists := newIndexes[name]; !exists {
			changes = append(changes, &Change{
				Type:        ChangeDropIndex,
				Collection:  collection,
				Index:       idx,
				Safe:        true,
				Description: fmt.Sprintf("Index %q will be dropped", name),
			})
		}
	}

	for name, idx := range newIndexes {
		if _, exists := oldIndexes[name]; !exists {
			changes = append(changes, &Change{
				Type:        ChangeAddIndex,
				Collection:  collection,
				Index:       idx,
				Safe:        true,
				Description: fmt.Sprintf("Index %q will be created", name),
			})
		}
	}

	return changes
}

func (d *Differ) rulesChanged(old, new *Rules) bool {
	if old == nil && new == nil {
		return false
	}
	if old == nil || new == nil {
		return true
	}
	return old.Create != new.Create ||
		old.Read != new.Read ||
		old.Update != new.Update ||
		old.Delete != new.Delete
}

func (d *Differ) SafeChanges(changes []*Change) []*Change {
	var safe []*Change
	for _, c := range changes {
		if c.Safe {
			safe = append(safe, c)
		}
	}
	return safe
}

func (d *Differ) UnsafeChanges(changes []*Change) []*Change {
	var unsafe []*Change
	for _, c := range changes {
		if !c.Safe {
			unsafe = append(unsafe, c)
		}
	}
	return unsafe
}

func (d *Differ) HasUnsafeChanges(changes []*Change) bool {
	for _, c := range changes {
		if !c.Safe {
			return true
		}
	}
	return false
}
