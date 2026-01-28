package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/auth"
	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/deploy"
	"github.com/watzon/alyx/internal/functions"
	"github.com/watzon/alyx/internal/schema"
)

// AdminHandlers handles admin API endpoints.
type AdminHandlers struct {
	deployService *deploy.Service
	authService   *auth.Service
	db            *database.DB
	schema        *schema.Schema
	funcService   *functions.Service
	cfg           *config.Config
	schemaPath    string
	configPath    string
	startTime     time.Time
	pendingStore  *schema.PendingChangesStore
	migrator      *schema.Migrator
	draftSchemas  map[string]string // session_id -> draft YAML content
}

// NewAdminHandlers creates new admin handlers.
func NewAdminHandlers(deployService *deploy.Service, authService *auth.Service, db *database.DB, sch *schema.Schema, funcService *functions.Service, cfg *config.Config, schemaPath, configPath string) *AdminHandlers {
	h := &AdminHandlers{
		deployService: deployService,
		authService:   authService,
		db:            db,
		schema:        sch,
		funcService:   funcService,
		cfg:           cfg,
		schemaPath:    schemaPath,
		configPath:    configPath,
		startTime:     time.Now(),
		draftSchemas:  make(map[string]string),
	}

	if db != nil && db.DB != nil {
		h.pendingStore = schema.NewPendingChangesStore(db.DB)
		if err := h.pendingStore.Init(); err != nil {
			log.Error().Err(err).Msg("Failed to initialize pending changes store")
		}
		h.migrator = schema.NewMigrator(db.DB, schemaPath, "")
	}

	return h
}

// requireAdminAuth validates either a JWT token from an admin user or a deploy token.
// JWT-authenticated admin users have all permissions.
func (h *AdminHandlers) requireAdminAuth(r *http.Request, perm deploy.TokenPermission) (*deploy.AdminToken, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errors.New("missing authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, errors.New("invalid authorization header format")
	}

	tokenStr := parts[1]

	if h.authService != nil {
		claims, err := h.authService.ValidateToken(tokenStr)
		if err == nil && claims != nil {
			return &deploy.AdminToken{
				Name:        "jwt:" + claims.Email,
				Permissions: []string{string(deploy.PermissionAdmin), string(deploy.PermissionDeploy), string(deploy.PermissionRollback)},
			}, nil
		}
	}

	token, err := h.deployService.ValidateToken(tokenStr)
	if err != nil {
		return nil, errors.New("invalid token")
	}

	if !token.HasPermission(perm) {
		return nil, errors.New("insufficient permissions")
	}

	return token, nil
}

func (h *AdminHandlers) Stats(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionDeploy)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	uptime := int64(time.Since(h.startTime).Seconds())

	var userCount int
	row := h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM _alyx_users")
	_ = row.Scan(&userCount)

	var docCount int
	if h.schema != nil {
		for _, col := range h.schema.Collections {
			var count int
			row := h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM "+col.Name)
			if row.Scan(&count) == nil {
				docCount += count
			}
		}
	}

	var funcCount int
	if h.funcService != nil {
		funcCount = len(h.funcService.ListFunctions())
	}

	var collectionCount int
	if h.schema != nil {
		collectionCount = len(h.schema.Collections)
	}

	JSON(w, http.StatusOK, map[string]any{
		"uptime":      uptime,
		"collections": collectionCount,
		"documents":   docCount,
		"users":       userCount,
		"functions":   funcCount,
	})
}

// DeployPrepare handles POST /api/admin/deploy/prepare.
func (h *AdminHandlers) DeployPrepare(w http.ResponseWriter, r *http.Request) {
	token, err := h.requireAdminAuth(r, deploy.PermissionDeploy)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	var req deploy.PrepareRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	log.Debug().
		Str("token_name", token.Name).
		Str("schema_hash", req.SchemaHash).
		Str("functions_hash", req.FunctionsHash).
		Msg("Deploy prepare request")

	resp, err := h.deployService.Prepare(&req)
	if err != nil {
		log.Error().Err(err).Msg("Deploy prepare failed")
		Error(w, http.StatusInternalServerError, "PREPARE_ERROR", err.Error())
		return
	}

	JSON(w, http.StatusOK, resp)
}

