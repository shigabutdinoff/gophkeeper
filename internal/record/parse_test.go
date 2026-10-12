package record

import (
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	id     = "0199b5e4-7c3a-7d21-9f4e-2a6c8b1d3e57"
	digest = "SHA-256=47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU="
)

func TestParsePutFile(t *testing.T) {
	data := newKey(t)
	fields := map[string]string{"type": "file", "name": "id_ed25519", "size": "411"}
	msg, err := PutFile(data, anna, id, fields, "ключ SSH", nil)
	require.NoError(t, err, "запись о файле не собрана")
	got, err := Parse(msg.Subject, msg.Data)
	require.NoError(t, err, "запись о файле не разобрана")
	assert.Equal(t, Change{Op: "put", Record: id, Note: "ключ SSH", Secret: got.Secret, Digest: digest, Owner: anna}, got, "разобрана не та запись о файле")
	assert.NotContains(t, string(msg.Data), "id_ed25519", "имя файла ушло открыто")
	opened, err := Open(data, id, got.Secret)
	require.NoError(t, err, "секрет записи о файле не открыт")
	assert.Equal(t, fields, opened, "не те имя и размер файла")
	assert.NotEmpty(t, msg.Header.Get("Nats-Msg-Id"), "нет идентификатора сообщения")
}

func TestParsePut(t *testing.T) {
	msg, id, c := put(t, newKey(t))
	got, err := Parse(msg.Subject, msg.Data)
	require.NoError(t, err, "изменение из Put не разобрано")
	assert.Equal(t, Change{Op: "put", Record: id, Note: c.Note, Secret: c.Secret, Owner: anna}, got, "разобрано не то изменение")
}

func TestParseCanonical(t *testing.T) {
	for _, form := range []string{"urn:uuid:" + id, "{" + id + "}", "0199B5E47C3A7D219F4E2A6C8B1D3E57"} {
		body := `{"op":"put","record":"` + form + `","note":"","secret":"AQ=="}`
		got, err := Parse(Topic(anna), []byte(body))
		require.NoError(t, err, "запись %q не принята", form)
		assert.Equal(t, id, got.Record, "запись %q не в каноническом виде", form)
	}
	got, err := Parse(Subject+"."+strings.ToUpper(anna), []byte(`{"op":"delete","record":"`+id+`"}`))
	require.NoError(t, err, "владелец прописными буквами не принят")
	assert.Equal(t, anna, got.Owner, "владелец не в каноническом виде")
}

func TestParseForged(t *testing.T) {
	body := `{"op":"put","record":"` + id + `","note":"","secret":"czNjcmV0"}`
	cases := []struct{ name, subject, body string }{
		{"no owner", Subject, body}, {"empty owner", Subject + ".", body}, {"owner not uuid", Subject + ".anna", body},
		{"wildcard owner", Subject + ".*", body}, {"deeper subject", Topic(anna) + ".x", body}, {"other subject", "changes.files." + anna, body},
		{"not json", Topic(anna), "s3cret"},
		{"op", Topic(anna), `{"op":"drop","record":"` + id + `","secret":"czNjcmV0"}`},
		{"record", Topic(anna), `{"op":"put","record":"s3cret","secret":"czNjcmV0"}`},
		{"empty secret", Topic(anna), `{"op":"put","record":"` + id + `"}`},
		{"empty edit secret", Topic(anna), `{"op":"edit","record":"` + id + `"}`},
		{"nul note", Topic(anna), `{"op":"put","record":"` + id + `","note":"a\u0000","secret":"czNjcmV0"}`},
		{"edit digest", Topic(anna), `{"op":"edit","record":"` + id + `","secret":"czNjcmV0","digest":"` + digest + `"}`},
		{"delete digest", Topic(anna), `{"op":"delete","record":"` + id + `","digest":"` + digest + `"}`},
		{"short digest", Topic(anna), `{"op":"put","record":"` + id + `","secret":"czNjcmV0","digest":"SHA-256=czNjcmV0"}`},
		{"other digest", Topic(anna), `{"op":"put","record":"` + id + `","secret":"czNjcmV0","digest":"` + strings.Replace(digest, "SHA-256", "SHA-512", 1) + `"}`},
		{"digest not base64", Topic(anna), `{"op":"put","record":"` + id + `","secret":"czNjcmV0","digest":"SHA-256=` + strings.Repeat("!", 44) + `"}`},
	}
	var reasons []string
	for _, c := range cases {
		_, err := Parse(c.subject, []byte(c.body))
		require.ErrorIs(t, err, ErrForged, "подделка %s принята", c.name)
		assert.NotContains(t, err.Error(), "s3cret", "в ошибке %s секрет", c.name)
		reasons = append(reasons, c.name+": "+err.Error())
	}
	snaps.MatchSnapshot(t, reasons)
}

func TestParseEdit(t *testing.T) {
	data := newKey(t)
	msg, err := Edit(data, anna, id, map[string]string{"login": "anna", "password": "n3w-s3cret"}, "почта")
	require.NoError(t, err, "правка не собрана")
	got, err := Parse(msg.Subject, msg.Data)
	require.NoError(t, err, "правка не разобрана")
	assert.Equal(t, []string{"edit", id, "почта", anna}, []string{got.Op, got.Record, got.Note, got.Owner}, "разобрана не та правка")
	fields, err := Open(data, id, got.Secret)
	require.NoError(t, err, "секрет правки не открыт")
	assert.Equal(t, map[string]string{"login": "anna", "password": "n3w-s3cret"}, fields, "не тот новый пароль")
	assert.NotEmpty(t, msg.Header.Get("Nats-Msg-Id"), "нет идентификатора сообщения")
}

func TestParseDelete(t *testing.T) {
	msg, err := Delete(anna, id)
	require.NoError(t, err, "удаление не собрано")
	got, err := Parse(msg.Subject, msg.Data)
	require.NoError(t, err, "удаление не разобрано")
	assert.Equal(t, Change{Op: "delete", Record: id, Owner: anna}, got, "разобрано не то удаление")
}

func TestChangeTooLarge(t *testing.T) {
	data := newKey(t)
	_, _, err := Put(data, anna, map[string]string{"login": "anna"}, strings.Repeat("я", 40000))
	assert.ErrorIs(t, err, ErrTooLarge, "большая пометка принята в Put")
	_, err = Edit(data, anna, id, map[string]string{"type": "text", "text": strings.Repeat("a", 70000)}, "")
	assert.ErrorIs(t, err, ErrTooLarge, "большой текст принят в Edit")
	snaps.MatchSnapshot(t, err.Error())
	_, _, err = Put(data, anna, map[string]string{"login": "anna"}, strings.Repeat("я", 30000))
	assert.NoError(t, err, "пометка в пределах отклонена")
}

func TestParseTooLarge(t *testing.T) {
	body := `{"op":"put","record":"` + id + `","note":"` + strings.Repeat("a", 70000) + `","secret":"czNjcmV0"}`
	_, err := Parse(Topic(anna), []byte(body))
	assert.ErrorIs(t, err, ErrForged, "большое изменение не отброшено")
}
