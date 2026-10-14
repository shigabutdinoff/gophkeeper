package account

import (
	"encoding/json"
	"fmt"

	"github.com/avast/retry-go/v5"
	"github.com/zalando/go-keyring"
)

const service, user = "gophkeeper", "session"

type record struct {
	Server  string `json:"server"`
	Email   string `json:"email"`
	Data    string `json:"data_key"`
	Refresh string `json:"refresh_token"`
	AppKey  string `json:"app_key"`
	Access  string `json:"access_token"`
	User    string `json:"user_id"`
	Cert    string `json:"server_cert"`
}

func keep(r record, set func(service, user, password string) error) error {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("запись входа: %w", err)
	}
	if err = retry.New(retry.Attempts(3), retry.Delay(retryWait()), retry.DelayType(retry.FixedDelay), retry.LastErrorOnly(true)).Do(func() error { return set(service, user, string(data)) }); err != nil {
		return fmt.Errorf("хранилище секретов: %w", err)
	}
	return nil
}

func recall() (record, bool) {
	var r record
	data, err := keyring.Get(service, user)
	return r, err == nil && json.Unmarshal([]byte(data), &r) == nil
}
