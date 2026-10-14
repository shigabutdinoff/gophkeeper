package server

import (
	"context"
	"crypto/rand"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestSignup(t *testing.T) {
	r := start(t)
	email, password := newEmail(), rand.Text()
	status, body := r.signup(t, email, password)
	assert.Equal(t, reference(t, "signup_ok.txt"), answer(status, masked(t, body)), "ответ регистрации не равен эталону Supabase")
	assert.NotEmpty(t, parse(t, body).Refresh, "нет токена продления")

	status, body = r.signup(t, email, password)
	assert.Equal(t, reference(t, "user_already_exists.txt"), answer(status, body), "отказ занятому адресу не равен эталону Supabase")
	status, body = r.signup(t, strings.ToUpper(email), password)
	assert.Equal(t, reference(t, "user_already_exists.txt"), answer(status, body), "адрес в другом регистре зарегистрирован повторно")
	assert.Equal(t, 1, r.rows(t, "WHERE email = $1", email), "повторная регистрация изменила число учётных записей")
}

func TestLogin(t *testing.T) {
	r := start(t)
	email, password := newEmail(), rand.Text()
	status, body := r.signup(t, strings.ToUpper(email), password)
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	id := parse(t, body).User.ID

	status, body = r.login(t, email, password)
	assert.Equal(t, reference(t, "token_ok.txt"), answer(status, masked(t, body)), "ответ входа не равен эталону Supabase")
	assert.Equal(t, id, parse(t, body).User.ID, "вход вернул другого пользователя")

	status, body = r.login(t, email, rand.Text())
	assert.Equal(t, reference(t, "invalid_credentials.txt"), answer(status, body), "отказ неверному паролю не равен эталону Supabase")
}

func TestRefusals(t *testing.T) {
	r := start(t)
	email, password := newEmail(), rand.Text()
	status, body := r.signup(t, email, password)
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	long := strings.Repeat("я", 37)
	big := `{"email":"` + newEmail() + `","password":"` + strings.Repeat("a", 1<<20) + `"}`
	for name, send := range map[string]func() (int, string){
		"адрес не по форме":          func() (int, string) { return r.signup(t, "anna.example.com", password) },
		"пустой ключ входа":          func() (int, string) { return r.signup(t, newEmail(), "") },
		"ключ входа длиннее 72 байт": func() (int, string) { return r.signup(t, newEmail(), long) },
		"негодный JSON":              func() (int, string) { return r.post(t, "/auth/v1/signup", "app-key", "не объект") },
		"тело больше предела": func() (int, string) {
			s, b, _ := r.send(t, http.MethodPost, "/auth/v1/signup", "app-key", []byte(big))
			return s, b
		},
		"вход с адресом не по форме": func() (int, string) { return r.login(t, "anna", password) },
		"продление без токена":       func() (int, string) { return r.refresh(t, "") },
		"вход без вида доступа": func() (int, string) {
			return r.post(t, "/auth/v1/token", "app-key", map[string]string{"email": email, "password": password})
		},
		"незарегистрированный адрес": func() (int, string) { return r.login(t, newEmail(), password) },
	} {
		got, refusal := send()
		assert.Contains(t, []int{http.StatusBadRequest, http.StatusInternalServerError}, got, "запрос не получил общий отказ: %s: %s", name, refusal)
		assert.Contains(t, []string{"validation_failed", "unexpected_failure"}, parse(t, refusal).Code, "клиент покажет чужую ошибку: %s", name)
		assert.Empty(t, parse(t, refusal).Access, "отказ выдал токен входа: %s", name)
	}
	assert.Equal(t, 1, r.rows(t, ""), "отказы создали учётные записи")
}

func TestStored(t *testing.T) {
	r := start(t)
	email, password := newEmail(), rand.Text()
	status, body := r.signup(t, email, password)
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	var hash string
	require.NoError(t, connect(t, r.dsn).QueryRow(context.Background(), "SELECT encrypted_password FROM accounts WHERE email = $1", email).Scan(&hash), "хеш не прочитан")
	cost, err := bcrypt.Cost([]byte(hash))
	require.NoError(t, err, "хеш не в формате bcrypt: %s", hash)
	assert.Equal(t, bcrypt.DefaultCost, cost, "стоимость хеша не та")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)), "хеш не подходит к ключу входа")
	assert.Zero(t, r.rows(t, "WHERE accounts::text LIKE '%' || $1 || '%'", password), "ключ входа лежит в таблице открыто")
}

