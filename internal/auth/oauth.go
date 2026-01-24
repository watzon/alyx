package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/watzon/alyx/internal/config"
)

const (
	ProviderGitHub = "github"
	ProviderGoogle = "google"
)

var (
	ErrProviderNotFound      = errors.New("oauth provider not found")
	ErrProviderNotEnabled    = errors.New("oauth provider not enabled")
	ErrInvalidState          = errors.New("invalid oauth state")
	ErrStateExpired          = errors.New("oauth state expired")
	ErrTokenExchange         = errors.New("failed to exchange token")
	ErrUserInfoFetch         = errors.New("failed to fetch user info")
	ErrOAuthEmailNotVerified = errors.New("email not verified by provider")
	ErrEmailRequired         = errors.New("email is required from oauth provider")
	ErrAccountAlreadyLinked  = errors.New("oauth account already linked to another user")
)

type OAuthUserInfo struct {
	ID            string
	Email         string
	EmailVerified bool
	Name          string
	AvatarURL     string
	Provider      string
	RawData       map[string]any
}

type OAuthProvider interface {
	Name() string
	AuthURL(state, redirectURI string) string
	ExchangeCode(ctx context.Context, code, redirectURI string) (*OAuthToken, error)
	GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUserInfo, error)
}

type OAuthToken struct {
	AccessToken  string
	TokenType    string
	RefreshToken string
	ExpiresAt    time.Time
	Scope        string
}

type OAuthManager struct {
	providers map[string]OAuthProvider
	states    *stateStore
	mu        sync.RWMutex
}

func NewOAuthManager(cfg map[string]config.OAuthProviderConfig) *OAuthManager {
	m := &OAuthManager{
		providers: make(map[string]OAuthProvider),
		states:    newStateStore(),
	}

	for name, providerCfg := range cfg {
		if providerCfg.ClientID == "" || providerCfg.ClientSecret == "" {
			continue
		}

		var provider OAuthProvider
		switch strings.ToLower(name) {
		case ProviderGitHub:
			provider = newGitHubProvider(providerCfg)
		case ProviderGoogle:
			provider = newGoogleProvider(providerCfg)
		default:
			if providerCfg.AuthURL != "" && providerCfg.TokenURL != "" {
				provider = newGenericOIDCProvider(name, providerCfg)
			}
		}

		if provider != nil {
			m.providers[strings.ToLower(name)] = provider
		}
	}

	return m
}

func (m *OAuthManager) GetProvider(name string) (OAuthProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, ok := m.providers[strings.ToLower(name)]
	if !ok {
		return nil, ErrProviderNotFound
	}
	return provider, nil
}

func (m *OAuthManager) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}

func (m *OAuthManager) GenerateState() (string, error) {
	return m.states.generate()
}

func (m *OAuthManager) ValidateState(state string) error {
	return m.states.validate(state)
}

type stateStore struct {
	states map[string]time.Time
	mu     sync.Mutex
	ttl    time.Duration
}

func newStateStore() *stateStore {
	s := &stateStore{
		states: make(map[string]time.Time),
		ttl:    10 * time.Minute,
	}
	go s.cleanup()
	return s
}

func (s *stateStore) generate() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := base64.URLEncoding.EncodeToString(b)

	s.mu.Lock()
	s.states[state] = time.Now().Add(s.ttl)
	s.mu.Unlock()

	return state, nil
}

func (s *stateStore) validate(state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	expiry, ok := s.states[state]
	if !ok {
		return ErrInvalidState
	}

	delete(s.states, state)

	if time.Now().After(expiry) {
		return ErrStateExpired
	}

	return nil
}

func (s *stateStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for state, expiry := range s.states {
			if now.After(expiry) {
				delete(s.states, state)
			}
		}
		s.mu.Unlock()
	}
}

type baseProvider struct {
	name         string
	clientID     string
	clientSecret string
	scopes       []string
	authURL      string
	tokenURL     string
	userInfoURL  string
}

func (p *baseProvider) Name() string {
	return p.name
}

func (p *baseProvider) AuthURL(state, redirectURI string) string {
	params := url.Values{}
	params.Set("client_id", p.clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("state", state)
	if len(p.scopes) > 0 {
		params.Set("scope", strings.Join(p.scopes, " "))
	}

	return p.authURL + "?" + params.Encode()
}

func (p *baseProvider) ExchangeCode(ctx context.Context, code, redirectURI string) (*OAuthToken, error) {
	data := url.Values{}
	data.Set("client_id", p.clientID)
	data.Set("client_secret", p.clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTokenExchange, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTokenExchange, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: status %d, body: %s", ErrTokenExchange, resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTokenExchange, err)
	}

	token := &OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		Scope:        tokenResp.Scope,
	}

	if tokenResp.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	return token, nil
}

func fetchUserInfo(ctx context.Context, url, accessToken string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUserInfoFetch, err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUserInfoFetch, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: status %d, body: %s", ErrUserInfoFetch, resp.StatusCode, string(body))
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUserInfoFetch, err)
	}

	return data, nil
}
