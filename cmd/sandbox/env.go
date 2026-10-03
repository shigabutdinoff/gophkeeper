package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/caarlos0/env/v11"
)

const (
	supabaseURL = "SUPABASE_URL"
	supabaseKey = "SUPABASE_ANON_KEY"
	adminCreds  = "SYNADIA_ADMIN_CREDS"
	clientCreds = "SYNADIA_CLIENT_CREDS"
)

type cloudConfig struct {
	URL         string `env:"SUPABASE_URL,notEmpty"`
	Key         string `env:"SUPABASE_ANON_KEY,notEmpty"`
	AdminCreds  string `env:"SYNADIA_ADMIN_CREDS,notEmpty"`
	ClientCreds string `env:"SYNADIA_CLIENT_CREDS,notEmpty"`
}

func cloudEnv() (cloudConfig, error) {
	c, err := env.ParseAs[cloudConfig]()
	if err != nil {
		return cloudConfig{}, envError(err)
	}
	if err := httpsOnly(c.URL); err != nil {
		return cloudConfig{}, err
	}
	return c, nil
}

func envError(err error) error {
	agg, ok := errors.AsType[env.AggregateError](err)
	if !ok {
		return err
	}
	keys := make([]string, 0, len(agg.Errors))
	for _, e := range agg.Errors {
		if empty, ok := errors.AsType[env.EmptyVarError](e); ok {
			keys = append(keys, empty.Key)
		}
	}
	return fmt.Errorf("не заданы %s", strings.Join(keys, ", "))
}
