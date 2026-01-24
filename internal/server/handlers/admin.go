package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/deploy"
)

// AdminHandlers handles admin API endpoints.
type AdminHandlers struct {
	deployService *deploy.Service
}

// NewAdminHandlers creates new admin handlers.
func NewAdminHandlers(deployService *deploy.Service) *AdminHandlers {
	return &AdminHandlers{
		deployService: deployService,
	}
}

// requireAdminToken validates admin token and required permission.
func (h *AdminHandlers) requireAdminToken(r *http.Request, perm deploy.TokenPermission) (*deploy.AdminToken, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errors.New("missing authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, errors.New("invalid authorization header format")
	}

	token, err := h.deployService.ValidateToken(parts[1])
	if err != nil {
		return nil, err
	}

	if !token.HasPermission(perm) {
		return nil, errors.New("insufficient permissions")
	}

	return token, nil
}

// DeployPrepare handles POST /api/admin/deploy/prepare.
func (h *AdminHandlers) DeployPrepare(w http.ResponseWriter, r *http.Request) {
	token, err := h.requireAdminToken(r, deploy.PermissionDeploy)
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
	token, err := h.requireAdminToken(r, deploy.PermissionDeploy)
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
	token, err := h.requireAdminToken(r, deploy.PermissionRollback)
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
	_, err := h.requireAdminToken(r, deploy.PermissionDeploy)
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
	creatorToken, err := h.requireAdminToken(r, deploy.PermissionAdmin)
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
	_, err := h.requireAdminToken(r, deploy.PermissionAdmin)
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
	_, err := h.requireAdminToken(r, deploy.PermissionAdmin)
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
	_, err := h.requireAdminToken(r, deploy.PermissionDeploy)
	if err != nil {
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	current, err := h.deployService.Store().GetCurrentDeployment()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get current deployment")
		Error(w, http.StatusInternalServerError, "SCHEMA_ERROR", err.Error())
		return
	}

	if current == nil {
		JSON(w, http.StatusOK, map[string]any{
			"version":        nil,
			"schema":         nil,
			"schema_hash":    "",
			"functions_hash": "",
		})
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"version":        current.Version,
		"schema":         current.SchemaSnapshot,
		"schema_hash":    current.SchemaHash,
		"functions_hash": current.FunctionsHash,
		"deployed_at":    current.DeployedAt,
		"deployed_by":    current.DeployedBy,
	})
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
