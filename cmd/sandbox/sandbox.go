package main

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func sandboxInit(cmd *cobra.Command, _ []string) error {
	p, err := prepareSandbox("/sandbox", "/config", "/src")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), p)
	return err
}

func prepareSandbox(state, mount, src string) (placement, error) {
	client, err := sandboxSecrets(state)
	if err != nil {
		return placement{}, fmt.Errorf("секреты песочницы: %w", err)
	}
	want, err := sandboxSettings(state, client)
	if err != nil {
		return placement{}, fmt.Errorf("настройки песочницы: %w", err)
	}
	p, err := placeSettings(want, "sandbox", filepath.Join(mount, "gophkeeper", settingsFile))
	if err == nil {
		err = chownLike([]string{mount, src}, filepath.Dir(p.client), cmp.Or(p.aside, p.client))
	}
	host := cmp.Or(os.Getenv("CONFIG_DIR"), mount)
	p.client, p.aside = strings.Replace(p.client, mount, host, 1), strings.Replace(p.aside, mount, host, 1)
	return p, err
}
