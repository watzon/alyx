// Package rules provides CEL-based access control for Alyx.
package rules

import (
	"errors"
	"fmt"
	"sync"

	"github.com/google/cel-go/cel"

	"github.com/watzon/alyx/internal/auth"
	"github.com/watzon/alyx/internal/schema"
)

var (
	ErrRuleNotFound    = errors.New("rule not found")
	ErrRuleEvaluation  = errors.New("rule evaluation failed")
	ErrAccessDenied    = errors.New("access denied")
	ErrInvalidRuleExpr = errors.New("invalid rule expression")
)

type Operation string

const (
	OpCreate   Operation = "create"
	OpRead     Operation = "read"
	OpUpdate   Operation = "update"
	OpDelete   Operation = "delete"
	OpDownload Operation = "download"
)

type Engine struct {
	env      *cel.Env
	programs map[string]cel.Program
	mu       sync.RWMutex
}

type EvalContext struct {
	Auth    map[string]any
	Doc     map[string]any
	File    map[string]any
	Request map[string]any
}

func NewEngine() (*Engine, error) {
	env, err := cel.NewEnv(
		cel.Variable("auth", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("doc", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("file", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("request", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, fmt.Errorf("creating CEL environment: %w", err)
	}

	return &Engine{
		env:      env,
		programs: make(map[string]cel.Program),
	}, nil
}

func (e *Engine) LoadSchema(s *schema.Schema) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for name, col := range s.Collections {
		if col.Rules == nil {
			continue
		}

		if col.Rules.Create != "" {
			if err := e.compileRule(name, OpCreate, col.Rules.Create); err != nil {
				return fmt.Errorf("compiling create rule for %s: %w", name, err)
			}
		}
		if col.Rules.Read != "" {
			if err := e.compileRule(name, OpRead, col.Rules.Read); err != nil {
				return fmt.Errorf("compiling read rule for %s: %w", name, err)
			}
		}
		if col.Rules.Update != "" {
			if err := e.compileRule(name, OpUpdate, col.Rules.Update); err != nil {
				return fmt.Errorf("compiling update rule for %s: %w", name, err)
			}
		}
		if col.Rules.Delete != "" {
			if err := e.compileRule(name, OpDelete, col.Rules.Delete); err != nil {
				return fmt.Errorf("compiling delete rule for %s: %w", name, err)
			}
		}
	}

	for name, bucket := range s.Buckets {
		if bucket.Rules == nil {
			continue
		}

		if bucket.Rules.Create != "" {
			if err := e.compileRule(name, OpCreate, bucket.Rules.Create); err != nil {
				return fmt.Errorf("compiling create rule for bucket %s: %w", name, err)
			}
		}
		if bucket.Rules.Read != "" {
			if err := e.compileRule(name, OpRead, bucket.Rules.Read); err != nil {
				return fmt.Errorf("compiling read rule for bucket %s: %w", name, err)
			}
		}
		if bucket.Rules.Update != "" {
			if err := e.compileRule(name, OpUpdate, bucket.Rules.Update); err != nil {
				return fmt.Errorf("compiling update rule for bucket %s: %w", name, err)
			}
		}
		if bucket.Rules.Delete != "" {
			if err := e.compileRule(name, OpDelete, bucket.Rules.Delete); err != nil {
				return fmt.Errorf("compiling delete rule for bucket %s: %w", name, err)
			}
		}
		if bucket.Rules.Download != "" {
			if err := e.compileRule(name, OpDownload, bucket.Rules.Download); err != nil {
				return fmt.Errorf("compiling download rule for bucket %s: %w", name, err)
			}
		}
	}

	return nil
}

func (e *Engine) compileRule(collection string, op Operation, expr string) error {
	ast, issues := e.env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("%w: %w", ErrInvalidRuleExpr, issues.Err())
	}

	program, err := e.env.Program(ast)
	if err != nil {
		return fmt.Errorf("creating program: %w", err)
	}

	key := ruleKey(collection, op)
	e.programs[key] = program
	return nil
}

func (e *Engine) Evaluate(collection string, op Operation, ctx *EvalContext) (bool, error) {
	e.mu.RLock()
	key := ruleKey(collection, op)
	program, ok := e.programs[key]
	e.mu.RUnlock()

	if !ok {
		return true, nil
	}

	vars := map[string]any{
		"auth":    ctx.Auth,
		"doc":     ctx.Doc,
		"file":    ctx.File,
		"request": ctx.Request,
	}

	if vars["auth"] == nil {
		vars["auth"] = map[string]any{}
	}
	if vars["doc"] == nil {
		vars["doc"] = map[string]any{}
	}
	if vars["file"] == nil {
		vars["file"] = map[string]any{}
	}
	if vars["request"] == nil {
		vars["request"] = map[string]any{}
	}

	result, _, err := program.Eval(vars)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrRuleEvaluation, err)
	}

	allowed, ok := result.Value().(bool)
	if !ok {
		return false, fmt.Errorf("%w: rule did not return boolean", ErrRuleEvaluation)
	}

	return allowed, nil
}

func (e *Engine) CheckAccess(collection string, op Operation, ctx *EvalContext) error {
	allowed, err := e.Evaluate(collection, op, ctx)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrAccessDenied
	}
	return nil
}

func (e *Engine) HasRule(collection string, op Operation) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.programs[ruleKey(collection, op)]
	return ok
}

func ruleKey(collection string, op Operation) string {
	return collection + ":" + string(op)
}

func BuildAuthContext(user *auth.User, claims *auth.Claims) map[string]any {
	if user == nil && claims == nil {
		return nil
	}

	authCtx := make(map[string]any)

	if user != nil {
		authCtx["id"] = user.ID
		authCtx["email"] = user.Email
		authCtx["verified"] = user.Verified
		if user.Role != "" {
			authCtx["role"] = user.Role
		}
		if user.Metadata != nil {
			authCtx["metadata"] = user.Metadata
		}
	} else if claims != nil {
		authCtx["id"] = claims.UserID
		authCtx["email"] = claims.Email
		authCtx["verified"] = claims.Verified
		if claims.Role != "" {
			authCtx["role"] = claims.Role
		}
	}

	return authCtx
}

func BuildRequestContext(method, ip string) map[string]any {
	return map[string]any{
		"method": method,
		"ip":     ip,
	}
}
