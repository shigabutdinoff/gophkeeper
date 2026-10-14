package server

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
)

var varying = []string{"access_token", "refresh_token", "id", "user_id", "identity_id", "sub", "email"}

func masked(t *testing.T, body string) string {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(body))
	dec.UseNumber()
	var b strings.Builder
	maskValue(t, dec, &b, "")
	_, err := dec.Token()
	require.ErrorIs(t, err, io.EOF, "после ответа остались данные: %s", body)
	if strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

func maskValue(t *testing.T, dec *json.Decoder, b *strings.Builder, key string) {
	t.Helper()
	tok, err := dec.Token()
	require.NoError(t, err, "ответ не JSON")
	delim, nested := tok.(json.Delim)
	switch {
	case nested && delim == '{':
		b.WriteString("{")
		for i := 0; dec.More(); i++ {
			name, err := dec.Token()
			require.NoError(t, err, "ответ не JSON")
			b.WriteString(strings.Repeat(",", min(i, 1)) + strconv.Quote(name.(string)) + ":")
			maskValue(t, dec, b, name.(string))
		}
	case nested:
		b.WriteString("[")
		for i := 0; dec.More(); i++ {
			b.WriteString(strings.Repeat(",", min(i, 1)))
			maskValue(t, dec, b, key)
		}
	case tok != nil && (slices.Contains(varying, key) || strings.HasSuffix(key, "_at")):
		b.WriteString(strconv.Quote("<" + key + ">"))
		return
	default:
		raw, err := json.Marshal(tok)
		require.NoError(t, err, "значение не закодировано")
		b.Write(raw)
		return
	}
	end, err := dec.Token()
	require.NoError(t, err, "ответ не JSON")
	b.WriteString(end.(json.Delim).String())
}

func reference(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err, "эталон Supabase не прочитан")
	return string(raw)
}

func headed(status int, h http.Header, body string) string {
	var lines []string
	for _, name := range []string{"proxy-status", "www-authenticate"} {
		if v := h.Get(name); v != "" {
			lines = append(lines, name+": "+v+"\n")
		}
	}
	return strconv.Itoa(status) + "\n" + strings.Join(lines, "") + body
}

func answer(status int, body string) string {
	return strconv.Itoa(status) + "\n" + body
}

type recorder struct {
	next   http.RoundTripper
	status int
	header http.Header
	body   []byte
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(body))
	r.status, r.header, r.body = resp.StatusCode, resp.Header, body
	return resp, err
}

func (r *recorder) last(owner, id string) string {
	lines := []string{strconv.Itoa(r.status) + "\n"}
	for _, name := range []string{"cache-control", "content-length", "content-type", "etag", "last-modified"} {
		v := r.header.Get(name)
		switch {
		case v == "":
			continue
		case name == "etag":
			v = "<etag>"
		case name == "last-modified":
			v = "<date>"
		}
		lines = append(lines, name+": "+v+"\n")
	}
	body := strings.ReplaceAll(strings.ReplaceAll(string(r.body), owner, "<owner>"), id, "<id>")
	return strings.Join(lines, "") + body
}

func rawObject(t *testing.T, client *http.Client, server, app, token, key string) error {
	t.Helper()
	raw := s3.New(s3.Options{BaseEndpoint: aws.String(server + "/storage/v1/s3"), UsePathStyle: true, Region: cmp.Or(os.Getenv("SUPABASE_REGION"), "us-east-1"),
		HTTPClient: client, Retryer: aws.NopRetryer{},
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: strings.Split(strings.TrimPrefix(server, "https://"), ".")[0], SecretAccessKey: app, SessionToken: token}, nil
		})})
	_, err := raw.GetObject(t.Context(), &s3.GetObjectInput{Bucket: aws.String("files"), Key: aws.String(key)})
	return err
}
