package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/api"
)

func described() (*openapi3.T, error) {
	var v2 openapi2.T
	if err := json.Unmarshal(api.Swagger, &v2); err != nil {
		return nil, err
	}
	doc, err := openapi2conv.ToV3(&v2)
	if err != nil {
		return nil, err
	}
	doc.Servers = nil
	return doc, doc.Validate(context.Background())
}

func router() (routers.Router, error) {
	doc, err := described()
	if err != nil {
		return nil, err
	}
	return gorillamux.NewRouter(doc)
}

type checked struct{ base http.RoundTripper }

func (c checked) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	rt, err := router()
	if err != nil {
		return resp, err
	}
	route, params, err := rt.FindRoute(req)
	if err != nil {
		return resp, nil
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	opts := &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}
	err = openapi3filter.ValidateResponse(req.Context(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route, Options: opts},
		Status:                 resp.StatusCode, Header: resp.Header, Body: io.NopCloser(bytes.NewReader(body)), Options: opts,
	})
	return resp, err
}

func TestSwaggerDescribed(t *testing.T) {
	doc, err := described()
	require.NoError(t, err, "описание API не прошло проверку OpenAPI")
	var ops []string
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			ops = append(ops, method+" "+path)
		}
	}
	slices.Sort(ops)
	snaps.MatchSnapshot(t, ops)
}

func TestSwaggerServed(t *testing.T) {
	srv := httptest.NewServer(offline(t).routes(Settings{AppKey: "app-key", ServerKey: "service-key"}))
	t.Cleanup(srv.Close)
	resp, err := srv.Client().Get(srv.URL + "/swagger/doc.json")
	require.NoError(t, err, "сервер не ответил")
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "ответ не прочитан")
	require.Equal(t, http.StatusOK, resp.StatusCode, "описание отдаётся без ключа проекта")
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"), "описание отдаётся в JSON")
	require.Equal(t, api.Swagger, body, "сервер отдаёт описание из пакета api")
}

func TestSwaggerChecksResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		respond(w, http.StatusOK, `[{"id":1}]`)
	}))
	t.Cleanup(srv.Close)
	client := &http.Client{Transport: checked{srv.Client().Transport}}
	resp, err := client.Get(srv.URL + "/rest/v1/records")
	if err == nil {
		_ = resp.Body.Close()
	}
	require.ErrorContains(t, err, "response body doesn't match schema", "ответ не по описанию не прошёл")
	resp, err = client.Get(srv.URL + "/swagger/doc.json")
	require.NoError(t, err, "адрес вне описания не проверяется")
	_ = resp.Body.Close()
}
