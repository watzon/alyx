package schema

import (
	"strings"
	"testing"
)

func TestParseFunctions_ValidMinimal(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  hello:
    runtime: node
    entrypoint: index.js
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	fn, ok := schema.Functions["hello"]
	if !ok {
		t.Fatal("hello function not found")
	}

	if fn.Name != "hello" {
		t.Errorf("expected name 'hello', got %q", fn.Name)
	}

	if fn.Runtime != "node" {
		t.Errorf("expected runtime 'node', got %q", fn.Runtime)
	}

	if fn.Entrypoint != "index.js" {
		t.Errorf("expected entrypoint 'index.js', got %q", fn.Entrypoint)
	}
}

func TestParseFunctions_ValidFull(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  process_user:
    runtime: python
    entrypoint: main.py
    timeout: 30s
    memory: 512mb
    env:
      API_KEY: ${API_KEY}
      DEBUG: "true"
    rules:
      invoke: "auth.id != null"
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	fn, ok := schema.Functions["process_user"]
	if !ok {
		t.Fatal("process_user function not found")
	}

	if fn.Runtime != "python" {
		t.Errorf("expected runtime 'python', got %q", fn.Runtime)
	}

	if fn.Timeout != "30s" {
		t.Errorf("expected timeout '30s', got %q", fn.Timeout)
	}

	if fn.Memory != "512mb" {
		t.Errorf("expected memory '512mb', got %q", fn.Memory)
	}

	if fn.Env["API_KEY"] != "${API_KEY}" {
		t.Errorf("expected env API_KEY '${API_KEY}', got %q", fn.Env["API_KEY"])
	}

	if fn.Rules == nil {
		t.Fatal("expected rules to be set")
	}

	if fn.Rules.Invoke != "auth.id != null" {
		t.Errorf("expected invoke rule 'auth.id != null', got %q", fn.Rules.Invoke)
	}
}

func TestParseFunctions_DatabaseHook(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  on_user_created:
    runtime: node
    entrypoint: index.js
    hooks:
      - type: database
        source: users
        action: insert
        mode: async
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	fn := schema.Functions["on_user_created"]
	if len(fn.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(fn.Hooks))
	}

	hook := fn.Hooks[0]
	if hook.Type != "database" {
		t.Errorf("expected hook type 'database', got %q", hook.Type)
	}

	if hook.Source != "users" {
		t.Errorf("expected hook source 'users', got %q", hook.Source)
	}

	if hook.Action != "insert" {
		t.Errorf("expected hook action 'insert', got %q", hook.Action)
	}

	if hook.Mode != "async" {
		t.Errorf("expected hook mode 'async', got %q", hook.Mode)
	}
}

func TestParseFunctions_AuthHook(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  on_signup:
    runtime: go
    entrypoint: main.go
    hooks:
      - type: auth
        source: signup
        action: after
        mode: sync
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	fn := schema.Functions["on_signup"]
	if len(fn.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(fn.Hooks))
	}

	hook := fn.Hooks[0]
	if hook.Type != "auth" {
		t.Errorf("expected hook type 'auth', got %q", hook.Type)
	}

	if hook.Source != "signup" {
		t.Errorf("expected hook source 'signup', got %q", hook.Source)
	}

	if hook.Action != "after" {
		t.Errorf("expected hook action 'after', got %q", hook.Action)
	}

	if hook.Mode != "sync" {
		t.Errorf("expected hook mode 'sync', got %q", hook.Mode)
	}
}

