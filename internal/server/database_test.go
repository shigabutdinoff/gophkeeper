package server

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

const schemaSQL = `CREATE TABLE accounts (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	email text NOT NULL UNIQUE,
	encrypted_password text NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now())`

func tokenKey(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "ключ подписи токенов не создан")
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err, "ключ подписи токенов не закодирован")
	return keyFile(t, "PRIVATE KEY", der)
}

func keyFile(t *testing.T, kind string, der []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0o600), "ключ подписи токенов не записан")
	return path
}

func connect(t *testing.T, dsn string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), dsn)
	require.NoError(t, err, "база не открыта")
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}

func database(t *testing.T) string {
	t.Helper()
	base := os.Getenv("GOPHKEEPER_TEST_DATABASE")
	if base == "" {
		t.Skip("не задана GOPHKEEPER_TEST_DATABASE")
	}
	schema := "t_" + strings.ToLower(rand.Text())
	conn := connect(t, base)
	_, err := conn.Exec(context.Background(), "CREATE SCHEMA "+schema)
	require.NoError(t, err, "схема не создана")
	t.Cleanup(func() { conn.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	u, err := url.Parse(base)
	require.NoError(t, err, "адрес базы не разобран")
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}

func newEmail() string {
	return "anna-" + strings.ToLower(rand.Text()) + "@example.com"
}

func (r *running) send(t *testing.T, method, path, apikey string, body []byte) (int, string, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, "https://"+r.addr+path, bytes.NewReader(body))
	require.NoError(t, err, "запрос не собран")
	if apikey != "" {
		req.Header.Set("apikey", apikey)
	}
	resp, err := r.client().Do(req)
	require.NoError(t, err, "сервер не ответил")
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "ответ не прочитан")
	return resp.StatusCode, string(got), resp.Header
}

func (r *running) post(t *testing.T, path, apikey string, body any) (int, string) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err, "тело запроса не собрано")
	status, got, _ := r.send(t, http.MethodPost, path, apikey, raw)
	return status, got
}

func (r *running) signup(t *testing.T, email, password string) (int, string) {
	t.Helper()
	return r.post(t, "/auth/v1/signup", "app-key", map[string]string{"email": email, "password": password})
}

func (r *running) login(t *testing.T, email, password string) (int, string) {
	t.Helper()
	return r.post(t, "/auth/v1/token?grant_type=password", "app-key", map[string]string{"email": email, "password": password})
}

func (r *running) rows(t *testing.T, where string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, connect(t, r.dsn).QueryRow(context.Background(), "SELECT count(*) FROM accounts "+where, args...).Scan(&n), "таблица не прочитана")
	return n
}

type granted struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
	User    struct {
		ID string `json:"id"`
	} `json:"user"`
	Code string `json:"error_code"`
}

func parse(t *testing.T, body string) granted {
	t.Helper()
	var s granted
	require.NoError(t, json.Unmarshal([]byte(body), &s), "ответ не JSON: %s", body)
	return s
}
