package auth

import (
	"context"

	"github.com/watzon/alyx/internal/config"
)

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token" //nolint:gosec // OAuth endpoint URL, not a credential
	googleUserURL  = "https://www.googleapis.com/oauth2/v2/userinfo"
)

type googleProvider struct {
	baseProvider
}

func newGoogleProvider(cfg config.OAuthProviderConfig) *googleProvider {
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "email", "profile"}
	}

	return &googleProvider{
		baseProvider: baseProvider{
			name:         "google",
			clientID:     cfg.ClientID,
			clientSecret: cfg.ClientSecret,
			scopes:       scopes,
			authURL:      googleAuthURL,
			tokenURL:     googleTokenURL,
			userInfoURL:  googleUserURL,
		},
	}
}

func (p *googleProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUserInfo, error) {
	userData, err := fetchUserInfo(ctx, googleUserURL, token.AccessToken)
	if err != nil {
		return nil, err
	}

	userInfo := &OAuthUserInfo{
		Provider: "google",
		RawData:  userData,
	}

	if id, ok := userData["id"].(string); ok {
		userInfo.ID = id
	}
	if email, ok := userData["email"].(string); ok {
		userInfo.Email = email
	}
	if verified, ok := userData["verified_email"].(bool); ok {
		userInfo.EmailVerified = verified
	}
	if name, ok := userData["name"].(string); ok {
		userInfo.Name = name
	}
	if picture, ok := userData["picture"].(string); ok {
		userInfo.AvatarURL = picture
	}

	if userInfo.Email == "" {
		return nil, ErrEmailRequired
	}

	return userInfo, nil
}