func TestParseFunctions_WebhookHook(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  stripe_webhook:
    runtime: node
    entrypoint: index.js
    hooks:
      - type: webhook
        verification:
          type: hmac-sha256
          header: X-Stripe-Signature
          secret: ${STRIPE_SECRET}
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	fn := schema.Functions["stripe_webhook"]
	if len(fn.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(fn.Hooks))
	}

	hook := fn.Hooks[0]
	if hook.Type != "webhook" {
		t.Errorf("expected hook type 'webhook', got %q", hook.Type)
	}

	if hook.Verification == nil {
		t.Fatal("expected verification config")
	}

	if hook.Verification.Type != "hmac-sha256" {
		t.Errorf("expected verification type 'hmac-sha256', got %q", hook.Verification.Type)
	}

	if hook.Verification.Header != "X-Stripe-Signature" {
		t.Errorf("expected verification header 'X-Stripe-Signature', got %q", hook.Verification.Header)
	}

	if hook.Verification.Secret != "${STRIPE_SECRET}" {
		t.Errorf("expected verification secret '${STRIPE_SECRET}', got %q", hook.Verification.Secret)
	}
}

func TestParseFunctions_CronSchedule(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  daily_cleanup:
    runtime: python
    entrypoint: cleanup.py
    schedules:
      - name: cleanup
        type: cron
        expression: "0 2 * * *"
        timezone: America/New_York
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	fn := schema.Functions["daily_cleanup"]
	if len(fn.Schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(fn.Schedules))
	}

	sched := fn.Schedules[0]
	if sched.Name != "cleanup" {
		t.Errorf("expected schedule name 'cleanup', got %q", sched.Name)
	}

	if sched.Type != "cron" {
		t.Errorf("expected schedule type 'cron', got %q", sched.Type)
	}

	if sched.Expression != "0 2 * * *" {
		t.Errorf("expected expression '0 2 * * *', got %q", sched.Expression)
	}

	if sched.Timezone != "America/New_York" {
		t.Errorf("expected timezone 'America/New_York', got %q", sched.Timezone)
	}
}

func TestParseFunctions_IntervalSchedule(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  health_check:
    runtime: node
    entrypoint: health.js
    schedules:
      - name: check
        type: interval
        expression: "5m"
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	fn := schema.Functions["health_check"]
	if len(fn.Schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(fn.Schedules))
	}

	sched := fn.Schedules[0]
	if sched.Type != "interval" {
		t.Errorf("expected schedule type 'interval', got %q", sched.Type)
	}

	if sched.Expression != "5m" {
		t.Errorf("expected expression '5m', got %q", sched.Expression)
	}
}

func TestParseFunctions_OneTimeSchedule(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  migration:
    runtime: go
    entrypoint: migrate.go
    schedules:
      - name: run_once
        type: one_time
        expression: "2026-12-31T23:59:59Z"
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	fn := schema.Functions["migration"]
	if len(fn.Schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(fn.Schedules))
	}

	sched := fn.Schedules[0]
	if sched.Type != "one_time" {
		t.Errorf("expected schedule type 'one_time', got %q", sched.Type)
	}

	if sched.Expression != "2026-12-31T23:59:59Z" {
		t.Errorf("expected expression '2026-12-31T23:59:59Z', got %q", sched.Expression)
	}
}

func TestParseFunctions_HTTPRoute(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  api_handler:
    runtime: node
    entrypoint: api.js
    routes:
      - path: /api/custom
        methods: [GET, POST]
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	fn := schema.Functions["api_handler"]
	if len(fn.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(fn.Routes))
	}

	route := fn.Routes[0]
	if route.Path != "/api/custom" {
		t.Errorf("expected path '/api/custom', got %q", route.Path)
	}

	if len(route.Methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(route.Methods))
	}

	if route.Methods[0] != "GET" || route.Methods[1] != "POST" {
		t.Errorf("expected methods [GET, POST], got %v", route.Methods)
	}
}

func TestParseFunctions_BuildConfig(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  typescript_fn:
    runtime: node
    entrypoint: dist/index.js
    build:
      command: tsc
      args: ["src/index.ts", "--outDir", "dist"]
      watch: ["src/**/*.ts"]
      output: dist/index.js
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	fn := schema.Functions["typescript_fn"]
	if fn.Build == nil {
		t.Fatal("expected build config")
	}

	if fn.Build.Command != "tsc" {
		t.Errorf("expected build command 'tsc', got %q", fn.Build.Command)
	}

	if fn.Build.Output != "dist/index.js" {
		t.Errorf("expected build output 'dist/index.js', got %q", fn.Build.Output)
	}

	if len(fn.Build.Args) != 3 {
		t.Errorf("expected 3 build args, got %d", len(fn.Build.Args))
	}

	if len(fn.Build.Watch) != 1 {
		t.Errorf("expected 1 watch pattern, got %d", len(fn.Build.Watch))
	}
}

