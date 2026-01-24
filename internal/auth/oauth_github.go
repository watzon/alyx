package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/watzon/alyx/internal/config"
)

const (
	githubAuthURL       = "https://github.com/login/oauth/authorize"
	githubTokenURL      = "https://github.com/login/oauth/access_token" //nolint:gosec // OAuth endpoint URL, not a credential
	githubUserURL       = "https://api.github.com/user"
	githubUserEmailsURL = "https://api.github.com/user/emails"
)

type githubProvider struct {
	baseProvider
}

func newGitHubProvider(cfg config.OAuthProviderConfig) *githubProvider {
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"user:email"}
	}

	return &githubProvider{
		baseProvider: baseProvider{
			name:         "github",
			clientID:     cfg.ClientID,
			clientSecret: cfg.ClientSecret,
			scopes:       scopes,
			authURL:      githubAuthURL,
			tokenURL:     githubTokenURL,
			userInfoURL:  githubUserURL,
		},
	}
}

func (p *githubProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUserInfo, error) {
	userData, err := fetchUserInfo(ctx, githubUserURL, token.AccessToken)
	if err != nil {
		return nil, err
	}

	email, emailVerified, err := p.fetchPrimaryEmail(ctx, token.AccessToken)
	if err != nil {
		if emailFromProfile, ok := userData["email"].(string); ok && emailFromProfile != "" {
			email = emailFromProfile
			emailVerified = true
		} else {
			return nil, fmt.Errorf("%w: could not get email from GitHub", ErrEmailRequired)
		}
	}

	userInfo := &OAuthUserInfo{
		Provider:      "github",
		Email:         email,
		EmailVerified: emailVerified,
		RawData:       userData,
	}

	if id, ok := userData["id"].(float64); ok {
		userInfo.ID = strconv.FormatInt(int64(id), 10)
	}
	if name, ok := userData["name"].(string); ok {
		userInfo.Name = name
	} else if login, ok := userData["login"].(string); ok {
		userInfo.Name = login
	}
	if avatar, ok := userData["avatar_url"].(string); ok {
		userInfo.AvatarURL = avatar
	}

	return userInfo, nil
}

func (p *githubProvider) fetchPrimaryEmail(ctx context.Context, accessToken string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserEmailsURL, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", false, fmt.Errorf("github emails API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", false, err
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, true, nil
		}
	}

	for _, e := range emails {
		if e.Primary {
			return e.Email, e.Verified, nil
		}
	}

	if len(emails) > 0 {
		return emails[0].Email, emails[0].Verified, nil
	}

	return "", false, fmt.Errorf("no email found")
}
