package schema

import (
	"database/sql"
	"fmt"
	"strings"
)

type columnInfo struct {
	Name       string
	Type       string
	NotNull    bool
	PK         bool
	HasDefault bool
}

func InferFromDB(db *sql.DB) (*Schema, error) {
	tables, err := getUserTables(db)
	if err != nil {
		return nil, fmt.Errorf("getting tables: %w", err)
	}

	if len(tables) == 0 {
		return &Schema{
			Version:     1,
			Collections: make(map[string]*Collection),
		}, nil
	}

	schema := &Schema{
		Version:     1,
		Collections: make(map[string]*Collection),
	}

	for _, table := range tables {
		cols, err := getTableColumns(db, table)
		if err != nil {
			return nil, fmt.Errorf("getting columns for %s: %w", table, err)
		}

		collection := &Collection{
			Name:   table,
			Fields: make(map[string]*Field),
		}

		var fieldOrder []string
		for _, col := range cols {
			field := columnToField(col)
			collection.Fields[col.Name] = field
			fieldOrder = append(fieldOrder, col.Name)
		}
		collection.SetFieldOrder(fieldOrder)

		if err := enrichFieldMetadata(db, table, collection); err != nil {
			return nil, fmt.Errorf("enriching metadata for %s: %w", table, err)
		}

		schema.Collections[table] = collection
	}

	return schema, nil
}

func getUserTables(db *sql.DB) ([]string, error) {
	systemTables := map[string]bool{
		"events":            true,
		"hooks":             true,
		"webhook_endpoints": true,
		"schedules":         true,
		"executions":        true,
	}

	rows, err := db.Query(`
		SELECT name FROM sqlite_master 
		WHERE type='table' 
		AND name NOT LIKE 'sqlite_%'
		AND name NOT LIKE '_alyx_%'
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	var allTables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		allTables = append(allTables, name)
		if !systemTables[name] {
			tables = append(tables, name)
		}
	}

	fmt.Printf("DEBUG: All tables from DB: %v\n", allTables)
	fmt.Printf("DEBUG: User tables after filtering: %v\n", tables)

	return tables, rows.Err()
}

func getTableColumns(db *sql.DB, table string) ([]columnInfo, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []columnInfo
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString

		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}

		cols = append(cols, columnInfo{
			Name:       name,
			Type:       colType,
			NotNull:    notNull == 1,
			PK:         pk == 1,
			HasDefault: dfltValue.Valid,
		})
	}
	return cols, rows.Err()
}

func columnToField(col columnInfo) *Field {
	field := &Field{
		Name:     col.Name,
		Type:     sqliteTypeToFieldType(col.Type),
		Primary:  col.PK,
		Nullable: !col.NotNull && !col.PK,
	}
	return field
}

func sqliteTypeToFieldType(sqlType string) FieldType {
	sqlType = strings.ToUpper(sqlType)
	switch {
	case strings.Contains(sqlType, "INT"):
		return FieldTypeInt
	case strings.Contains(sqlType, "REAL") || strings.Contains(sqlType, "FLOAT") || strings.Contains(sqlType, "DOUBLE"):
		return FieldTypeFloat
	case strings.Contains(sqlType, "BLOB"):
		return FieldTypeBlob
	case strings.Contains(sqlType, "BOOL"):
		return FieldTypeBool
	default:
		return FieldTypeString
	}
}

func enrichFieldMetadata(db *sql.DB, table string, collection *Collection) error {
	fkRows, err := db.Query(fmt.Sprintf("PRAGMA foreign_key_list(%s)", table))
	if err != nil {
		return err
	}
	defer fkRows.Close()

	for fkRows.Next() {
		var id, seq int
		var refTable, from, to string
		var onUpdate, onDelete, match string
		if err := fkRows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return err
		}
		if field, exists := collection.Fields[from]; exists {
			field.References = refTable + "." + to
		}
	}

	indexRows, err := db.Query(fmt.Sprintf("PRAGMA index_list(%s)", table))
	if err != nil {
		return err
	}

	type indexInfo struct {
		name   string
		unique bool
	}
	var indexes []indexInfo

	for indexRows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := indexRows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			indexRows.Close()
			return err
		}
		indexes = append(indexes, indexInfo{name: name, unique: unique == 1})
	}
	indexRows.Close()

	for _, idx := range indexes {
		if !idx.unique {
			continue
		}

		infoRows, err := db.Query(fmt.Sprintf("PRAGMA index_info(%s)", idx.name))
		if err != nil {
			continue
		}

		var fieldNames []string
		for infoRows.Next() {
			var seqno, cid int
			var fieldName sql.NullString
			if err := infoRows.Scan(&seqno, &cid, &fieldName); err != nil {
				infoRows.Close()
				continue
			}
			if fieldName.Valid {
				fieldNames = append(fieldNames, fieldName.String)
			}
		}
		infoRows.Close()

		if len(fieldNames) == 1 {
			if field, exists := collection.Fields[fieldNames[0]]; exists {
				field.Unique = true
			}
		}
	}

	return nil
}
