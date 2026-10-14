package server

import (
	"encoding/json"
	"time"
)

type issued struct {
	AccessToken  string          `json:"access_token"`
	TokenType    string          `json:"token_type"`
	ExpiresIn    int             `json:"expires_in"`
	ExpiresAt    int64           `json:"expires_at"`
	RefreshToken string          `json:"refresh_token"`
	User         user            `json:"user"`
	WeakPassword json.RawMessage `json:"weak_password,omitempty" swaggertype:"object" extensions:"x-nullable"`
}

type user struct {
	ID               string         `json:"id"`
	Aud              string         `json:"aud"`
	Role             string         `json:"role"`
	Email            string         `json:"email"`
	EmailConfirmedAt time.Time      `json:"email_confirmed_at"`
	Phone            string         `json:"phone"`
	ConfirmedAt      *time.Time     `json:"confirmed_at,omitempty"`
	LastSignInAt     time.Time      `json:"last_sign_in_at"`
	AppMetadata      map[string]any `json:"app_metadata"`
	UserMetadata     profile        `json:"user_metadata"`
	Identities       []identity     `json:"identities"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	IsAnonymous      bool           `json:"is_anonymous"`
}
