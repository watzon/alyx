package database

import (
	"fmt"
	"strings"
)

type FilterOp string

const (
	OpEq       FilterOp = "eq"
	OpNe       FilterOp = "ne"
	OpGt       FilterOp = "gt"
	OpGte      FilterOp = "gte"
	OpLt       FilterOp = "lt"
	OpLte      FilterOp = "lte"
	OpLike     FilterOp = "like"
	OpIn       FilterOp = "in"
	OpContains FilterOp = "contains"
	OpIsNull   FilterOp = "is_null"
	OpNotNull  FilterOp = "not_null"
)

type Filter struct {
	Field string
	Op    FilterOp
	Value any
}

type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

type Sort struct {
	Field string
	Order SortOrder
}

type QueryBuilder struct {
	table   string
	selects []string
	filters []*Filter
	sorts   []*Sort
	limit   int
	offset  int
	args    []any
}

func NewQuery(table string) *QueryBuilder {
	return &QueryBuilder{
		table:   table,
		selects: []string{"*"},
	}
}

func (q *QueryBuilder) Select(fields ...string) *QueryBuilder {
	q.selects = fields
	return q
}

func (q *QueryBuilder) Filter(field string, op FilterOp, value any) *QueryBuilder {
	q.filters = append(q.filters, &Filter{Field: field, Op: op, Value: value})
	return q
}

func (q *QueryBuilder) Where(field string, value any) *QueryBuilder {
	return q.Filter(field, OpEq, value)
}

func (q *QueryBuilder) Sort(field string, order SortOrder) *QueryBuilder {
	q.sorts = append(q.sorts, &Sort{Field: field, Order: order})
	return q
}

func (q *QueryBuilder) OrderBy(field string) *QueryBuilder {
	return q.Sort(field, SortAsc)
}

func (q *QueryBuilder) OrderByDesc(field string) *QueryBuilder {
	return q.Sort(field, SortDesc)
}

func (q *QueryBuilder) Limit(n int) *QueryBuilder {
	q.limit = n
	return q
}

func (q *QueryBuilder) Offset(n int) *QueryBuilder {
	q.offset = n
	return q
}

func (q *QueryBuilder) Build() (string, []any) {
	var sb strings.Builder
	q.args = nil

	sb.WriteString("SELECT ")
	sb.WriteString(strings.Join(q.selects, ", "))
	sb.WriteString(" FROM ")
	sb.WriteString(q.table)

	if len(q.filters) > 0 {
		sb.WriteString(" WHERE ")
		var conditions []string
		for _, f := range q.filters {
			cond, args := q.buildFilter(f)
			conditions = append(conditions, cond)
			q.args = append(q.args, args...)
		}
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	if len(q.sorts) > 0 {
		sb.WriteString(" ORDER BY ")
		var sortClauses []string
		for _, s := range q.sorts {
			sortClauses = append(sortClauses, fmt.Sprintf("%s %s", s.Field, s.Order))
		}
		sb.WriteString(strings.Join(sortClauses, ", "))
	}

	if q.limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", q.limit))
	}

	if q.offset > 0 {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", q.offset))
	}

	return sb.String(), q.args
}

func (q *QueryBuilder) buildFilter(f *Filter) (string, []any) {
	switch f.Op {
	case OpEq:
		return fmt.Sprintf("%s = ?", f.Field), []any{f.Value}
	case OpNe:
		return fmt.Sprintf("%s != ?", f.Field), []any{f.Value}
	case OpGt:
		return fmt.Sprintf("%s > ?", f.Field), []any{f.Value}
	case OpGte:
		return fmt.Sprintf("%s >= ?", f.Field), []any{f.Value}
	case OpLt:
		return fmt.Sprintf("%s < ?", f.Field), []any{f.Value}
	case OpLte:
		return fmt.Sprintf("%s <= ?", f.Field), []any{f.Value}
	case OpLike:
		return fmt.Sprintf("%s LIKE ?", f.Field), []any{f.Value}
	case OpContains:
		return fmt.Sprintf("%s LIKE ?", f.Field), []any{"%" + fmt.Sprint(f.Value) + "%"}
	case OpIn:
		values, ok := f.Value.([]any)
		if !ok {
			return fmt.Sprintf("%s = ?", f.Field), []any{f.Value}
		}
		placeholders := make([]string, len(values))
		for i := range values {
			placeholders[i] = "?"
		}
		return fmt.Sprintf("%s IN (%s)", f.Field, strings.Join(placeholders, ", ")), values
	case OpIsNull:
		return fmt.Sprintf("%s IS NULL", f.Field), nil
	case OpNotNull:
		return fmt.Sprintf("%s IS NOT NULL", f.Field), nil
	default:
		return fmt.Sprintf("%s = ?", f.Field), []any{f.Value}
	}
}