func TestParseFunctions_MultipleFunctions(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  hello:
    runtime: node
    entrypoint: hello.js
  goodbye:
    runtime: python
    entrypoint: goodbye.py
  process:
    runtime: go
    entrypoint: main.go
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	if len(schema.Functions) != 3 {
		t.Errorf("expected 3 functions, got %d", len(schema.Functions))
	}

	if _, ok := schema.Functions["hello"]; !ok {
		t.Error("hello function not found")
	}

	if _, ok := schema.Functions["goodbye"]; !ok {
		t.Error("goodbye function not found")
	}

	if _, ok := schema.Functions["process"]; !ok {
		t.Error("process function not found")
	}
}

func TestParseFunctions_SchemaWithoutFunctions(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	if schema.Functions == nil {
		t.Error("expected Functions map to be initialized")
	}

	if len(schema.Functions) != 0 {
		t.Errorf("expected 0 functions, got %d", len(schema.Functions))
	}
}

func TestParseFunctions_EmptyFunctions(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions: {}
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	if len(schema.Functions) != 0 {
		t.Errorf("expected 0 functions, got %d", len(schema.Functions))
	}
}

func TestValidation_MissingRuntime(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  hello:
    entrypoint: index.js
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for missing runtime")
	}
	if err != nil && !strings.Contains(err.Error(), "runtime") {
		t.Errorf("expected error about missing runtime, got: %v", err)
	}
}

func TestValidation_MissingEntrypoint(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  hello:
    runtime: node
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for missing entrypoint")
	}
	if err != nil && !strings.Contains(err.Error(), "entrypoint") {
		t.Errorf("expected error about missing entrypoint, got: %v", err)
	}
}

func TestValidation_InvalidRuntime(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  hello:
    runtime: ruby
    entrypoint: hello.rb
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for invalid runtime")
	}
	if err != nil && !strings.Contains(err.Error(), "runtime") {
		t.Errorf("expected error about invalid runtime, got: %v", err)
	}
}

func TestValidation_InvalidHookType(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  hello:
    runtime: node
    entrypoint: index.js
    hooks:
      - type: invalid
        source: users
        action: insert
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for invalid hook type")
	}
	if err != nil && !strings.Contains(err.Error(), "hook") && !strings.Contains(err.Error(), "type") {
		t.Errorf("expected error about invalid hook type, got: %v", err)
	}
}

func TestValidation_InvalidScheduleType(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  hello:
    runtime: node
    entrypoint: index.js
    schedules:
      - name: test
        type: invalid
        expression: "* * * * *"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for invalid schedule type")
	}
	if err != nil && !strings.Contains(err.Error(), "schedule") && !strings.Contains(err.Error(), "type") {
		t.Errorf("expected error about invalid schedule type, got: %v", err)
	}
}

func TestValidation_HookReferencesNonExistentCollection(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  hello:
    runtime: node
    entrypoint: index.js
    hooks:
      - type: database
        source: posts
        action: insert
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for hook referencing non-existent collection")
	}
	if err != nil && !strings.Contains(err.Error(), "collection") && !strings.Contains(err.Error(), "posts") {
		t.Errorf("expected error about non-existent collection 'posts', got: %v", err)
	}
}

func TestValidation_FunctionNameCollidesWithCollection(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  users:
    runtime: node
    entrypoint: index.js
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for function name collision with collection")
	}
	if err != nil && !strings.Contains(err.Error(), "collection") && !strings.Contains(err.Error(), "users") {
		t.Errorf("expected error about name collision with collection, got: %v", err)
	}
}

