package account

import (
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthKey(t *testing.T) {
	key := authKey(t, "anna@example.com", "пароль-Анны")
	snaps.MatchSnapshot(t, key)
	assert.Equal(t, key, authKey(t, " Anna@Example.COM ", "пароль-Анны"), "ключ зависит от регистра и пробелов email")
	assert.NotEqual(t, key, authKey(t, "anna@example.com", "пароль-Анны!"), "ключ не зависит от пароля")
	assert.NotEqual(t, key, authKey(t, "boris@example.com", "пароль-Анны"), "ключ не зависит от email")
}

func authKey(t *testing.T, email, password string) string {
	t.Helper()
	key, err := AuthKey(email, password)
	require.NoError(t, err, "ключ входа не выведен")
	return key
}
