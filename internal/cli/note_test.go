package cli

import (
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"
)

func TestListNote(t *testing.T) {
	recs := []stored{sealed(t, "anna", "s3cret", "Сбербанк, зарплатная"), sealed(t, "anna", "pin", "сайт example.com")}
	srv := serveRecords(t, replyJSON(t, recs))
	login(t)
	listed(t)
	found, none := listed(t, "--note", "сбербанк"), listed(t, "--note", "почта")
	srv.Close()
	all, err := noServer(t, "", "list")
	require.NoError(t, err, "список без сервера не показан")
	offline, err := noServer(t, "", "list", "--note", "САЙТ")
	require.NoError(t, err, "поиск без сервера не прошёл")
	snaps.MatchSnapshot(t, recordID.ReplaceAllString(found, "<id>"), none,
		recordID.ReplaceAllString(all, "<id>"), recordID.ReplaceAllString(offline, "<id>"))
}

func TestListNoteWithID(t *testing.T) {
	_, _, err := executeIn(keyboard(t, true, ""), "list", "--note", "почта", "id")
	require.Error(t, err, "поиск с идентификатором принят")
	snaps.MatchSnapshot(t, err.Error())
}
