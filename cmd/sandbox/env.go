package main

import "github.com/caarlos0/env/v11"

const (
	supabaseURL = "SUPABASE_URL"
	supabaseKey = "SUPABASE_ANON_KEY"
	adminCreds  = "SYNADIA_ADMIN_CREDS"
	clientCreds = "SYNADIA_CLIENT_CREDS"
)

type httpsURL string

func (u *httpsURL) UnmarshalText(text []byte) error {
	*u = httpsURL(text)
	return httpsOnly(string(text))
}

type supabaseConfig struct {
	URL httpsURL `env:"SUPABASE_URL,notEmpty"`
	Key string   `env:"SUPABASE_ANON_KEY,notEmpty"`
}

type cloudConfig struct {
	Supabase    supabaseConfig
	AdminCreds  string `env:"SYNADIA_ADMIN_CREDS,notEmpty"`
	ClientCreds string `env:"SYNADIA_CLIENT_CREDS,notEmpty"`
}

type serviceConfig struct {
	Supabase   supabaseConfig
	ServiceKey string `env:"SUPABASE_SERVICE_ROLE_KEY,notEmpty"`
}

func parseEnv[T any]() (T, error) {
	c, err := env.ParseAs[T]()
	return c, envError(err)
}