func TestGuardedBeforeBody(t *testing.T) {
	r := start(t)
	email := newEmail()
	for name, body := range map[string]string{"обычное тело": `{"email":"` + email + `","password":"ключ"}`, "тело больше предела": strings.Repeat("a", 1<<20+1)} {
		status, got, _ := r.send(t, http.MethodPost, "/auth/v1/signup", "", []byte(body))
		assert.Equal(t, http.StatusUnauthorized, status, "запрос без ключа проекта принят: %s", name)
		assert.Contains(t, got, "No API key found in request", "отказ без ключа проекта изменился: %s", name)
	}
	assert.Zero(t, r.rows(t, ""), "запрос без ключа проекта создал учётную запись")
	for _, path := range []string{"/auth/v1/signup/", "/auth/v1/unknown", "/auth/v1/token/"} {
		status, _, _ := r.send(t, http.MethodPost, path, "app-key", []byte(`{}`))
		assert.Equal(t, http.StatusNotFound, status, "незнакомый адрес не получил 404: %s", path)
	}
}

func TestReplyShape(t *testing.T) {
	r := start(t)
	email, password := newEmail(), rand.Text()
	raw := []byte(`{"email":"` + email + `","password":"` + password + `"}`)
	for name, path := range map[string]string{"регистрация": "/auth/v1/signup", "отказ занятому адресу": "/auth/v1/signup", "вход": "/auth/v1/token?grant_type=password"} {
		_, body, header := r.send(t, http.MethodPost, path, "app-key", raw)
		assert.Equal(t, "application/json", header.Get("Content-Type"), "тип ответа не тот: %s", name)
		assert.False(t, strings.HasSuffix(body, "\n"), "ответ кончается переводом строки: %s", name)
	}
}

func TestConcurrentSignup(t *testing.T) {
	r := start(t)
	email, password := newEmail(), rand.Text()
	statuses := make([]int, 8)
	var wg sync.WaitGroup
	for i := range statuses {
		wg.Go(func() { statuses[i], _ = r.signup(t, email, password) })
	}
	wg.Wait()
	ok := 0
	for _, s := range statuses {
		if s == http.StatusOK {
			ok++
			continue
		}
		assert.Equal(t, http.StatusUnprocessableEntity, s, "одновременная регистрация получила не отказ занятому адресу")
	}
	assert.Equal(t, 1, ok, "успешных одновременных регистраций не одна")
	assert.Equal(t, 1, r.rows(t, "WHERE email = $1", email), "одновременная регистрация создала не одну запись")
}

func TestLostTable(t *testing.T) {
	r := start(t)
	email, password := newEmail(), rand.Text()
	status, body := r.signup(t, email, password)
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	_, err := connect(t, r.dsn).Exec(context.Background(), "DROP TABLE accounts")
	require.NoError(t, err, "таблица не удалена")
	for name, send := range map[string]func(*testing.T, string, string) (int, string){"вход": r.login, "регистрация": r.signup} {
		status, body = send(t, email, password)
		assert.Equal(t, http.StatusInternalServerError, status, "запрос без таблицы не получил общий отказ: %s", name)
		assert.Equal(t, "unexpected_failure", parse(t, body).Code, "сбой базы показан клиенту чужой ошибкой: %s", name)
	}
	got := r.get(t, keysPath, nil)
	assert.Equal(t, http.StatusOK, got.status, "сервер перестал отвечать после сбоя базы")
}

func TestQuiet(t *testing.T) {
	out, err := os.Create(filepath.Join(t.TempDir(), "out"))
	require.NoError(t, err, "файл вывода не создан")
	set(t, &os.Stdout, out)
	set(t, &os.Stderr, out)
	log.SetOutput(out)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	r := start(t)
	status, body := r.signup(t, newEmail(), rand.Text())
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	r.signup(t, "anna", "")
	r.get(t, keysPath, nil)
	r.stop()
	require.NoError(t, r.wait(t), "сервер остановился с ошибкой")
	got, err := os.ReadFile(out.Name())
	require.NoError(t, err, "вывод не прочитан")
	assert.Empty(t, string(got), "сервер пишет в stdout или stderr процесса")
}
