package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExpired     = errors.New("session has expired")
	ErrRegistrationClosed = errors.New("registration is disabled")
	ErrEmailNotVerified   = errors.New("email not verified")
)

// Service provides authentication operations.
type Service struct {
	db          *database.DB
	jwt         *JWTService
	cfg         *config.AuthConfig
	oauth       *OAuthManager
	hookTrigger HookTrigger
}

// HookTrigger defines the interface for auth event hooks.
type HookTrigger interface {
	OnSignup(ctx context.Context, user *User, metadata map[string]any) error
	OnLogin(ctx context.Context, user *User, metadata map[string]any) error
	OnLogout(ctx context.Context, user *User, metadata map[string]any) error
	OnPasswordReset(ctx context.Context, user *User, metadata map[string]any) error
	OnEmailVerify(ctx context.Context, user *User, metadata map[string]any) error
}

// NewService creates a new auth service.
func NewService(db *database.DB, cfg *config.AuthConfig) *Service {
	return &Service{
		db:    db,
		jwt:   NewJWTService(cfg.JWT),
		cfg:   cfg,
		oauth: NewOAuthManager(cfg.OAuth),
	}
}

// SetHookTrigger sets the hook trigger for auth events.
func (s *Service) SetHookTrigger(trigger HookTrigger) {
	s.hookTrigger = trigger
}

// OAuth returns the OAuth manager.
func (s *Service) OAuth() *OAuthManager {
	return s.oauth
}

// Register creates a new user account.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*User, *TokenPair, error) {
	hasUsers, err := s.HasUsers(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("checking for existing users: %w", err)
	}

	if hasUsers && !s.cfg.AllowRegistration {
		return nil, nil, ErrRegistrationClosed
	}

	if validationErr := ValidatePassword(input.Password, s.cfg.Password); validationErr != nil {
		return nil, nil, fmt.Errorf("password validation: %w", validationErr)
	}

	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	existing, existingErr := s.getUserByEmail(ctx, input.Email)
	if existingErr != nil && !errors.Is(existingErr, ErrUserNotFound) {
		return nil, nil, fmt.Errorf("checking existing user: %w", existingErr)
	}
	if existing != nil {
		return nil, nil, ErrUserAlreadyExists
	}

	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, nil, fmt.Errorf("hashing password: %w", err)
	}

	// First user becomes admin, subsequent users get default role
	role := RoleUser
	if !hasUsers {
		role = RoleAdmin
	}

	user := &User{
		ID:        uuid.New().String(),
		Email:     input.Email,
		Role:      role,
		Verified:  !s.cfg.RequireVerification,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Metadata:  input.Metadata,
	}

	if createErr := s.createUser(ctx, user, passwordHash); createErr != nil {
		return nil, nil, fmt.Errorf("creating user: %w", createErr)
	}

	log.Info().Str("user_id", user.ID).Str("email", user.Email).Msg("User registered")

	if s.hookTrigger != nil {
		if err := s.hookTrigger.OnSignup(ctx, user, nil); err != nil {
			log.Error().Err(err).Str("user_id", user.ID).Msg("Signup hook failed")
		}
	}

	tokens, err := s.createSession(ctx, user, "", "")
	if err != nil {
		return nil, nil, fmt.Errorf("creating session: %w", err)
	}

	return user, tokens, nil
}

// Login authenticates a user and returns tokens.
func (s *Service) Login(ctx context.Context, input LoginInput, userAgent, ipAddress string) (*User, *TokenPair, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	user, passwordHash, err := s.getUserWithPassword(ctx, input.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, fmt.Errorf("getting user: %w", err)
	}

	if passwordHash == "" {
		return nil, nil, ErrInvalidCredentials
	}

	if verifyErr := VerifyPassword(input.Password, passwordHash); verifyErr != nil {
		return nil, nil, ErrInvalidCredentials
	}

	if s.cfg.RequireVerification && !user.Verified {
		return nil, nil, ErrEmailNotVerified
	}

	log.Info().Str("user_id", user.ID).Str("email", user.Email).Msg("User logged in")

	if s.hookTrigger != nil {
		metadata := map[string]any{
			"ip":         ipAddress,
			"user_agent": userAgent,
		}
		if err := s.hookTrigger.OnLogin(ctx, user, metadata); err != nil {
			log.Error().Err(err).Str("user_id", user.ID).Msg("Login hook failed")
		}
	}

	tokens, err := s.createSession(ctx, user, userAgent, ipAddress)
	if err != nil {
		return nil, nil, fmt.Errorf("creating session: %w", err)
	}

	return user, tokens, nil
}