// DeployExecute handles POST /api/admin/deploy/execute.
func (h *AdminHandlers) DeployExecute(w http.ResponseWriter, r *http.Request) {
	token, err := h.requireAdminAuth(r, deploy.PermissionDeploy)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	var req deploy.ExecuteRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	if req.Schema == "" {
		Error(w, http.StatusBadRequest, "MISSING_SCHEMA", "Schema is required")
		return
	}

	log.Info().
		Str("token_name", token.Name).
		Str("schema_hash", req.SchemaHash).
		Str("description", req.Description).
		Msg("Deploy execute request")

	resp, err := h.deployService.Execute(&req, token.Name)
	if err != nil {
		log.Error().Err(err).Msg("Deploy execute failed")
		Error(w, http.StatusInternalServerError, "DEPLOY_ERROR", err.Error())
		return
	}

	JSON(w, http.StatusOK, resp)
}

// DeployRollback handles POST /api/admin/deploy/rollback.
func (h *AdminHandlers) DeployRollback(w http.ResponseWriter, r *http.Request) {
	token, err := h.requireAdminAuth(r, deploy.PermissionRollback)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	var req deploy.RollbackRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	log.Info().
		Str("token_name", token.Name).
		Str("to_version", req.ToVersion).
		Str("reason", req.Reason).
		Msg("Deploy rollback request")

	resp, err := h.deployService.Rollback(&req, token.Name)
	if err != nil {
		log.Error().Err(err).Msg("Deploy rollback failed")
		Error(w, http.StatusInternalServerError, "ROLLBACK_ERROR", err.Error())
		return
	}

	JSON(w, http.StatusOK, resp)
}

// DeployHistory handles GET /api/admin/deploy/history.
func (h *AdminHandlers) DeployHistory(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionDeploy)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	req := deploy.HistoryRequest{
		Limit:  10,
		Status: r.URL.Query().Get("status"),
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		var limit int
		if _, scanErr := json.Number(limitStr).Int64(); scanErr == nil {
			limit = int(mustParseInt(limitStr))
			if limit > 0 {
				req.Limit = limit
			}
		}
	}

	resp, err := h.deployService.History(&req)
	if err != nil {
		log.Error().Err(err).Msg("Deploy history failed")
		Error(w, http.StatusInternalServerError, "HISTORY_ERROR", err.Error())
		return
	}

	JSON(w, http.StatusOK, resp)
}

// TokenCreate handles POST /api/admin/tokens.
func (h *AdminHandlers) TokenCreate(w http.ResponseWriter, r *http.Request) {
	// For token creation, we require an existing admin token
	creatorToken, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	var req deploy.CreateTokenRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	if req.Name == "" {
		Error(w, http.StatusBadRequest, "MISSING_NAME", "Token name is required")
		return
	}

	if len(req.Permissions) == 0 {
		req.Permissions = []string{string(deploy.PermissionDeploy)}
	}

	log.Info().
		Str("creator", creatorToken.Name).
		Str("name", req.Name).
		Strs("permissions", req.Permissions).
		Msg("Creating admin token")

	resp, err := h.deployService.CreateToken(&req, creatorToken.Name)
	if err != nil {
		log.Error().Err(err).Msg("Token creation failed")
		Error(w, http.StatusInternalServerError, "TOKEN_ERROR", err.Error())
		return
	}

	JSON(w, http.StatusCreated, resp)
}

// TokenList handles GET /api/admin/tokens.
func (h *AdminHandlers) TokenList(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	tokens, err := h.deployService.ListTokens()
	if err != nil {
		log.Error().Err(err).Msg("Token list failed")
		Error(w, http.StatusInternalServerError, "TOKEN_ERROR", err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"tokens": tokens,
		"total":  len(tokens),
	})
}

// TokenDelete handles DELETE /api/admin/tokens/{name}.
func (h *AdminHandlers) TokenDelete(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	name := r.PathValue("name")
	if name == "" {
		Error(w, http.StatusBadRequest, "MISSING_NAME", "Token name is required")
		return
	}

	if err := h.deployService.DeleteToken(name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			Error(w, http.StatusNotFound, "TOKEN_NOT_FOUND", "Token not found")
			return
		}
		log.Error().Err(err).Str("name", name).Msg("Token deletion failed")
		Error(w, http.StatusInternalServerError, "TOKEN_ERROR", err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"deleted": true,
		"name":    name,
	})
}

