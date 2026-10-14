package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm/clause"
)

func (a *auth) signup(c *gin.Context) {
	var in credentials
	if c.ShouldBindJSON(&in) != nil {
		c.Data(http.StatusBadRequest, gin.MIMEJSON, []byte(invalid))
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		c.Data(http.StatusBadRequest, gin.MIMEJSON, []byte(invalid))
		return
	}
	acc := account{Email: strings.ToLower(in.Email), EncryptedPassword: string(hash)}
	res := a.db.WithContext(c.Request.Context()).Clauses(clause.OnConflict{DoNothing: true}).Create(&acc)
	switch {
	case res.Error != nil:
		c.Data(http.StatusInternalServerError, gin.MIMEJSON, []byte(failed))
	case res.RowsAffected == 0:
		c.Data(http.StatusUnprocessableEntity, gin.MIMEJSON, []byte(exists))
	default:
		a.admit(c, acc, false)
	}
}
