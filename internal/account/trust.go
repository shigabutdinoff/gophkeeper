package account

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	"github.com/go-resty/resty/v2"
)

// Trust оставляет клиенту client доверие только сертификату сервера из
// файла path вместо системных центров. Пустой path ничего не меняет.
// После ошибки клиент не доверяет ни одному серверу.
func Trust(client *http.Client, path string) error {
	if path == "" {
		return nil
	}
	roots := x509.NewCertPool()
	resty.NewWithClient(client).SetLogger(quiet{}).SetTLSClientConfig(&tls.Config{RootCAs: roots}).SetRootCertificate(path)
	if roots.Equal(x509.NewCertPool()) {
		return fmt.Errorf("сертификат сервера не прочитан из файла %s в server_cert", path)
	}
	return nil
}
