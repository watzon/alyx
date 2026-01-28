package functions

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/watzon/alyx/internal/schema"
)

const (
	defaultTimeout = 30
	defaultMemory  = 128
)

func SchemaToFunctionDefs(s *schema.Schema, functionsDir string) ([]*FunctionDef, error) {
	if s.Functions == nil {
		return []*FunctionDef{}, nil
	}

	defs := make([]*FunctionDef, 0, len(s.Functions))

	for name, fn := range s.Functions {
		def, err := convertFunction(name, fn, functionsDir)
		if err != nil {
			return nil, fmt.Errorf("converting function %q: %w", name, err)
		}
		defs = append(defs, def)
	}

	return defs, nil
}

func convertFunction(name string, fn *schema.Function, functionsDir string) (*FunctionDef, error) {
	path := fn.Path
	if path == "" {
		path = filepath.Join(functionsDir, name)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("function directory does not exist: %s", path)
	}

	entrypointPath := filepath.Join(path, fn.Entrypoint)
	if _, err := os.Stat(entrypointPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("entrypoint file does not exist: %s", entrypointPath)
	}

	timeout, err := parseTimeout(fn.Timeout)
	if err != nil {
		return nil, fmt.Errorf("parsing timeout: %w", err)
	}

	memory, err := parseMemory(fn.Memory)
	if err != nil {
		return nil, fmt.Errorf("parsing memory: %w", err)
	}

	runtime := Runtime(fn.Runtime)

	hooks := make([]HookConfig, len(fn.Hooks))
	for i, h := range fn.Hooks {
		hooks[i] = HookConfig{
			Type:   h.Type,
			Source: h.Source,
			Action: h.Action,
			Mode:   h.Mode,
		}
		if h.Verification != nil {
			hooks[i].Verification = &VerificationConfig{
				Type:   h.Verification.Type,
				Header: h.Verification.Header,
				Secret: h.Verification.Secret,
			}
		}
	}

	schedules := make([]ScheduleConfig, len(fn.Schedules))
	for i, s := range fn.Schedules {
		schedules[i] = ScheduleConfig{
			Name:       s.Name,
			Type:       s.Type,
			Expression: s.Expression,
			Timezone:   s.Timezone,
			Config:     s.Config,
			Input:      s.Input,
		}
	}

	routes := make([]RouteConfig, len(fn.Routes))
	for i, r := range fn.Routes {
		routes[i] = RouteConfig{
			Path:    r.Path,
			Methods: r.Methods,
		}
	}

	var build *BuildConfig
	var outputPath string
	if fn.Build != nil {
		build = &BuildConfig{
			Command: fn.Build.Command,
			Args:    fn.Build.Args,
			Watch:   fn.Build.Watch,
			Output:  fn.Build.Output,
		}
		if build.Output != "" {
			outputPath = filepath.Join(path, build.Output)
		}
	}

	return &FunctionDef{
		Name:        name,
		Runtime:     runtime,
		Path:        path,
		OutputPath:  outputPath,
		HasBuild:    build != nil,
		Timeout:     timeout,
		Memory:      memory,
		Env:         fn.Env,
		HasManifest: true,
		Routes:      routes,
		Hooks:       hooks,
		Schedules:   schedules,
	}, nil
}

func parseTimeout(s string) (int, error) {
	if s == "" {
		return defaultTimeout, nil
	}

	if val, err := strconv.Atoi(s); err == nil {
		return val, nil
	}

	if strings.HasSuffix(s, "s") {
		val, err := strconv.Atoi(strings.TrimSuffix(s, "s"))
		if err != nil {
			return 0, fmt.Errorf("invalid timeout format: %s", s)
		}
		return val, nil
	}

	if strings.HasSuffix(s, "m") {
		val, err := strconv.Atoi(strings.TrimSuffix(s, "m"))
		if err != nil {
			return 0, fmt.Errorf("invalid timeout format: %s", s)
		}
		return val * 60, nil
	}

	return 0, fmt.Errorf("invalid timeout format: %s (use 30s, 5m, or 30)", s)
}

func parseMemory(s string) (int, error) {
	if s == "" {
		return defaultMemory, nil
	}

	if val, err := strconv.Atoi(s); err == nil {
		return val, nil
	}

	lower := strings.ToLower(s)

	if strings.HasSuffix(lower, "mb") {
		val, err := strconv.Atoi(strings.TrimSuffix(lower, "mb"))
		if err != nil {
			return 0, fmt.Errorf("invalid memory format: %s", s)
		}
		return val, nil
	}

	if strings.HasSuffix(lower, "gb") {
		val, err := strconv.Atoi(strings.TrimSuffix(lower, "gb"))
		if err != nil {
			return 0, fmt.Errorf("invalid memory format: %s", s)
		}
		return val * 1024, nil
	}

	return 0, fmt.Errorf("invalid memory format: %s (use 128mb, 1gb, or 128)", s)
}