// Refresh exchanges a refresh token for new tokens.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*User, *TokenPair, error) {
	userID, err := s.jwt.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, nil, fmt.Errorf("validating refresh token: %w", err)
	}

	refreshHash := HashToken(refreshToken)

	session, err := s.getSessionByRefreshHash(ctx, refreshHash)
	if err != nil {
		return nil, nil, err
	}

	if session.UserID != userID {
		return nil, nil, ErrInvalidToken
	}

	if time.Now().After(session.ExpiresAt) {
		_ = s.deleteSession(ctx, session.ID)
		return nil, nil, ErrSessionExpired
	}

	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("getting user: %w", err)
	}

	_ = s.deleteSession(ctx, session.ID)

	tokens, err := s.createSession(ctx, user, session.UserAgent, session.IPAddress)
	if err != nil {
		return nil, nil, fmt.Errorf("creating new session: %w", err)
	}

	return user, tokens, nil
}

// Logout invalidates a session.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	refreshHash := HashToken(refreshToken)

	session, err := s.getSessionByRefreshHash(ctx, refreshHash)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		return err
	}

	if s.hookTrigger != nil {
		user, getUserErr := s.GetUserByID(ctx, session.UserID)
		if getUserErr == nil {
			metadata := map[string]any{
				"ip":         session.IPAddress,
				"user_agent": session.UserAgent,
			}
			if hookErr := s.hookTrigger.OnLogout(ctx, user, metadata); hookErr != nil {
				log.Error().Err(hookErr).Str("user_id", user.ID).Msg("Logout hook failed")
			}
		}
	}

	return s.deleteSession(ctx, session.ID)
}

// ValidateToken validates an access token and returns the claims.
func (s *Service) ValidateToken(token string) (*Claims, error) {
	return s.jwt.ValidateAccessToken(token)
}

// HasUsers returns true if any users exist in the system.
func (s *Service) HasUsers(ctx context.Context) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM _alyx_users LIMIT 1)`
	var exists bool
	err := s.db.QueryRowContext(ctx, query).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking for users: %w", err)
	}
	return exists, nil
}

// GetUserByID retrieves a user by ID.
func (s *Service) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `SELECT id, email, verified, role, created_at, updated_at, metadata FROM _alyx_users WHERE id = ?`
	return s.scanUserRow(s.db.QueryRowContext(ctx, query, id))
}

func (s *Service) getUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `SELECT id, email, verified, role, created_at, updated_at, metadata FROM _alyx_users WHERE email = ?`
	return s.scanUserRow(s.db.QueryRowContext(ctx, query, email))
}

func (s *Service) scanUserRow(row *sql.Row) (*User, error) {
	user := &User{}
	var metadataJSON sql.NullString
	var role sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(&user.ID, &user.Email, &user.Verified, &role, &createdAt, &updatedAt, &metadataJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning user: %w", err)
	}

	user.Role = role.String
	if user.Role == "" {
		user.Role = RoleUser
	}
	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	user.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return user, nil
}

func (s *Service) getUserWithPassword(ctx context.Context, email string) (*User, string, error) {
	query := `SELECT id, email, password_hash, verified, role, created_at, updated_at, metadata FROM _alyx_users WHERE email = ?`
	row := s.db.QueryRowContext(ctx, query, email)

	user := &User{}
	var passwordHash sql.NullString
	var metadataJSON sql.NullString
	var role sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(&user.ID, &user.Email, &passwordHash, &user.Verified, &role, &createdAt, &updatedAt, &metadataJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrUserNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("scanning user: %w", err)
	}

	user.Role = role.String
	if user.Role == "" {
		user.Role = RoleUser
	}
	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	user.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return user, passwordHash.String, nil
}

func (s *Service) createUser(ctx context.Context, user *User, passwordHash string) error {
	query := `INSERT INTO _alyx_users (id, email, password_hash, verified, created_at, updated_at, metadata) VALUES (?, ?, ?, ?, ?, ?, ?)`

	var metadata any
	if user.Metadata != nil {
		metadata = user.Metadata
	}

	_, err := s.db.ExecContext(ctx, query,
		user.ID,
		user.Email,
		passwordHash,
		user.Verified,
		user.CreatedAt.Format(time.RFC3339),
		user.UpdatedAt.Format(time.RFC3339),
		metadata,
	)

	return err
}

func (s *Service) createSession(ctx context.Context, user *User, userAgent, ipAddress string) (*TokenPair, error) {
	accessToken, expiresAt, err := s.jwt.GenerateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, refreshExpiresAt, err := s.jwt.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	refreshHash := HashToken(refreshToken)

	session := &Session{
		ID:               uuid.New().String(),
		UserID:           user.ID,
		RefreshTokenHash: refreshHash,
		ExpiresAt:        refreshExpiresAt,
		CreatedAt:        time.Now().UTC(),
		UserAgent:        userAgent,
		IPAddress:        ipAddress,
	}

	query := `INSERT INTO _alyx_sessions (id, user_id, refresh_token_hash, expires_at, created_at, user_agent, ip_address) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err = s.db.ExecContext(ctx, query,
		session.ID,
		session.UserID,
		session.RefreshTokenHash,
		session.ExpiresAt.Format(time.RFC3339),
		session.CreatedAt.Format(time.RFC3339),
		session.UserAgent,
		session.IPAddress,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting session: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		TokenType:    "Bearer",
	}, nil
}

