package server

import (
	"embed"
	"time"
)

//go:embed *.sql
var migrations embed.FS

type account struct {
	ID                string `gorm:"default:gen_random_uuid()"`
	Email             string
	EncryptedPassword string
	Created           time.Time `gorm:"column:created_at;default:now()"`
}

type session struct {
	ID           string `gorm:"default:gen_random_uuid()"`
	AccountID    string
	RefreshToken string
	Account      account
}

type stored struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Note     string `json:"note"`
	Secret   string `json:"secret"`
	Inserted bool   `json:"-" gorm:"->"`
}

func (stored) TableName() string { return "records" }
