package server

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (a *auth) admit(c *gin.Context, acc account, login bool) {
	s := session{AccountID: acc.ID, RefreshToken: rand.Text()}
	if a.db.WithContext(c.Request.Context()).Create(&s).Error != nil {
		c.Data(http.StatusInternalServerError, gin.MIMEJSON, []byte(failed))
		return
	}
	a.grant(c, acc, s, login, json.RawMessage("null"))
}

func (a *auth) grant(c *gin.Context, acc account, s session, login bool, weak json.RawMessage) {
	now, created := time.Now().UTC(), acc.Created.UTC()
	token, err := a.sign("https://"+c.Request.Host+"/auth/v1", acc.ID, s.ID, now.Add(lifetime))
	if err != nil {
		c.Data(http.StatusInternalServerError, gin.MIMEJSON, []byte(failed))
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
	c.Data(http.StatusOK, gin.MIMEJSON, body)
}
