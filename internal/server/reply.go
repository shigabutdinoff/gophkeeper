package server

const (
	exists  = `{"code":422,"error_code":"user_already_exists","msg":"User already registered"}`
	wrong   = `{"code":400,"error_code":"invalid_credentials","msg":"Invalid login credentials"}`
	invalid = `{"code":400,"error_code":"validation_failed","msg":"Invalid request"}`
	failed  = `{"code":500,"error_code":"unexpected_failure","msg":"Unexpected failure"}`
	gone    = `{"code":400,"error_code":"refresh_token_not_found","msg":"Invalid Refresh Token: Refresh Token Not Found"}`
)
