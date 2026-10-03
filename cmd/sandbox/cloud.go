package main

import (
	"context"
	"fmt"
	"os"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

const synadiaURL = "tls://connect.ngs.global"

type queueAccess struct {
	URL           string
	Admin, Client nats.Option
}

func cloud(cmd *cobra.Command, _ []string) error {
	c, err := cloudEnv()
	if err != nil {
		return err
	}
	p, err := paths("cloud")
	if err != nil {
		return err
	}
	q := queueAccess{synadiaURL, nats.UserCredentials(c.AdminCreds), nats.UserCredentials(c.ClientCreds)}
	msg, err := connectCloud(cmd.Context(), c, p, q)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "облако подготовлено,", msg)
	return err
}

func connectCloud(ctx context.Context, c cloudConfig, p sandboxPaths, q queueAccess) (placement, error) {
	creds, err := os.ReadFile(c.ClientCreds)
	if err != nil {
		return placement{}, fmt.Errorf("учётные данные клиента очереди: %w", err)
	}
	if err := prepareQueue(ctx, q); err != nil {
		return placement{}, err
	}
	data, err := saveCloud(c, q.URL, p.state, creds)
	if err != nil {
		return placement{}, fmt.Errorf("запись настроек облака: %w", err)
	}
	return placeSettings(data, "cloud", p.client)
}
