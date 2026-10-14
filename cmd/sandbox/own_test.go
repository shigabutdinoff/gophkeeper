package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
	"go.yaml.in/yaml/v3"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

var caPEM = regexp.MustCompile(`(?s)-----BEGIN CERTIFICATE-----.*-----END CERTIFICATE-----`)

func fakeDocker() {
	state := "нет"
	if _, err := os.Stat(filepath.Join(os.Getenv("CONFIG"), "gophkeeper", sandboxName)); err == nil {
		state = "есть"
	}
	line := strings.Join(os.Args[1:], " ")
	f, err := os.OpenFile(os.Getenv("DOCKER_JOURNAL"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "журнал подделки:", err)
		os.Exit(2)
	}
	fmt.Fprintf(f, "%s, каталог песочницы: %s\n", line, state)
	f.Close()
	if fail := os.Getenv("DOCKER_FAIL"); fail != "" && strings.Contains(line, fail) {
		fmt.Fprintln(os.Stderr, "Cannot connect to the Docker daemon at unix:///fake.sock. Is the docker daemon running?")
		os.Exit(1)
	}
}

func clientSettings(ts *testscript.TestScript) (config.Settings, string) {
	var c config.Settings
	text := ts.ReadFile(filepath.Join(ts.Getenv("CONFIG"), "gophkeeper", config.File))
	ts.Check(yaml.Unmarshal([]byte(text), &c))
	return c, text
}

func status(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) > 1 {
		ts.Fatalf("использование: status [nokey]")
	}
	c, _ := clientSettings(ts)
	client := &http.Client{Timeout: 5 * time.Second}
	ts.Check(account.Trust(client, c.ServerCert))
	req, err := http.NewRequest(http.MethodGet, c.Server+"/auth/v1/health", nil)
	ts.Check(err)
	if len(args) == 0 {
		req.Header.Set("apikey", c.AppKey)
	}
	resp, err := client.Do(req)
	ts.Check(err)
	resp.Body.Close()
	_, err = fmt.Fprintln(ts.Stdout(), resp.StatusCode)
	ts.Check(err)
}

func signin(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("использование: signin signup|token|refresh|logout")
	}
	c, _ := clientSettings(ts)
	client := &http.Client{Timeout: 5 * time.Second}
	ts.Check(account.Trust(client, c.ServerCert))
	body, path := `{"email":"anna-`+strings.ToLower(c.AppKey)+`@example.com","password":"ключ входа"}`, args[0]+"?grant_type=password"
	switch args[0] {
	case "refresh":
		body, path = `{"refresh_token":"`+ts.Getenv("REFRESH")+`"}`, "token?grant_type=refresh_token"
	case "logout":
		body, path = `{}`, "logout?scope=local"
	}
	req, err := http.NewRequest(http.MethodPost, c.Server+"/auth/v1/"+path, strings.NewReader(body))
	ts.Check(err)
	req.Header.Set("apikey", c.AppKey)
	if args[0] == "logout" {
		req.Header.Set("Authorization", "Bearer "+ts.Getenv("ACCESS"))
	}
	resp, err := client.Do(req)
	ts.Check(err)
	defer resp.Body.Close()
	var got struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
	}
	if resp.StatusCode == http.StatusOK {
		ts.Check(json.NewDecoder(resp.Body).Decode(&got))
		ts.Setenv("ACCESS", got.Access)
		ts.Setenv("REFRESH", got.Refresh)
	}
	_, err = fmt.Fprintln(ts.Stdout(), resp.StatusCode)
	ts.Check(err)
}

func settings(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 0 {
		ts.Fatalf("использование: settings")
	}
	c, text := clientSettings(ts)
	_, port, err := net.SplitHostPort(ownAddr)
	ts.Check(err)
	masked := strings.NewReplacer(ts.Getenv("CONFIG"), "$CONFIG", c.AppKey, "<app_key>", ":"+port, ":<own>",
		":"+strconv.Itoa(sandboxPort), ":<queue>", ":"+strconv.Itoa(sandboxLeafPort), ":<leaf>").Replace(caPEM.ReplaceAllString(text, "<ca>"))
	_, err = fmt.Fprint(ts.Stdout(), masked)
	ts.Check(err)
}

func occupy(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 0 {
		ts.Fatalf("использование: occupy")
	}
	l, err := net.Listen("tcp", ownAddr)
	ts.Check(err)
	ts.Defer(func() { l.Close() })
}