// SchemaGet handles GET /api/admin/schema.
func (h *AdminHandlers) SchemaGet(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionDeploy)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	if h.schema == nil {
		JSON(w, http.StatusOK, map[string]any{
			"version":     0,
			"collections": []any{},
		})
		return
	}

	collections := make([]map[string]any, 0, len(h.schema.Collections))
	for _, col := range h.schema.Collections {
		collections = append(collections, serializeCollection(col))
	}
	// Sort collections by name
	sort.Slice(collections, func(i, j int) bool {
		return collections[i]["name"].(string) < collections[j]["name"].(string)
	})

	buckets := make([]map[string]any, 0, len(h.schema.Buckets))
	for name, bucket := range h.schema.Buckets {
		b := map[string]any{
			"name":    name,
			"backend": bucket.Backend,
		}
		if bucket.MaxFileSize > 0 {
			b["maxFileSize"] = bucket.MaxFileSize
		}
		if len(bucket.AllowedTypes) > 0 {
			b["allowedTypes"] = bucket.AllowedTypes
		}
		buckets = append(buckets, b)
	}
	// Sort buckets by name
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i]["name"].(string) < buckets[j]["name"].(string)
	})

	JSON(w, http.StatusOK, map[string]any{
		"version":     h.schema.Version,
		"collections": collections,
		"buckets":     buckets,
	})
}

func serializeCollection(col *schema.Collection) map[string]any {
	fields := make([]map[string]any, 0, len(col.Fields))
	for _, f := range col.OrderedFields() {
		fields = append(fields, serializeField(f))
	}

	collection := map[string]any{
		"name":   col.Name,
		"fields": fields,
	}

	if len(col.Indexes) > 0 {
		indexes := make([]map[string]any, 0, len(col.Indexes))
		for _, idx := range col.Indexes {
			indexes = append(indexes, map[string]any{
				"name":   idx.Name,
				"fields": idx.Fields,
				"unique": idx.Unique,
			})
		}
		collection["indexes"] = indexes
	}

	if rules := serializeRules(col.Rules); len(rules) > 0 {
		collection["rules"] = rules
	}

	return collection
}

func serializeField(f *schema.Field) map[string]any {
	field := map[string]any{
		"name":     f.Name,
		"type":     string(f.Type),
		"primary":  f.Primary,
		"unique":   f.Unique,
		"nullable": f.Nullable,
		"index":    f.Index,
	}
	if f.Default != "" {
		field["default"] = f.Default
	}
	if f.References != "" {
		field["references"] = f.References
	}
	if f.OnDelete != "" {
		field["onDelete"] = string(f.OnDelete)
	}
	if f.Validate != nil {
		validate := map[string]any{}
		if f.Validate.MinLength != nil {
			validate["minLength"] = *f.Validate.MinLength
		}
		if f.Validate.MaxLength != nil {
			validate["maxLength"] = *f.Validate.MaxLength
		}
		if f.Validate.Min != nil {
			validate["min"] = *f.Validate.Min
		}
		if f.Validate.Max != nil {
			validate["max"] = *f.Validate.Max
		}
		if f.Validate.Format != "" {
			validate["format"] = f.Validate.Format
		}
		if f.Validate.Pattern != "" {
			validate["pattern"] = f.Validate.Pattern
		}
		if len(f.Validate.Enum) > 0 {
			validate["enum"] = f.Validate.Enum
		}
		if len(validate) > 0 {
			field["validate"] = validate
		}
	}
	if f.RichText != nil {
		richtext := map[string]any{}
		if f.RichText.Preset != "" {
			richtext["preset"] = string(f.RichText.Preset)
		}
		if len(f.RichText.Allow) > 0 {
			allow := make([]string, len(f.RichText.Allow))
			for i, a := range f.RichText.Allow {
				allow[i] = string(a)
			}
			richtext["allow"] = allow
		}
		if len(f.RichText.Deny) > 0 {
			deny := make([]string, len(f.RichText.Deny))
			for i, d := range f.RichText.Deny {
				deny[i] = string(d)
			}
			richtext["deny"] = deny
		}
		field["richtext"] = richtext
	}
	if f.Select != nil {
		selectConfig := map[string]any{
			"values": f.Select.Values,
		}
		if f.Select.MaxSelect != 0 {
			selectConfig["maxSelect"] = f.Select.MaxSelect
		}
		field["select"] = selectConfig
	}
	if f.Relation != nil {
		relation := map[string]any{
			"collection": f.Relation.Collection,
		}
		if f.Relation.Field != "" {
			relation["field"] = f.Relation.Field
		}
		if f.Relation.OnDelete != "" {
			relation["onDelete"] = string(f.Relation.OnDelete)
		}
		if f.Relation.DisplayName != "" {
			relation["displayName"] = f.Relation.DisplayName
		}
		field["relation"] = relation
	}
	if f.File != nil {
		fileConfig := map[string]any{
			"bucket": f.File.Bucket,
		}
		if f.File.MaxSize > 0 {
			fileConfig["maxSize"] = f.File.MaxSize
		}
		if len(f.File.AllowedTypes) > 0 {
			fileConfig["allowedTypes"] = f.File.AllowedTypes
		}
		if f.File.OnDelete != "" {
			fileConfig["onDelete"] = string(f.File.OnDelete)
		}
		field["file"] = fileConfig
	}
	return field
}