func (q *QueryBuilder) BuildCount() (string, []any) {
	var sb strings.Builder
	q.args = nil

	sb.WriteString("SELECT COUNT(*) FROM ")
	sb.WriteString(q.table)

	if len(q.filters) > 0 {
		sb.WriteString(" WHERE ")
		var conditions []string
		for _, f := range q.filters {
			cond, args := q.buildFilter(f)
			conditions = append(conditions, cond)
			q.args = append(q.args, args...)
		}
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	return sb.String(), q.args
}

type InsertBuilder struct {
	table  string
	fields []string
	values []any
}

func NewInsert(table string) *InsertBuilder {
	return &InsertBuilder{table: table}
}

func (b *InsertBuilder) Set(field string, value any) *InsertBuilder {
	b.fields = append(b.fields, field)
	b.values = append(b.values, value)
	return b
}

func (b *InsertBuilder) SetMap(data map[string]any) *InsertBuilder {
	for k, v := range data {
		b.Set(k, v)
	}
	return b
}

func (b *InsertBuilder) Build() (string, []any) {
	placeholders := make([]string, len(b.fields))
	for i := range b.fields {
		placeholders[i] = "?"
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		b.table,
		strings.Join(b.fields, ", "),
		strings.Join(placeholders, ", "))

	return sql, b.values
}

type UpdateBuilder struct {
	table   string
	sets    []string
	values  []any
	filters []*Filter
}

func NewUpdate(table string) *UpdateBuilder {
	return &UpdateBuilder{table: table}
}

func (b *UpdateBuilder) Set(field string, value any) *UpdateBuilder {
	b.sets = append(b.sets, fmt.Sprintf("%s = ?", field))
	b.values = append(b.values, value)
	return b
}

func (b *UpdateBuilder) SetMap(data map[string]any) *UpdateBuilder {
	for k, v := range data {
		b.Set(k, v)
	}
	return b
}

func (b *UpdateBuilder) Where(field string, value any) *UpdateBuilder {
	b.filters = append(b.filters, &Filter{Field: field, Op: OpEq, Value: value})
	return b
}

func (b *UpdateBuilder) Build() (string, []any) {
	var sb strings.Builder
	args := make([]any, 0, len(b.values)+len(b.filters))

	sb.WriteString("UPDATE ")
	sb.WriteString(b.table)
	sb.WriteString(" SET ")
	sb.WriteString(strings.Join(b.sets, ", "))

	args = append(args, b.values...)

	if len(b.filters) > 0 {
		sb.WriteString(" WHERE ")
		var conditions []string
		for _, f := range b.filters {
			conditions = append(conditions, fmt.Sprintf("%s = ?", f.Field))
			args = append(args, f.Value)
		}
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	return sb.String(), args
}

type DeleteBuilder struct {
	table   string
	filters []*Filter
}

func NewDelete(table string) *DeleteBuilder {
	return &DeleteBuilder{table: table}
}

func (b *DeleteBuilder) Where(field string, value any) *DeleteBuilder {
	b.filters = append(b.filters, &Filter{Field: field, Op: OpEq, Value: value})
	return b
}

func (b *DeleteBuilder) Build() (string, []any) {
	var sb strings.Builder
	var args []any

	sb.WriteString("DELETE FROM ")
	sb.WriteString(b.table)

	if len(b.filters) > 0 {
		sb.WriteString(" WHERE ")
		var conditions []string
		for _, f := range b.filters {
			conditions = append(conditions, fmt.Sprintf("%s = ?", f.Field))
			args = append(args, f.Value)
		}
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	return sb.String(), args
}

func ParseSortString(s string) (field string, order SortOrder) {
	if strings.HasPrefix(s, "-") {
		return s[1:], SortDesc
	}
	if strings.HasPrefix(s, "+") {
		return s[1:], SortAsc
	}
	return s, SortAsc
}

func ParseFilterString(s string) (*Filter, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid filter format: %s", s)
	}

	field := parts[0]
	op := FilterOp(parts[1])
	var value any
	if len(parts) > 2 {
		value = parts[2]
	}

	return &Filter{Field: field, Op: op, Value: value}, nil
}