func TestValidation_FunctionNameCollidesWithBucket(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

buckets:
  avatars:
    backend: local

functions:
  avatars:
    runtime: node
    entrypoint: index.js
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for function name collision with bucket")
	}
	if err != nil && !strings.Contains(err.Error(), "bucket") && !strings.Contains(err.Error(), "avatars") {
		t.Errorf("expected error about name collision with bucket, got: %v", err)
	}
}

func TestValidation_InvalidFunctionName(t *testing.T) {
	tests := []struct {
		name         string
		functionName string
		wantError    bool
	}{
		{"uppercase", "MyFunction", true},
		{"starts with number", "1function", true},
		{"hyphens allowed", "my-function", false},
		{"special chars space", "my function", true},
		{"valid lowercase", "my_function", false},
		{"valid with numbers", "function_123", false},
		{"reserved prefix", "_alyx_function", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  ` + tt.functionName + `:
    runtime: node
    entrypoint: index.js
`
			_, err := Parse([]byte(yaml))
			if tt.wantError && err == nil {
				t.Errorf("expected validation error for function name %q", tt.functionName)
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error for valid function name %q: %v", tt.functionName, err)
			}
		})
	}
}

func TestValidation_RoutePathMissingLeadingSlash(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  api:
    runtime: node
    entrypoint: index.js
    routes:
      - path: api/test
        methods: [GET]
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for route path without leading slash")
	}
	if err != nil && !strings.Contains(err.Error(), "path") && !strings.Contains(err.Error(), "/") {
		t.Errorf("expected error about route path format, got: %v", err)
	}
}

func TestValidation_WebhookMissingVerification(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  webhook:
    runtime: node
    entrypoint: index.js
    hooks:
      - type: webhook
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for webhook hook without verification")
	}
	if err != nil && !strings.Contains(err.Error(), "verification") {
		t.Errorf("expected error about missing verification, got: %v", err)
	}
}

func TestValidation_AllRuntimes(t *testing.T) {
	runtimes := []string{"node", "python", "go", "deno", "bun"}

	for _, runtime := range runtimes {
		t.Run(runtime, func(t *testing.T) {
			yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  test:
    runtime: ` + runtime + `
    entrypoint: index.js
`
			_, err := Parse([]byte(yaml))
			if err != nil {
				t.Errorf("expected runtime %q to be valid, got error: %v", runtime, err)
			}
		})
	}
}

func TestValidation_AllHookTypes(t *testing.T) {
	tests := []struct {
		hookType string
		yaml     string
	}{
		{
			"database",
			`
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  test:
    runtime: node
    entrypoint: index.js
    hooks:
      - type: database
        source: users
        action: insert
`,
		},
		{
			"auth",
			`
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  test:
    runtime: node
    entrypoint: index.js
    hooks:
      - type: auth
        source: signup
        action: after
`,
		},
		{
			"webhook",
			`
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  test:
    runtime: node
    entrypoint: index.js
    hooks:
      - type: webhook
        verification:
          type: hmac-sha256
          header: X-Signature
          secret: secret
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.hookType, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err != nil {
				t.Errorf("expected hook type %q to be valid, got error: %v", tt.hookType, err)
			}
		})
	}
}

func TestValidation_AllScheduleTypes(t *testing.T) {
	tests := []struct {
		scheduleType string
		yaml         string
	}{
		{
			"cron",
			`
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  test:
    runtime: node
    entrypoint: index.js
    schedules:
      - name: test
        type: cron
        expression: "0 * * * *"
`,
		},
		{
			"interval",
			`
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  test:
    runtime: node
    entrypoint: index.js
    schedules:
      - name: test
        type: interval
        expression: "5m"
`,
		},
		{
			"one_time",
			`
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

functions:
  test:
    runtime: node
    entrypoint: index.js
    schedules:
      - name: test
        type: one_time
        expression: "2026-12-31T23:59:59Z"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.scheduleType, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err != nil {
				t.Errorf("expected schedule type %q to be valid, got error: %v", tt.scheduleType, err)
			}
		})
	}
}
