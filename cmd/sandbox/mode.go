package main

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	appKeyFile     = "app_key"
	serviceKeyFile = "service_key"
	tokenKeyFile   = "token_key.pem"
)

func ownMode(state string) bool {
	_, err := os.Stat(filepath.Join(state, serviceKeyFile))
	return err == nil
}

func backend(state string) (c serviceConfig, cert string, err error) {
	if !ownMode(state) {
		c, err = parseEnv[serviceConfig]()
		return c, "", err
	}
	service, err := os.ReadFile(filepath.Join(state, serviceKeyFile))
	app, aerr := os.ReadFile(filepath.Join(state, appKeyFile))
	c.Supabase.URL, c.Supabase.Key, c.Supabase.Region, c.ServiceKey = httpsURL("https://"+ownAddr()), string(app), "us-east-1", string(service)
	return c, filepath.Join(state, caFile), errors.Join(err, aerr)
}

func checkMode(state string, own bool) (err error) {
	_, cerr := os.Stat(filepath.Join(state, certFile))
	switch {
	case ownMode(state) != own && (ownMode(state) || cerr == nil):
		return errors.New("песочница создана в другом режиме, сначала выполните " + displayName + " reset")
	case !own:
		_, _, err = backend(state)
	}
	return err
}
