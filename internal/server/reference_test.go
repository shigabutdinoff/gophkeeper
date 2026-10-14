package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

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