func serializeRules(r *schema.Rules) map[string]string {
	if r == nil {
		return nil
	}
	rules := make(map[string]string)
	if r.Create != "" {
		rules["create"] = r.Create
	}
	if r.Read != "" {
		rules["read"] = r.Read
	}
	if r.Update != "" {
		rules["update"] = r.Update
	}
	if r.Delete != "" {
		rules["delete"] = r.Delete
	}
	return rules
}

func mustParseInt(s string) int64 {
	var n int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int64(c-'0')
		}
	}
	return n
}

// UserList handles GET /api/admin/users.
func (h *AdminHandlers) UserList(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	opts := auth.ListUsersOptions{
		Limit:   20,
		Offset:  0,
		SortBy:  r.URL.Query().Get("sort_by"),
		SortDir: r.URL.Query().Get("sort_dir"),
		Search:  r.URL.Query().Get("search"),
		Role:    r.URL.Query().Get("role"),
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit := int(mustParseInt(limitStr)); limit > 0 {
			opts.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset := int(mustParseInt(offsetStr)); offset >= 0 {
			opts.Offset = offset
		}
	}

	result, err := h.authService.ListUsers(r.Context(), opts)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list users")
		InternalError(w, "Failed to list users")
		return
	}

	JSON(w, http.StatusOK, result)
}

// UserGet handles GET /api/admin/users/{id}.
func (h *AdminHandlers) UserGet(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	id := r.PathValue("id")
	if id == "" {
		BadRequest(w, "User ID is required")
		return
	}

	user, err := h.authService.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			NotFound(w, "User not found")
			return
		}
		log.Error().Err(err).Str("user_id", id).Msg("Failed to get user")
		InternalError(w, "Failed to get user")
		return
	}

	JSON(w, http.StatusOK, user)
}

// UserCreate handles POST /api/admin/users.
func (h *AdminHandlers) UserCreate(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	var input auth.CreateUserInput
	if decodeErr := json.NewDecoder(r.Body).Decode(&input); decodeErr != nil {
		BadRequest(w, "Invalid JSON body")
		return
	}

	if input.Email == "" {
		BadRequest(w, "Email is required")
		return
	}

	if input.Password == "" {
		BadRequest(w, "Password is required")
		return
	}

	user, err := h.authService.CreateUserByAdmin(r.Context(), input)
	if err != nil {
		if errors.Is(err, auth.ErrUserAlreadyExists) {
			Error(w, http.StatusConflict, "USER_EXISTS", "User with this email already exists")
			return
		}
		if strings.Contains(err.Error(), "password validation") {
			BadRequest(w, err.Error())
			return
		}
		if strings.Contains(err.Error(), "invalid role") {
			BadRequest(w, err.Error())
			return
		}
		log.Error().Err(err).Msg("Failed to create user")
		InternalError(w, "Failed to create user")
		return
	}

	JSON(w, http.StatusCreated, user)
}

