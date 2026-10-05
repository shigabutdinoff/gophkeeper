package account

import (
	"encoding/json"
	"fmt"

	"github.com/zalando/go-keyring"
)

const service, user = "gophkeeper", "session"

type record struct {
	Server  string `json:"server"`
	Email   string `json:"email"`
	Data    string `json:"data_key"`
	Refresh string `json:"refresh_token"`
	AppKey  string `json:"app_key"`
	Sign    []byte `json:"sign_key"`
}

func keep(r record) error {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("запись входа: %w", err)
	}
	if err = keyring.Set(service, user, string(data)); err != nil {
		return fmt.Errorf("хранилище секретов: %w", err)
	}
	return nil
}

func recall() (record, bool) {
	var r record
	data, err := keyring.Get(service, user)
	return r, err == nil && json.Unmarshal([]byte(data), &r) == nil
}
