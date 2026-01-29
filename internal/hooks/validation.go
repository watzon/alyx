package hooks

import (
	"fmt"
	"strings"
)

type FunctionChecker interface {
	FunctionExists(name string) bool
}

type Validator struct {
	funcChecker FunctionChecker
}

func NewValidator(funcChecker FunctionChecker) *Validator {
	return &Validator{funcChecker: funcChecker}
}

func (v *Validator) ValidateHook(hook *Hook) error {
	if err := v.validateType(hook.Type); err != nil {
		return err
	}

	if err := v.validateMode(hook.Mode); err != nil {
		return err
	}

	if hook.Source == "" {
		return fmt.Errorf("source cannot be empty")
	}

	if hook.FunctionName == "" {
		return fmt.Errorf("function_name cannot be empty")
	}

	if v.funcChecker != nil && !v.funcChecker.FunctionExists(hook.FunctionName) {
		return fmt.Errorf("function not found: %s", hook.FunctionName)
	}

	switch hook.Type {
	case HookTypeDatabase:
		return v.validateDatabaseHook(hook)
	case HookTypeWebhook:
		return v.validateWebhookHook(hook)
	case HookTypeSchedule:
		return v.validateScheduleHook(hook)
	}

	return nil
}

func (v *Validator) validateType(hookType HookType) error {
	switch hookType {
	case HookTypeDatabase, HookTypeWebhook, HookTypeSchedule:
		return nil
	default:
		return fmt.Errorf("invalid hook type: %s (must be database, webhook, or schedule)", hookType)
	}
}

func (v *Validator) validateMode(mode HookMode) error {
	switch mode {
	case HookModeSync, HookModeAsync:
		return nil
	default:
		return fmt.Errorf("invalid hook mode: %s (must be sync or async)", mode)
	}
}

func (v *Validator) validateDatabaseHook(hook *Hook) error {
	if hook.Action == "" {
		return fmt.Errorf("database hook requires action field")
	}

	action := strings.ToLower(hook.Action)
	switch DatabaseAction(action) {
	case ActionInsert, ActionUpdate, ActionDelete:
		return nil
	default:
		return fmt.Errorf("invalid database action: %s (must be insert, update, or delete)", hook.Action)
	}
}

func (v *Validator) validateWebhookHook(hook *Hook) error {
	if hook.Action != "" {
		return fmt.Errorf("webhook hook should not have action field")
	}
	return nil
}

func (v *Validator) validateScheduleHook(hook *Hook) error {
	if hook.Action != "" {
		return fmt.Errorf("schedule hook should not have action field")
	}
	return nil
}