// UserUpdate handles PATCH /api/admin/users/{id}.
func (h *AdminHandlers) UserUpdate(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	id := r.PathValue("id")
	if id == "" {
		BadRequest(w, "User ID is required")
		return
	}

	var input auth.UpdateUserInput
	if decodeErr := json.NewDecoder(r.Body).Decode(&input); decodeErr != nil {
		BadRequest(w, "Invalid JSON body")
		return
	}

	user, err := h.authService.UpdateUser(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			NotFound(w, "User not found")
			return
		}
		if errors.Is(err, auth.ErrUserAlreadyExists) {
			Error(w, http.StatusConflict, "EMAIL_EXISTS", "Email already in use")
			return
		}
		if strings.Contains(err.Error(), "invalid role") {
			BadRequest(w, err.Error())
			return
		}
		log.Error().Err(err).Str("user_id", id).Msg("Failed to update user")
		InternalError(w, "Failed to update user")
		return
	}

	JSON(w, http.StatusOK, user)
}

// UserDelete handles DELETE /api/admin/users/{id}.
func (h *AdminHandlers) UserDelete(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	id := r.PathValue("id")
	if id == "" {
		BadRequest(w, "User ID is required")
		return
	}

	if err := h.authService.DeleteUser(r.Context(), id); err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			NotFound(w, "User not found")
			return
		}
		log.Error().Err(err).Str("user_id", id).Msg("Failed to delete user")
		InternalError(w, "Failed to delete user")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"deleted": true,
		"id":      id,
	})
}

// UserSetPassword handles POST /api/admin/users/{id}/password.
func (h *AdminHandlers) UserSetPassword(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	id := r.PathValue("id")
	if id == "" {
		BadRequest(w, "User ID is required")
		return
	}

	var input struct {
		Password string `json:"password"`
	}
	if decodeErr := json.NewDecoder(r.Body).Decode(&input); decodeErr != nil {
		BadRequest(w, "Invalid JSON body")
		return
	}

	if input.Password == "" {
		BadRequest(w, "Password is required")
		return
	}

	if err := h.authService.SetPassword(r.Context(), id, input.Password); err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			NotFound(w, "User not found")
			return
		}
		if strings.Contains(err.Error(), "password validation") {
			BadRequest(w, err.Error())
			return
		}
		log.Error().Err(err).Str("user_id", id).Msg("Failed to set password")
		InternalError(w, "Failed to set password")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"id":      id,
	})
}

func (h *AdminHandlers) isDevMode() bool {
	return h.cfg != nil && h.cfg.Dev.Enabled
}

func (h *AdminHandlers) SchemaRawGet(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	if h.schemaPath == "" {
		Error(w, http.StatusNotFound, "SCHEMA_NOT_FOUND", "Schema file path not configured")
		return
	}

	content, err := os.ReadFile(h.schemaPath)
	if err != nil {
		log.Error().Err(err).Str("path", h.schemaPath).Msg("Failed to read schema file")
		InternalError(w, "Failed to read schema file")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"content": string(content),
		"path":    h.schemaPath,
	})
}

func (h *AdminHandlers) SchemaRawUpdate(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	if !h.isDevMode() {
		Error(w, http.StatusForbidden, "DEV_MODE_REQUIRED", "Schema editing is only available in development mode")
		return
	}

	if h.schemaPath == "" {
		Error(w, http.StatusNotFound, "SCHEMA_NOT_FOUND", "Schema file path not configured")
		return
	}

	var input struct {
		Content string `json:"content"`
	}
	if decodeErr := json.NewDecoder(r.Body).Decode(&input); decodeErr != nil {
		BadRequest(w, "Invalid JSON body")
		return
	}

	if input.Content == "" {
		BadRequest(w, "Content is required")
		return
	}

	if _, parseErr := schema.Parse([]byte(input.Content)); parseErr != nil {
		Error(w, http.StatusBadRequest, "INVALID_SCHEMA", parseErr.Error())
		return
	}

	if err := os.WriteFile(h.schemaPath, []byte(input.Content), 0o600); err != nil {
		log.Error().Err(err).Str("path", h.schemaPath).Msg("Failed to write schema file")
		InternalError(w, "Failed to write schema file")
		return
	}

	log.Info().Str("path", h.schemaPath).Msg("Schema file updated via admin API")

	JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Schema updated successfully. Restart the server or wait for hot-reload to apply changes.",
	})
}

