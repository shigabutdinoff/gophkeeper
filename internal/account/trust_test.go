package account

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pemFile(t *testing.T, kind string, der []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "server.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0o600), "файл PEM не записан")
	return path
}

func foreign(t *testing.T) (cert, key []byte) {
	t.Helper()
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "ключ не создан")
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), NotAfter: time.Now().Add(time.Hour)}
	cert, err = x509.CreateCertificate(rand.Reader, tmpl, tmpl, &private.PublicKey, private)
	require.NoError(t, err, "сертификат не выпущен")
	key, err = x509.MarshalPKCS8PrivateKey(private)
	require.NoError(t, err, "ключ не закодирован")
	return cert, key
}

func TestTrust(t *testing.T) {
	g := startGotrue(t, false)
	client := &http.Client{Timeout: 2 * time.Second}
	_, err := Login(t.Context(), client, g.settings, "anna@example.com", "пароль-Анны")
	require.ErrorIs(t, err, ErrUntrusted, "вход прошёл на сервере с недоверенным сертификатом")
	assert.EqualError(t, err, "сертификат сервера не доверенный")
	assert.Empty(t, g.log(t), "сервер получил запрос от клиента без доверия")

	require.NoError(t, Trust(client, pemFile(t, "CERTIFICATE", g.srv.Certificate().Raw)), "сертификат сервера не принят")
	want := x509.NewCertPool()
	want.AddCert(g.srv.Certificate())
	assert.True(t, client.Transport.(*http.Transport).TLSClientConfig.RootCAs.Equal(want), "клиент доверяет не только сертификату из файла")
	_, err = Login(t.Context(), client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход с файлом сертификата сервера не прошёл")

	other, _ := foreign(t)
	calls := g.log(t)
	client = &http.Client{Timeout: 2 * time.Second}
	require.NoError(t, Trust(client, pemFile(t, "CERTIFICATE", other)), "чужой сертификат не принят")
	_, err = Unlock(t.Context(), client, g.settings, noAsk)
	assert.ErrorIs(t, err, ErrUntrusted, "сервер с другим сертификатом принят или стал причиной работы без сервера")
	assert.Equal(t, calls, g.log(t), "сервер с другим сертификатом получил запрос")
	_, ok := recall()
	assert.True(t, ok, "вход забыт из-за недоверенного сертификата")
}

func TestTrustBadFile(t *testing.T) {
	g := startGotrue(t, false)
	_, key := foreign(t)
	text := filepath.Join(t.TempDir(), "server.pem")
	require.NoError(t, os.WriteFile(text, []byte("не PEM"), 0o600), "файл не записан")
	var got []string
	for _, path := range []string{filepath.Join(t.TempDir(), "server.pem"), text, pemFile(t, "PRIVATE KEY", key)} {
		err := Trust(g.client, path)
		require.Error(t, err, "негодный файл сертификата принят")
		got = append(got, strings.ReplaceAll(err.Error(), path, "<файл>"))
		_, err = Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
		assert.ErrorIs(t, err, ErrUntrusted, "после негодного файла клиент доверяет серверу")
	}
	snaps.MatchSnapshot(t, got)
	assert.Empty(t, g.log(t), "после негодного файла сервер получил запрос")
}

func TestTrustNothing(t *testing.T) {
	g := startGotrue(t, false)
	require.NoError(t, Trust(g.client, ""), "пустая настройка вернула ошибку")
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "пустая настройка изменила доверие клиента")
}
