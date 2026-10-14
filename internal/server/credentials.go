package server

type credentials struct {
	Email    string `json:"email" binding:"email"`
	Password string `json:"password" binding:"required"`
}
