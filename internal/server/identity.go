package server

import "time"

type identity struct {
	IdentityID   string    `json:"identity_id"`
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	IdentityData profile   `json:"identity_data"`
	Provider     string    `json:"provider"`
	LastSignInAt time.Time `json:"last_sign_in_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
}

type profile struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	PhoneVerified bool   `json:"phone_verified"`
	Sub           string `json:"sub"`
}
