package server

const (
	exists  = `{"code":422,"error_code":"user_already_exists","msg":"User already registered"}`
	wrong   = `{"code":400,"error_code":"invalid_credentials","msg":"Invalid login credentials"}`
	invalid = `{"code":400,"error_code":"validation_failed","msg":"Invalid request"}`
	failed  = `{"code":500,"error_code":"unexpected_failure","msg":"Unexpected failure"}`
	gone    = `{"code":400,"error_code":"refresh_token_not_found","msg":"Invalid Refresh Token: Refresh Token Not Found"}`
)

const (
	jwtInvalid = `{"code":"PGRST301","details":"None of the keys was able to decode the JWT","hint":null,"message":"No suitable key or wrong key type"}`
	syntax     = `{"code":"22P02","details":null,"hint":null,"message":"invalid input syntax"}`
	internal   = `{"code":"XX000","details":null,"hint":null,"message":"Internal server error"}`
	orphan     = `{"code":"23503","details":null,"hint":null,"message":"insert or update on table \"records\" violates foreign key constraint \"records_user_id_fkey\""}`
)
