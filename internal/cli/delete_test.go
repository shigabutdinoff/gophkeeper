package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDelete(t *testing.T) {
	rec := sealed(t, "anna", "s3cret", "")
	_, stream := withQueue(t, replyJSON(t, []stored{rec}))
	out, errOut, err := executeIn(keyboard(t, true, ""), "delete", rec.ID)
	require.NoError(t, err, "удаление не принято")
	assert.Equal(t, "Изменение принято, запись "+rec.ID+".\n", out, "не тот ответ")
	assert.Empty(t, errOut, "удаление спрашивало пароль")
	c := lastChange(t, stream)
	assert.Equal(t, []string{"delete", rec.ID}, []string{c.Op, c.Record}, "в очереди не то удаление")
}

func TestDeleteOffline(t *testing.T) {
	srv, stream := withQueue(t, replyJSON(t, []stored{}))
	srv.Close()
	out, _, err := executeIn(keyboard(t, true, ""), "delete", missing)
	require.NoError(t, err, "удаление без сервера не принято")
	assert.Contains(t, out, "Изменение принято, запись "+missing+".", "не тот ответ")
	assert.Equal(t, "delete", lastChange(t, stream).Op, "в очереди не удаление")
}

func TestDeleteTypes(t *testing.T) {
	for _, rec := range []stored{sealedAs(t, codes, ""), sealedAs(t, card, "")} {
		_, stream := withQueue(t, replyJSON(t, []stored{rec}))
		_, _, err := executeIn(keyboard(t, true, ""), "delete", rec.ID)
		require.NoError(t, err, "удаление не принято")
		c := lastChange(t, stream)
		assert.Equal(t, []string{"delete", rec.ID}, []string{c.Op, c.Record}, "в очереди не то удаление")
	}
}
