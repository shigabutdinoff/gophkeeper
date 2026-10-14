package server

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"time"
)

func (a *auth) admit(w http.ResponseWriter, r *http.Request, acc account, login bool) {
	s := session{AccountID: acc.ID, RefreshToken: rand.Text()}
	if a.db.WithContext(r.Context()).Create(&s).Error != nil {
		respond(w, http.StatusInternalServerError, failed)
		return
	}
	a.grant(w, r, acc, s, login, json.RawMessage("null"))
}

func (a *auth) grant(w http.ResponseWriter, req *http.Request, acc account, s session, login bool, weak json.RawMessage) {
	now, created := time.Now().UTC(), acc.Created.UTC()
	token, err := a.sign("https://"+req.Host+"/auth/v1", acc.ID, s.ID, now.Add(lifetime))
	if err != nil {
		respond(w, http.StatusInternalServerError, failed)
		return
	}
	r := issued{AccessToken: token, TokenType: "bearer", ExpiresIn: int(lifetime.Seconds()),
		ExpiresAt: now.Add(lifetime).Unix(), RefreshToken: s.RefreshToken, User: user{
			ID: acc.ID, Aud: "authenticated", Role: "authenticated", Email: acc.Email, EmailConfirmedAt: created,
			LastSignInAt: now, AppMetadata: map[string]any{"provider": "email", "providers": []string{"email"}},
			UserMetadata: profile{acc.Email, true, false, acc.ID}, CreatedAt: created, UpdatedAt: now,
			Identities: []identity{{IdentityID: acc.ID, ID: acc.ID, UserID: acc.ID, IdentityData: profile{acc.Email, !login, false, acc.ID},
				Provider: "email", LastSignInAt: created, CreatedAt: created, UpdatedAt: created, Email: acc.Email}}}}
	if login {
		r.User.ConfirmedAt, r.WeakPassword = &created, weak
	}
	body, _ := json.Marshal(r)
	respond(w, http.StatusOK, string(body))
}