func (s *Service) getSessionByRefreshHash(ctx context.Context, refreshHash string) (*Session, error) {
	query := `SELECT id, user_id, refresh_token_hash, expires_at, created_at, user_agent, ip_address FROM _alyx_sessions WHERE refresh_token_hash = ?`
	row := s.db.QueryRowContext(ctx, query, refreshHash)

	session := &Session{}
	var expiresAt, createdAt string
	var userAgent, ipAddress sql.NullString

	err := row.Scan(&session.ID, &session.UserID, &session.RefreshTokenHash, &expiresAt, &createdAt, &userAgent, &ipAddress)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning session: %w", err)
	}

	session.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	session.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	session.UserAgent = userAgent.String
	session.IPAddress = ipAddress.String

	return session, nil
}

func (s *Service) deleteSession(ctx context.Context, id string) error {
	query := `DELETE FROM _alyx_sessions WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// OAuthLogin handles OAuth login/registration flow.
func (s *Service) OAuthLogin(ctx context.Context, userInfo *OAuthUserInfo, userAgent, ipAddress string) (*User, *TokenPair, error) {
	oauthAccount, err := s.getOAuthAccount(ctx, userInfo.Provider, userInfo.ID)
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, nil, fmt.Errorf("getting oauth account: %w", err)
	}

	if oauthAccount != nil {
		user, getUserErr := s.GetUserByID(ctx, oauthAccount.UserID)
		if getUserErr != nil {
			return nil, nil, fmt.Errorf("getting user: %w", getUserErr)
		}

		log.Info().Str("user_id", user.ID).Str("provider", userInfo.Provider).Msg("OAuth login")

		tokens, sessionErr := s.createSession(ctx, user, userAgent, ipAddress)
		if sessionErr != nil {
			return nil, nil, fmt.Errorf("creating session: %w", sessionErr)
		}

		return user, tokens, nil
	}

	existingUser, getUserErr := s.getUserByEmail(ctx, userInfo.Email)
	if getUserErr != nil && !errors.Is(getUserErr, ErrUserNotFound) {
		return nil, nil, fmt.Errorf("checking existing user: %w", getUserErr)
	}

	if existingUser != nil {
		if linkErr := s.linkOAuthAccount(ctx, existingUser.ID, userInfo); linkErr != nil {
			return nil, nil, fmt.Errorf("linking oauth account: %w", linkErr)
		}

		log.Info().Str("user_id", existingUser.ID).Str("provider", userInfo.Provider).Msg("OAuth account linked to existing user")

		tokens, sessionErr := s.createSession(ctx, existingUser, userAgent, ipAddress)
		if sessionErr != nil {
			return nil, nil, fmt.Errorf("creating session: %w", sessionErr)
		}

		return existingUser, tokens, nil
	}

	user := &User{
		ID:        uuid.New().String(),
		Email:     userInfo.Email,
		Verified:  userInfo.EmailVerified,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if createErr := s.createUser(ctx, user, ""); createErr != nil {
		return nil, nil, fmt.Errorf("creating user: %w", createErr)
	}

	if linkErr := s.linkOAuthAccount(ctx, user.ID, userInfo); linkErr != nil {
		return nil, nil, fmt.Errorf("linking oauth account: %w", linkErr)
	}

	log.Info().Str("user_id", user.ID).Str("email", user.Email).Str("provider", userInfo.Provider).Msg("User registered via OAuth")

	tokens, err := s.createSession(ctx, user, userAgent, ipAddress)
	if err != nil {
		return nil, nil, fmt.Errorf("creating session: %w", err)
	}

	return user, tokens, nil
}

func (s *Service) getOAuthAccount(ctx context.Context, provider, providerUserID string) (*OAuthAccount, error) {
	query := `SELECT id, user_id, provider, provider_user_id, created_at FROM _alyx_oauth_accounts WHERE provider = ? AND provider_user_id = ?`
	row := s.db.QueryRowContext(ctx, query, provider, providerUserID)

	account := &OAuthAccount{}
	var createdAt string

	err := row.Scan(&account.ID, &account.UserID, &account.Provider, &account.ProviderUserID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning oauth account: %w", err)
	}

	account.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	return account, nil
}

func (s *Service) linkOAuthAccount(ctx context.Context, userID string, userInfo *OAuthUserInfo) error {
	existing, err := s.getOAuthAccount(ctx, userInfo.Provider, userInfo.ID)
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return err
	}
	if existing != nil {
		if existing.UserID != userID {
			return ErrAccountAlreadyLinked
		}
		return nil
	}

	query := `INSERT INTO _alyx_oauth_accounts (id, user_id, provider, provider_user_id, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err = s.db.ExecContext(ctx, query,
		uuid.New().String(),
		userID,
		userInfo.Provider,
		userInfo.ID,
		time.Now().UTC().Format(time.RFC3339),
	)

	return err
}

const (
	defaultListLimit = 20
	maxListLimit     = 100
	defaultSortField = "created_at"
	defaultSortDir   = "desc"
	sortDirAsc       = "asc"
)

var allowedSortFields = map[string]bool{
	"id": true, "email": true, "verified": true, "role": true, "created_at": true, "updated_at": true,
}

// ListUsers returns a paginated list of users with optional filtering.
func (s *Service) ListUsers(ctx context.Context, opts ListUsersOptions) (*ListUsersResult, error) {
	opts = normalizeListOptions(opts)

	whereClause, args := buildUserWhereClause(opts)

	total, err := s.countUsers(ctx, whereClause, args)
	if err != nil {
		return nil, err
	}

	users, err := s.queryUsers(ctx, whereClause, args, opts)
	if err != nil {
		return nil, err
	}

	return &ListUsersResult{Users: users, Total: total}, nil
}

func normalizeListOptions(opts ListUsersOptions) ListUsersOptions {
	if opts.Limit <= 0 {
		opts.Limit = defaultListLimit
	}
	if opts.Limit > maxListLimit {
		opts.Limit = maxListLimit
	}
	if opts.SortBy == "" || !allowedSortFields[opts.SortBy] {
		opts.SortBy = defaultSortField
	}
	if opts.SortDir != sortDirAsc && opts.SortDir != defaultSortDir {
		opts.SortDir = defaultSortDir
	}
	return opts
}

func buildUserWhereClause(opts ListUsersOptions) (string, []any) {
	var conditions []string
	var args []any

	if opts.Search != "" {
		conditions = append(conditions, "email LIKE ?")
		args = append(args, "%"+opts.Search+"%")
	}
	if opts.Role != "" {
		conditions = append(conditions, "role = ?")
		args = append(args, opts.Role)
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func (s *Service) countUsers(ctx context.Context, whereClause string, args []any) (int, error) {
	var total int
	query := "SELECT COUNT(*) FROM _alyx_users" + whereClause
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}
	return total, nil
}

func (s *Service) queryUsers(ctx context.Context, whereClause string, args []any, opts ListUsersOptions) ([]*User, error) {
	query := fmt.Sprintf(
		"SELECT id, email, verified, role, created_at, updated_at, metadata FROM _alyx_users%s ORDER BY %s %s LIMIT ? OFFSET ?",
		whereClause, opts.SortBy, strings.ToUpper(opts.SortDir),
	)
	args = append(args, opts.Limit, opts.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying users: %w", err)
	}
	defer rows.Close()

	users := make([]*User, 0)
	for rows.Next() {
		user, scanErr := s.scanUserFromRows(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating users: %w", err)
	}

	return users, nil
}

func (s *Service) scanUserFromRows(rows *sql.Rows) (*User, error) {
	user := &User{}
	var metadataJSON sql.NullString
	var role sql.NullString
	var createdAt, updatedAt string

	if err := rows.Scan(&user.ID, &user.Email, &user.Verified, &role, &createdAt, &updatedAt, &metadataJSON); err != nil {
		return nil, fmt.Errorf("scanning user: %w", err)
	}

	user.Role = role.String
	if user.Role == "" {
		user.Role = RoleUser
	}
	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	user.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return user, nil
}

// UpdateUser updates a user's information by ID.
func (s *Service) UpdateUser(ctx context.Context, id string, input UpdateUserInput) (*User, error) {
	user, err := s.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var updates []string
	var args []any

	if input.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*input.Email))
		existing, existingErr := s.getUserByEmail(ctx, email)
		if existingErr != nil && !errors.Is(existingErr, ErrUserNotFound) {
			return nil, fmt.Errorf("checking existing email: %w", existingErr)
		}
		if existing != nil && existing.ID != id {
			return nil, ErrUserAlreadyExists
		}
		updates = append(updates, "email = ?")
		args = append(args, email)
	}

	if input.Verified != nil {
		updates = append(updates, "verified = ?")
		args = append(args, *input.Verified)
	}

	if input.Role != nil {
		role := strings.TrimSpace(*input.Role)
		if role != RoleUser && role != RoleAdmin {
			return nil, fmt.Errorf("invalid role: %s", role)
		}
		updates = append(updates, "role = ?")
		args = append(args, role)
	}

	if input.Metadata != nil {
		updates = append(updates, "metadata = ?")
		args = append(args, *input.Metadata)
	}

	if len(updates) == 0 {
		return user, nil
	}

	updates = append(updates, "updated_at = ?")
	args = append(args, time.Now().UTC().Format(time.RFC3339))
	args = append(args, id)

	query := fmt.Sprintf("UPDATE _alyx_users SET %s WHERE id = ?", strings.Join(updates, ", "))
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return nil, fmt.Errorf("updating user: %w", err)
	}

	return s.GetUserByID(ctx, id)
}

