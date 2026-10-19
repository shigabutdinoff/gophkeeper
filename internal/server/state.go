package server

import (
	"crypto/ecdsa"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type auth struct {
	db       *gorm.DB
	key      *ecdsa.PrivateKey
	kid      string
	keys     []byte
	log      *zap.Logger
	files    jetstream.ObjectStore
	validate *validator.Validate
	pauses   []time.Duration
}