func (h *AdminHandlers) ConfigRawGet(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	if h.configPath == "" {
		Error(w, http.StatusNotFound, "CONFIG_NOT_FOUND", "Config file path not configured")
		return
	}

	content, err := os.ReadFile(h.configPath)
	if err != nil {
		log.Error().Err(err).Str("path", h.configPath).Msg("Failed to read config file")
		InternalError(w, "Failed to read config file")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"content": string(content),
		"path":    h.configPath,
	})
}

func (h *AdminHandlers) ConfigRawUpdate(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	if !h.isDevMode() {
		Error(w, http.StatusForbidden, "DEV_MODE_REQUIRED", "Config editing is only available in development mode")
		return
	}

	if h.configPath == "" {
		Error(w, http.StatusNotFound, "CONFIG_NOT_FOUND", "Config file path not configured")
		return
	}

	var input struct {
		Content string `json:"content"`
	}
	if decodeErr := json.NewDecoder(r.Body).Decode(&input); decodeErr != nil {
		BadRequest(w, "Invalid JSON body")
		return
	}

	if input.Content == "" {
		BadRequest(w, "Content is required")
		return
	}

	if err := os.WriteFile(h.configPath, []byte(input.Content), 0o600); err != nil {
		log.Error().Err(err).Str("path", h.configPath).Msg("Failed to write config file")
		InternalError(w, "Failed to write config file")
		return
	}

	log.Info().Str("path", h.configPath).Msg("Config file updated via admin API")

	JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Config updated successfully. Restart the server to apply changes.",
	})
}

// ValidateRuleRequest is the request body for CEL rule validation.
type ValidateRuleRequest struct {
	Expression string   `json:"expression"`
	Fields     []string `json:"fields,omitempty"`
}

// ValidateRuleResponse is the response for CEL rule validation.
type ValidateRuleResponse struct {
	Valid   bool     `json:"valid"`
	Error   string   `json:"error,omitempty"`
	Message string   `json:"message,omitempty"`
	Hints   []string `json:"hints,omitempty"`
}

// ValidateRule handles POST /api/admin/schema/validate-rule.
func (h *AdminHandlers) ValidateRule(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionDeploy)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	var req ValidateRuleRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		BadRequest(w, "Invalid JSON body")
		return
	}

	if req.Expression == "" {
		JSON(w, http.StatusOK, ValidateRuleResponse{
			Valid:   true,
			Message: "Empty expression (allows all)",
		})
		return
	}

	validationErr := validateCELExpression(req.Expression)
	if validationErr != nil {
		resp := ValidateRuleResponse{
			Valid: false,
			Error: validationErr.Error(),
		}

		errStr := validationErr.Error()
		if strings.Contains(errStr, "undeclared reference") {
			resp.Hints = append(resp.Hints, "Available variables: auth, doc, request")
			resp.Hints = append(resp.Hints, "auth fields: id, email, verified, role, metadata")
			resp.Hints = append(resp.Hints, "request fields: method, ip")
		}
		if strings.Contains(errStr, "found no matching overload") {
			resp.Hints = append(resp.Hints, "Check operator types match (e.g., comparing string to string)")
		}

		JSON(w, http.StatusOK, resp)
		return
	}

	JSON(w, http.StatusOK, ValidateRuleResponse{
		Valid:   true,
		Message: "Expression is valid",
	})
}

