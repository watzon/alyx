package database

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrForeignKey      = errors.New("foreign key constraint failed")
	ErrUniqueViolation = errors.New("unique constraint violated")
	ErrNotNull         = errors.New("not null constraint failed")
	ErrCheckConstraint = errors.New("check constraint failed")
)

type ConstraintError struct {
	Type       string
	Table      string
	Column     string
	Referenced string
	Message    string
	Cause      error
}

func (e *ConstraintError) Error() string {
	return e.Message
}

func (e *ConstraintError) Unwrap() error {
	return e.Cause
}

var (
	fkPattern     = regexp.MustCompile(`FOREIGN KEY constraint failed`)
	fkDetailRegex = regexp.MustCompile(`foreign key mismatch - "([^"]+)" referencing "([^"]+)"`)
	uniquePattern = regexp.MustCompile(`UNIQUE constraint failed: ([^\s]+)`)
	notNullRegex  = regexp.MustCompile(`NOT NULL constraint failed: ([^\s]+)`)
	checkRegex    = regexp.MustCompile(`CHECK constraint failed`)
)

func ClassifyError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	if fkPattern.MatchString(errStr) {
		ce := &ConstraintError{
			Type:    "foreign_key",
			Cause:   ErrForeignKey,
			Message: "Referenced record does not exist",
		}
		if matches := fkDetailRegex.FindStringSubmatch(errStr); len(matches) == 3 {
			ce.Table = matches[1]
			ce.Referenced = matches[2]
			ce.Message = "Referenced record in '" + matches[2] + "' does not exist"
		}
		return ce
	}

	if matches := uniquePattern.FindStringSubmatch(errStr); len(matches) == 2 {
		parts := strings.Split(matches[1], ".")
		ce := &ConstraintError{
			Type:    "unique",
			Cause:   ErrUniqueViolation,
			Message: "A record with this value already exists",
		}
		if len(parts) == 2 {
			ce.Table = parts[0]
			ce.Column = parts[1]
			ce.Message = "A record with this '" + parts[1] + "' already exists"
		}
		return ce
	}

	if matches := notNullRegex.FindStringSubmatch(errStr); len(matches) == 2 {
		parts := strings.Split(matches[1], ".")
		ce := &ConstraintError{
			Type:    "not_null",
			Cause:   ErrNotNull,
			Message: "Required field is missing",
		}
		if len(parts) == 2 {
			ce.Table = parts[0]
			ce.Column = parts[1]
			ce.Message = "Field '" + parts[1] + "' is required"
		}
		return ce
	}

	if checkRegex.MatchString(errStr) {
		return &ConstraintError{
			Type:    "check",
			Cause:   ErrCheckConstraint,
			Message: "Value does not meet requirements",
		}
	}

	return err
}

func IsConstraintError(err error) bool {
	var ce *ConstraintError
	return errors.As(err, &ce)
}

func IsForeignKeyError(err error) bool {
	var ce *ConstraintError
	if errors.As(err, &ce) {
		return ce.Type == "foreign_key"
	}
	return false
}

func IsUniqueError(err error) bool {
	var ce *ConstraintError
	if errors.As(err, &ce) {
		return ce.Type == "unique"
	}
	return false
}

func IsNotNullError(err error) bool {
	var ce *ConstraintError
	if errors.As(err, &ce) {
		return ce.Type == "not_null"
	}
	return false
}

func AsConstraintError(err error) *ConstraintError {
	var ce *ConstraintError
	if errors.As(err, &ce) {
		return ce
	}
	return nil
}
