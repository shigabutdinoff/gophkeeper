package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/golang-jwt/jwt/v5/request"
	"gorm.io/gorm"
)

func (a *auth) refresh(c *gin.Context) {
	var in struct {
		Token string `json:"refresh_token" binding:"required"`
	}
	if c.ShouldBindJSON(&in) != nil {
		c.Data(http.StatusBadRequest, gin.MIMEJSON, []byte(invalid))
		return
	}
	var s session
	err := a.db.WithContext(c.Request.Context()).InnerJoins("Account").Take(&s, "refresh_token = ?", in.Token).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.Data(http.StatusBadRequest, gin.MIMEJSON, []byte(gone))
	case err != nil:
		c.Data(http.StatusInternalServerError, gin.MIMEJSON, []byte(failed))
	default:
		a.grant(c, s.Account, s, true, nil)
	}
}

func (a *auth) logout(c *gin.Context) {
	var claims jwt.RegisteredClaims
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"ES256"}), jwt.WithExpirationRequired())
	_, err := request.ParseFromRequest(c.Request, request.AuthorizationHeaderExtractor,
		func(*jwt.Token) (any, error) { return &a.key.PublicKey, nil }, request.WithClaims(&claims), request.WithParser(parser))
	switch {
	case err != nil || c.Query("scope") != "local":
		c.Data(http.StatusBadRequest, gin.MIMEJSON, []byte(invalid))
	case a.db.WithContext(c.Request.Context()).Delete(&session{}, "id = ?", claims.ID).Error != nil:
		c.Data(http.StatusInternalServerError, gin.MIMEJSON, []byte(failed))
	default:
		c.Status(http.StatusNoContent)
	}
}
