package server

import (
	"crypto/ecdsa"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type auth struct {
	db       *gorm.DB
	key      *ecdsa.PrivateKey
	kid      string
	keys     []byte
	log      *logrus.Logger
	files    jetstream.ObjectStore
	validate *validator.Validate
	pauses   []time.Duration
}
