package sandboxtest

import (
	"path/filepath"
	"testing"

	"github.com/madflojo/testcerts"
	"github.com/stretchr/testify/require"
)

// Project называет проект Compose, под которым работает песочница.
const Project = "gophkeeper-sandbox"

// Label отбирает в Docker объекты песочницы по метке проекта Compose.
const Label = "label=com.docker.compose.project=" + Project

// HomeEnv возвращает переменные окружения, которые переносят домашний
// каталог и каталог настроек пользователя внутрь home.
func HomeEnv(home string) []string {
	return []string{"HOME=" + home, "USERPROFILE=" + home, "XDG_CONFIG_HOME=" + ConfigDir(home)}
}

// ConfigDir возвращает каталог настроек, который получается при окружении
// HomeEnv.
func ConfigDir(home string) string {
	return filepath.Join(home, ".config")
}

// Cert выпускает тестовый сертификат для 127.0.0.1 и возвращает пути
// к его файлам во временном каталоге.
func Cert(t testing.TB) (cert, key string) {
	pair, err := testcerts.NewCA().NewKeyPair()
	require.NoError(t, err, "тестовый сертификат не выпущен")
	certFile, keyFile, err := pair.ToTempFile(t.TempDir())
	require.NoError(t, err, "файлы тестового сертификата не записаны")
	return certFile.Name(), keyFile.Name()
}
