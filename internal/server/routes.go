package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *auth) routes(s Settings) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	e, mux := gin.New(), http.NewServeMux()
	e.POST("/auth/v1/signup", a.signup)
	e.POST("/auth/v1/token", a.token)
	e.POST("/auth/v1/logout", a.logout)
	e.Match([]string{http.MethodGet, http.MethodHead}, "/auth/v1/.well-known/jwks.json", func(c *gin.Context) {
		c.Data(http.StatusOK, gin.MIMEJSON, a.keys)
	})
	mux.Handle("GET /auth/v1/.well-known/jwks.json", e)
	mux.Handle("POST /auth/v1/signup", guard(s, e))
	mux.Handle("POST /auth/v1/token", guard(s, e))
	mux.Handle("POST /auth/v1/logout", guard(s, e))
	mux.Handle("/", guard(s, operations))
	return http.MaxBytesHandler(mux, 1<<20)
}
