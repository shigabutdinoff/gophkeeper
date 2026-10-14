package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func (a *auth) token(c *gin.Context) {
	if c.Query("grant_type") == "refresh_token" {
		a.refresh(c)
		return
	}
	var in credentials
	if c.ShouldBindJSON(&in) != nil || c.Query("grant_type") != "password" {
		c.Data(http.StatusBadRequest, gin.MIMEJSON, []byte(invalid))
		return
	}
	var acc account
	err := a.db.WithContext(c.Request.Context()).Take(&acc, "email = ?", strings.ToLower(in.Email)).Error
	switch {
	case err != nil:
		c.Data(http.StatusInternalServerError, gin.MIMEJSON, []byte(failed))
	case bcrypt.CompareHashAndPassword([]byte(acc.EncryptedPassword), []byte(in.Password)) != nil:
		c.Data(http.StatusBadRequest, gin.MIMEJSON, []byte(wrong))
	default:
		a.admit(c, acc, true)
	}
}
