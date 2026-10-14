package api

import _ "embed"

// Swagger содержит описание API сервера в JSON.
//
//go:embed swagger.json
var Swagger []byte
