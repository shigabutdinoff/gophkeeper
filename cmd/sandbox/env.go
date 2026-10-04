package main

import "github.com/caarlos0/env/v11"

const (
	supabaseURL = "SUPABASE_URL"
	supabaseKey = "SUPABASE_ANON_KEY"
	adminCreds  = "SYNADIA_ADMIN_CREDS"
	clientCreds = "SYNADIA_CLIENT_CREDS"
)

type supabaseConfig struct {
	URL string `env:"SUPABASE_URL,notEmpty"`
	Key string `env:"SUPABASE_ANON_KEY,notEmpty"`
}

type cloudConfig struct {
	Supabase    supabaseConfig
	AdminCreds  string `env:"SYNADIA_ADMIN_CREDS,notEmpty"`
	ClientCreds string `env:"SYNADIA_CLIENT_CREDS,notEmpty"`
}

func (s supabaseConfig) check() error {
	return httpsOnly(s.URL)
}

func (c cloudConfig) check() error {
	return c.Supabase.check()
}

func parseEnv[T interface{ check() error }]() (T, error) {
	var zero T
	c, err := env.ParseAs[T]()
	if err != nil {
		return zero, envError(err)
	}
	if err = c.check(); err != nil {
		return zero, err
	}
	return c, nil
}
