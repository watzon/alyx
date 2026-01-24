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
	db    *database.DB
	jwt   *JWTService
	cfg   *config.AuthConfig
	oauth *OAuthManager
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

// OAuth returns the OAuth manager.
func (s *Service) OAuth() *OAuthManager {
	return s.oauth
}

// Register creates a new user account.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*User, *TokenPair, error) {
	if !s.cfg.AllowRegistration {
		return nil, nil, ErrRegistrationClosed
	}

	if err := ValidatePassword(input.Password, s.cfg.Password); err != nil {
		return nil, nil, fmt.Errorf("password validation: %w", err)
	}

	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	existing, err := s.getUserByEmail(ctx, input.Email)
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, nil, fmt.Errorf("checking existing user: %w", err)
	}
	if existing != nil {
		return nil, nil, ErrUserAlreadyExists
	}

	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, nil, fmt.Errorf("hashing password: %w", err)
	}

	user := &User{
		ID:        uuid.New().String(),
		Email:     input.Email,
		Verified:  !s.cfg.RequireVerification,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Metadata:  input.Metadata,
	}

	if createErr := s.createUser(ctx, user, passwordHash); createErr != nil {
		return nil, nil, fmt.Errorf("creating user: %w", createErr)
	}

	log.Info().Str("user_id", user.ID).Str("email", user.Email).Msg("User registered")

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

	refreshHash, err := HashPassword(refreshToken)
	if err != nil {
		return nil, nil, fmt.Errorf("hashing refresh token: %w", err)
	}

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
	refreshHash, err := HashPassword(refreshToken)
	if err != nil {
		return fmt.Errorf("hashing refresh token: %w", err)
	}

	session, err := s.getSessionByRefreshHash(ctx, refreshHash)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		return err
	}

	return s.deleteSession(ctx, session.ID)
}

// ValidateToken validates an access token and returns the claims.
func (s *Service) ValidateToken(token string) (*Claims, error) {
	return s.jwt.ValidateAccessToken(token)
}

// GetUserByID retrieves a user by ID.
func (s *Service) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `SELECT id, email, verified, created_at, updated_at, metadata FROM _alyx_users WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	user := &User{}
	var metadataJSON sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(&user.ID, &user.Email, &user.Verified, &createdAt, &updatedAt, &metadataJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning user: %w", err)
	}

	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	user.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return user, nil
}

func (s *Service) getUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `SELECT id, email, verified, created_at, updated_at, metadata FROM _alyx_users WHERE email = ?`
	row := s.db.QueryRowContext(ctx, query, email)

	user := &User{}
	var metadataJSON sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(&user.ID, &user.Email, &user.Verified, &createdAt, &updatedAt, &metadataJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning user: %w", err)
	}

	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	user.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return user, nil
}

func (s *Service) getUserWithPassword(ctx context.Context, email string) (*User, string, error) {
	query := `SELECT id, email, password_hash, verified, created_at, updated_at, metadata FROM _alyx_users WHERE email = ?`
	row := s.db.QueryRowContext(ctx, query, email)

	user := &User{}
	var passwordHash sql.NullString
	var metadataJSON sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(&user.ID, &user.Email, &passwordHash, &user.Verified, &createdAt, &updatedAt, &metadataJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrUserNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("scanning user: %w", err)
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

	refreshHash, err := HashPassword(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("hashing refresh token: %w", err)
	}

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