// DeleteUser deletes a user by ID.
func (s *Service) DeleteUser(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM _alyx_users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	log.Info().Str("user_id", id).Msg("User deleted")
	return nil
}

// CreateUserByAdmin creates a new user via admin API (bypasses registration settings).
func (s *Service) CreateUserByAdmin(ctx context.Context, input CreateUserInput) (*User, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	existing, existingErr := s.getUserByEmail(ctx, input.Email)
	if existingErr != nil && !errors.Is(existingErr, ErrUserNotFound) {
		return nil, fmt.Errorf("checking existing user: %w", existingErr)
	}
	if existing != nil {
		return nil, ErrUserAlreadyExists
	}

	if validationErr := ValidatePassword(input.Password, s.cfg.Password); validationErr != nil {
		return nil, fmt.Errorf("password validation: %w", validationErr)
	}

	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	role := input.Role
	if role == "" {
		role = RoleUser
	}
	if role != RoleUser && role != RoleAdmin {
		return nil, fmt.Errorf("invalid role: %s", role)
	}

	user := &User{
		ID:        uuid.New().String(),
		Email:     input.Email,
		Verified:  input.Verified,
		Role:      role,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Metadata:  input.Metadata,
	}

	if createErr := s.createUserWithRole(ctx, user, passwordHash); createErr != nil {
		return nil, fmt.Errorf("creating user: %w", createErr)
	}

	log.Info().Str("user_id", user.ID).Str("email", user.Email).Str("role", user.Role).Msg("User created by admin")

	return user, nil
}

