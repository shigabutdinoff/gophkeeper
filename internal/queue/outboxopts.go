package queue

import (
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/nats-io/nats-server/v2/server"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

func deviceOptions(s config.Settings, dir, data, access string) (*server.Options, error) {
	key, err := storeKey(data)
	if err != nil {
		return nil, err
	}
	opts := &server.Options{DontListen: true, NoLog: true, NoSigs: true, JetStream: true, JetStreamDomain: "device",
		StoreDir: dir, JetStreamKey: key, JetStreamCipher: server.ChaCha}
	if access == "" {
		return opts, nil
	}
	leaf, err := url.Parse(s.QueueLeaf)
	if err != nil {
		return nil, fmt.Errorf("адрес queue_leaf: %w", err)
	}
	leaf.User = url.User(access)
	t, err := secure(s.QueueCA)
	if err != nil {
		return nil, err
	}
	opts.LeafNode.Remotes = []*server.RemoteLeafOpts{{URLs: []*url.URL{leaf}, Credentials: s.QueueCreds, TLSConfig: t}}
	return opts, nil
}

func storeKey(data string) (string, error) {
	master, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("ключ к данным: %w", err)
	}
	key, err := hkdf.Key(sha256.New, master, nil, "gophkeeper outbox", 32)
	return base64.RawStdEncoding.EncodeToString(key), err
}