func validateCELExpression(expr string) error {
	env, err := cel.NewEnv(
		cel.Variable("auth", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("doc", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("request", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return fmt.Errorf("creating CEL environment: %w", err)
	}

	_, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return issues.Err()
	}

	return nil
}

func (h *AdminHandlers) SchemaPendingChanges(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	if h.pendingStore == nil {
		Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Pending changes store not initialized")
		return
	}

	changes, err := h.pendingStore.ListUnsafe()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list pending changes")
		InternalError(w, "Failed to list pending changes")
		return
	}

	hasPending, _ := h.pendingStore.HasPending()

	JSON(w, http.StatusOK, map[string]any{
		"pending": hasPending,
		"changes": changes,
		"total":   len(changes),
	})
}

func (h *AdminHandlers) SchemaConfirmChanges(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	if !h.isDevMode() {
		Error(w, http.StatusForbidden, "DEV_MODE_REQUIRED", "Schema changes can only be confirmed in development mode")
		return
	}

	if h.pendingStore == nil || h.migrator == nil {
		Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Migration services not initialized")
		return
	}

	pending, err := h.pendingStore.ListUnsafe()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list pending changes")
		InternalError(w, "Failed to list pending changes")
		return
	}

	if len(pending) == 0 {
		JSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "No pending changes to apply",
			"applied": 0,
		})
		return
	}

	changes := h.pendingStore.ToChanges(pending)

	validationErrors := h.migrator.ValidateUnsafeChanges(changes)
	if len(validationErrors) > 0 {
		errorDetails := make([]map[string]string, len(validationErrors))
		for i, ve := range validationErrors {
			errorDetails[i] = map[string]string{
				"path":    ve.Path,
				"message": ve.Message,
			}
		}
		Error(w, http.StatusConflict, "VALIDATION_FAILED", "Some changes cannot be applied")
		return
	}

	if err := h.migrator.ApplyUnsafeChanges(changes); err != nil {
		log.Error().Err(err).Msg("Failed to apply unsafe changes")
		Error(w, http.StatusInternalServerError, "MIGRATION_FAILED", err.Error())
		return
	}

	if h.schema != nil {
		gen := schema.NewSQLGenerator(h.schema)
		for _, col := range h.schema.Collections {
			for _, stmt := range gen.GenerateTriggers(col) {
				if _, err := h.db.Exec(stmt); err != nil {
					log.Warn().Err(err).Str("trigger", stmt[:50]).Msg("Failed to recreate trigger")
				}
			}
		}
	}

	if err := h.pendingStore.Clear(); err != nil {
		log.Error().Err(err).Msg("Failed to clear pending changes")
	}

	log.Info().Int("count", len(changes)).Msg("Applied unsafe schema changes")

	JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Schema changes applied successfully",
		"applied": len(changes),
	})
}

func (h *AdminHandlers) SchemaCancelChanges(w http.ResponseWriter, r *http.Request) {
	_, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	if h.pendingStore == nil {
		Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Pending changes store not initialized")
		return
	}

	if err := h.pendingStore.Clear(); err != nil {
		log.Error().Err(err).Msg("Failed to clear pending changes")
		InternalError(w, "Failed to clear pending changes")
		return
	}

	log.Info().Msg("Cancelled pending schema changes")

	JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Pending changes cancelled",
	})
}

func (h *AdminHandlers) SchemaDraftPreview(w http.ResponseWriter, r *http.Request) {
	token, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	if !h.isDevMode() {
		Error(w, http.StatusForbidden, "DEV_MODE_REQUIRED", "Schema editing is only available in development mode")
		return
	}

	var input struct {
		Content string `json:"content"`
	}
	if decodeErr := json.NewDecoder(r.Body).Decode(&input); decodeErr != nil {
		BadRequest(w, "Invalid JSON body")
		return
	}

	if input.Content == "" {
		BadRequest(w, "Content is required")
		return
	}

	newSchema, parseErr := schema.Parse([]byte(input.Content))
	if parseErr != nil {
		Error(w, http.StatusBadRequest, "INVALID_SCHEMA", parseErr.Error())
		return
	}

	sessionID := token.Name
	h.draftSchemas[sessionID] = input.Content

	currentSchema, err := schema.InferFromDB(h.db.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to infer current schema from database")
		InternalError(w, "Failed to load current schema")
		return
	}

	differ := schema.NewDiffer()
	diff := differ.Diff(currentSchema, newSchema)

	safeChanges := differ.SafeChanges(diff)
	unsafeChanges := differ.UnsafeChanges(diff)

	JSON(w, http.StatusOK, map[string]any{
		"sessionId":     sessionID,
		"valid":         true,
		"safeChanges":   safeChanges,
		"unsafeChanges": unsafeChanges,
		"totalChanges":  len(diff),
	})
}