func (s *Service) createUserWithRole(ctx context.Context, user *User, passwordHash string) error {
	query := `INSERT INTO _alyx_users (id, email, password_hash, verified, role, created_at, updated_at, metadata) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	var metadata any
	if user.Metadata != nil {
		metadata = user.Metadata
	}

	_, err := s.db.ExecContext(ctx, query,
		user.ID,
		user.Email,
		passwordHash,
		user.Verified,
		user.Role,
		user.CreatedAt.Format(time.RFC3339),
		user.UpdatedAt.Format(time.RFC3339),
		metadata,
	)

	return err
}

// SetPassword sets a new password for a user (admin operation).
func (s *Service) SetPassword(ctx context.Context, userID, newPassword string) error {
	if validationErr := ValidatePassword(newPassword, s.cfg.Password); validationErr != nil {
		return fmt.Errorf("password validation: %w", validationErr)
	}

	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	result, err := s.db.ExecContext(ctx,
		"UPDATE _alyx_users SET password_hash = ?, updated_at = ? WHERE id = ?",
		passwordHash, time.Now().UTC().Format(time.RFC3339), userID,
	)
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	log.Info().Str("user_id", userID).Msg("Password reset by admin")

	if s.hookTrigger != nil {
		user, getUserErr := s.GetUserByID(ctx, userID)
		if getUserErr == nil {
			if hookErr := s.hookTrigger.OnPasswordReset(ctx, user, nil); hookErr != nil {
				log.Error().Err(hookErr).Str("user_id", userID).Msg("Password reset hook failed")
			}
		}
	}

	return nil
}
