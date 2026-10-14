package server

import (
	"io"
	"net/http"
)

const xmlHead = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Error xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Resource>`

type s3Error struct {
	status        int
	code, message string
}

var (
	noToken      = s3Error{http.StatusForbidden, "InvalidAccessKeyId", "The Access Key Id you provided does not exist in our records."}
	badToken     = s3Error{http.StatusForbidden, "AccessDenied", "signature verification failed"}
	putDenied    = s3Error{http.StatusForbidden, "AccessDenied", "new row violates row-level security policy"}
	deleteDenied = s3Error{http.StatusForbidden, "AccessDenied", "Access denied"}
	noObject     = s3Error{http.StatusNotFound, "NoSuchKey", "Object not found"}
	tooLarge     = s3Error{http.StatusRequestEntityTooLarge, "EntityTooLarge", "The object exceeded the maximum allowed size"}
	s3Failed     = s3Error{http.StatusInternalServerError, "InternalError", "Internal Server Error"}
)

func (e s3Error) send(w http.ResponseWriter, resource string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(e.status)
	io.WriteString(w, xmlHead+resource+"</Resource><Code>"+e.code+"</Code><Message>"+e.message+"</Message></Error>")
}
