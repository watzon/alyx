package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/auth"
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
	startTime     time.Time
}

// NewAdminHandlers creates new admin handlers.
func NewAdminHandlers(deployService *deploy.Service, authService *auth.Service, db *database.DB, sch *schema.Schema, funcService *functions.Service) *AdminHandlers {
	return &AdminHandlers{
		deployService: deployService,
		authService:   authService,
		db:            db,
		schema:        sch,
		funcService:   funcService,
		startTime:     time.Now(),
	}
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

	JSON(w, http.StatusOK, map[string]any{
		"version":     h.schema.Version,
		"collections": collections,
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
