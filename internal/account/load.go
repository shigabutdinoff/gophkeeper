package account

import "github.com/shigabutdinoff/gophkeeper/internal/config"

func load() (session, error) {
	path, err := sessionPath()
	if err != nil {
		return session{}, err
	}
	return config.Load[session](path)
}

func loadEmail(server string) (string, error) {
	s, err := load()
	if err != nil {
		return "", err
	}
	if s.Email == "" || s.Server != server {
		return "", ErrLoggedOut
	}
	return s.Email, nil
}