func (h *AdminHandlers) SchemaDraftApply(w http.ResponseWriter, r *http.Request) {
	token, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	if !h.isDevMode() {
		Error(w, http.StatusForbidden, "DEV_MODE_REQUIRED", "Schema changes can only be applied in development mode")
		return
	}

	if h.schemaPath == "" {
		Error(w, http.StatusNotFound, "SCHEMA_NOT_FOUND", "Schema file path not configured")
		return
	}

	sessionID := token.Name
	draftContent, exists := h.draftSchemas[sessionID]
	if !exists {
		Error(w, http.StatusNotFound, "NO_DRAFT", "No draft schema found. Call PUT /api/admin/schema first.")
		return
	}

	newSchema, parseErr := schema.Parse([]byte(draftContent))
	if parseErr != nil {
		Error(w, http.StatusBadRequest, "INVALID_SCHEMA", parseErr.Error())
		return
	}

	currentSchema, err := schema.InferFromDB(h.db.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to infer current schema")
		InternalError(w, "Failed to load current schema")
		return
	}

	differ := schema.NewDiffer()
	diff := differ.Diff(currentSchema, newSchema)

	safeChanges := differ.SafeChanges(diff)
	unsafeChanges := differ.UnsafeChanges(diff)

	migrator := schema.NewMigrator(h.db.DB, h.schemaPath, "")

	if len(safeChanges) > 0 {
		if err := migrator.ApplySafeChanges(safeChanges); err != nil {
			log.Error().Err(err).Msg("Failed to apply safe changes")
			Error(w, http.StatusInternalServerError, "SAFE_MIGRATION_FAILED", err.Error())
			return
		}
	}

	if len(unsafeChanges) > 0 {
		validationErrors := migrator.ValidateUnsafeChanges(unsafeChanges)
		if len(validationErrors) > 0 {
			errorDetails := make([]map[string]string, len(validationErrors))
			for i, ve := range validationErrors {
				errorDetails[i] = map[string]string{
					"path":    ve.Path,
					"message": ve.Message,
				}
			}
			Error(w, http.StatusConflict, "VALIDATION_FAILED", "Some unsafe changes cannot be applied")
			return
		}

		if err := migrator.ApplyUnsafeChanges(unsafeChanges); err != nil {
			log.Error().Err(err).Msg("Failed to apply unsafe changes")
			Error(w, http.StatusInternalServerError, "UNSAFE_MIGRATION_FAILED", err.Error())
			return
		}

		gen := schema.NewSQLGenerator(newSchema)
		for _, col := range newSchema.Collections {
			for _, stmt := range gen.GenerateTriggers(col) {
				if _, err := h.db.Exec(stmt); err != nil {
					log.Warn().Err(err).Str("trigger", stmt[:50]).Msg("Failed to recreate trigger")
				}
			}
		}
	}

	if err := os.WriteFile(h.schemaPath, []byte(draftContent), 0o600); err != nil {
		log.Error().Err(err).Str("path", h.schemaPath).Msg("Failed to write schema file")
		InternalError(w, "Failed to write schema file")
		return
	}

	delete(h.draftSchemas, sessionID)

	log.Info().
		Str("path", h.schemaPath).
		Int("safe_changes", len(safeChanges)).
		Int("unsafe_changes", len(unsafeChanges)).
		Msg("Applied schema changes and wrote to file")

	JSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"message":       "Schema applied successfully",
		"safeApplied":   len(safeChanges),
		"unsafeApplied": len(unsafeChanges),
	})
}

func (h *AdminHandlers) SchemaDraftCancel(w http.ResponseWriter, r *http.Request) {
	token, err := h.requireAdminAuth(r, deploy.PermissionAdmin)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	sessionID := token.Name
	if _, exists := h.draftSchemas[sessionID]; exists {
		delete(h.draftSchemas, sessionID)
		log.Info().Str("session", sessionID).Msg("Cancelled draft schema")
	}

	JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Draft schema cancelled",
	})
}
