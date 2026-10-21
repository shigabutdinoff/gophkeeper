package cli

import (
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
)

func TestOwnServerNote(t *testing.T) {
	s, dsn := startOwnServer(t)
	recs := []stored{sealed(t, "anna", "s3cret", "Сбербанк, зарплатная"), sealed(t, "anna", "s3cret", "сайт example.com")}
	putOwn(t, dsn, registerOwn(t, s, dsn), recs)
	snaps.MatchSnapshot(t, recordID.ReplaceAllString(listed(t, "--note", "сбербанк"), "<id>"))
}
