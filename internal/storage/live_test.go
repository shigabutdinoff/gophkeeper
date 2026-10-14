package storage

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type liveUser struct {
	Token string `json:"access_token"`
	User  struct {
		ID string `json:"id"`
	} `json:"user"`
}

func signup(t *testing.T, web *http.Client, server, app, service, name string) liveUser {
	t.Helper()
	var u liveUser
	c := resty.NewWithClient(web).SetBaseURL(server)
	email := name + "+gop6-" + time.Now().Format("20060102150405.000000") + "@example.com"
	resp, err := c.R().SetHeader("apikey", app).SetResult(&u).
		SetBody(map[string]string{"email": email, "password": "live-" + email}).Post("auth/v1/signup")
	require.NoError(t, err, "сервер не принял регистрацию")
	require.NotEmpty(t, u.Token, "нет токена доступа, ответ %d", resp.StatusCode())
	t.Cleanup(func() {
		_, err := c.R().SetHeader("apikey", service).SetAuthToken(service).Delete("auth/v1/admin/users/" + u.User.ID)
		assert.NoError(t, err, "учётная запись %s не удалена", email)
	})
	return u
}

func TestLive(t *testing.T) {
	server, region := os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_REGION")
	app, service := os.Getenv("SUPABASE_ANON_KEY"), os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	if os.Getenv("GOPHKEEPER_LIVE") == "" || server == "" || region == "" || app == "" || service == "" {
		t.Skip("нет переменных облачного сервера")
	}
	web := &http.Client{Timeout: 30 * time.Second}
	anna, boris := signup(t, web, server, app, service, "anna"), signup(t, web, server, app, service, "boris")
	admin, data := New(web, server, region, app, service), []byte("шифр файла")
	require.NoError(t, admin.Put(t.Context(), anna.User.ID, record, data), "служебный ключ не записал объект")
	got, err := admin.Get(t.Context(), anna.User.ID, record, 100)
	require.NoError(t, err, "служебный ключ не прочитал объект")
	assert.Equal(t, data, got, "служебный ключ прочитал не то")
	got, err = New(web, server, region, app, anna.Token).Get(t.Context(), anna.User.ID, record, 100)
	require.NoError(t, err, "владелец не прочитал свой объект")
	assert.Equal(t, data, got, "владелец прочитал не то")
	stranger := New(web, server, region, app, boris.Token)
	_, err = stranger.Get(t.Context(), anna.User.ID, record, 100)
	assert.Error(t, err, "чужой объект прочитан")
	assert.Error(t, stranger.Put(t.Context(), boris.User.ID, record, data), "пользователь записал объект сам")
	assert.Error(t, stranger.Put(t.Context(), anna.User.ID, record, []byte("подмена")), "пользователь подменил чужой объект")
	_ = stranger.Delete(t.Context(), anna.User.ID, record)
	_, err = admin.Get(t.Context(), anna.User.ID, record, 100)
	assert.NoError(t, err, "пользователь удалил чужой объект")
	require.NoError(t, admin.Delete(t.Context(), anna.User.ID, record), "служебный ключ не удалил объект")
	assert.NoError(t, admin.Delete(t.Context(), anna.User.ID, record), "удаление отсутствующего объекта не успех")
	_, err = admin.Get(t.Context(), anna.User.ID, record, 100)
	assert.ErrorIs(t, err, ErrNoObject, "после удаления объект читается или ошибка не та")
}
