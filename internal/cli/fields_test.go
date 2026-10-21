package cli

import (
	"bytes"
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func card() map[string]string {
	return map[string]string{"type": "card", "number": "4111 1111 1111 1111", "expiry": "12/28", "holder": "ANNA IVANOVA", "code": "123"}
}

func typing(t *testing.T, values ...string) {
	t.Helper()
	keyboard(t, true)
	setVar(t, &runForm, func(_ *cobra.Command, f *huh.Form) error {
		pump(f, f.Init())
		for _, v := range values {
			enter(f, v)
		}
		return nil
	})
}

func enter(f *huh.Form, value string) {
	for _, r := range value {
		f.Update(tea.KeyPressMsg(tea.Key{Text: string(r), Code: r}))
	}
	pump(f, func() tea.Msg { return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}) })
}

func pump(f *huh.Form, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	got := make(chan tea.Msg, 1)
	go func() { got <- cmd() }()
	select {
	case msg := <-got:
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				pump(f, c)
			}
			return
		}
		_, next := f.Update(msg)
		pump(f, next)
	case <-time.After(20 * time.Millisecond):
	}
}

func fieldsOf(t *testing.T, name string, in *os.File, old map[string]string) (map[string]string, string, error) {
	t.Helper()
	k, err := kindOf(name)
	require.NoError(t, err, "тип %q не найден", name)
	var errOut bytes.Buffer
	c := &cobra.Command{}
	c.SetIn(in)
	c.SetErr(&errOut)
	got, err := askFields(c, k, old)
	return got, errOut.String(), err
}

func TestFieldsText(t *testing.T) {
	keyboard(t, false)
	got, errOut, err := fieldsOf(t, "text", input(t, "abcd-1234\nefgh-5678\n\nijkl-9012\n"), nil)
	require.NoError(t, err, "текст не прочитан")
	assert.Equal(t, map[string]string{"type": "text", "text": "abcd-1234\nefgh-5678\n\nijkl-9012\n"}, got, "текст изменён")
	assert.Empty(t, errOut, "подсказка напечатана без терминала")
}

func TestFieldsTextEmpty(t *testing.T) {
	keyboard(t, true)
	_, errOut, err := fieldsOf(t, "text", input(t, ""), nil)
	assert.EqualError(t, err, "текст не может быть пустым", "пустой текст принят")
	assert.Equal(t, "Текст, конец ввода Ctrl-D:\n", errOut, "нет подсказки в терминале")
	got, _, err := fieldsOf(t, "text", input(t, ""), map[string]string{"type": "text", "text": "прежний"})
	require.NoError(t, err, "пустой ввод при правке отклонён")
	assert.Equal(t, "прежний", got["text"], "прежний текст не сохранён")
}

func TestFieldsCard(t *testing.T) {
	typing(t, "4111 1111 1111 1111", "12/28", "ANNA IVANOVA", "123")
	got, _, err := fieldsOf(t, "card", input(t, ""), nil)
	require.NoError(t, err, "карта не прочитана")
	assert.Equal(t, card(), got, "карта прочитана не та")
}

func TestFieldsCardKept(t *testing.T) {
	typing(t, "", "01/30", "", "")
	got, _, err := fieldsOf(t, "card", input(t, ""), card())
	require.NoError(t, err, "правка карты не прочитана")
	assert.Equal(t, map[string]string{"type": "card", "number": "4111 1111 1111 1111", "expiry": "01/30", "holder": "ANNA IVANOVA", "code": "123"}, got, "прежние поля не сохранены")
}

func TestFieldsCardEmpty(t *testing.T) {
	typing(t, "", "", "", "")
	_, _, err := fieldsOf(t, "card", input(t, ""), nil)
	assert.EqualError(t, err, "номер не может быть пустым", "пустой номер принят")
	typing(t, "4111", "", "", "")
	got, _, err := fieldsOf(t, "card", input(t, ""), nil)
	require.NoError(t, err, "карта без срока, имени и кода отклонена")
	assert.Equal(t, map[string]string{"type": "card", "number": "4111", "expiry": "", "holder": "", "code": ""}, got, "не та карта")
}

func TestFieldsCardNotTerminal(t *testing.T) {
	keyboard(t, false, "4111")
	_, _, err := fieldsOf(t, "card", input(t, "12/28\n"), nil)
	assert.EqualError(t, err, "пароль вводится только с клавиатуры", "карта не с клавиатуры принята")
}

func TestKindUnknown(t *testing.T) {
	_, err := kindOf("bogus")
	assert.EqualError(t, err, `неизвестный тип записи "bogus", допустимы text, card и file`, "неизвестный тип принят")
}

func TestFormQuit(t *testing.T) {
	keyboard(t, true)
	setVar(t, &runForm, func(_ *cobra.Command, f *huh.Form) error {
		pump(f, f.Init())
		for _, r := range "hunter" {
			f.Update(tea.KeyPressMsg(tea.Key{Text: string(r), Code: r}))
		}
		return nil
	})
	c := &cobra.Command{}
	c.SetIn(input(t, ""))
	_, err := askHidden(c, "Пароль")
	assert.ErrorIs(t, err, huh.ErrUserAborted, "оборванная форма не завершила команду")
}

func TestFormPaste(t *testing.T) {
	setVar(t, &isTerminal, func(uintptr) bool { return true })
	r, w, err := os.Pipe()
	require.NoError(t, err, "канал ввода не создан")
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })
	_, err = w.WriteString("s3cret-pass\r")
	require.NoError(t, err, "ввод не записан")
	var errOut bytes.Buffer
	c := &cobra.Command{}
	c.SetIn(r)
	c.SetErr(&errOut)
	pass, err := askHidden(c, "Пароль")
	require.NoError(t, err, "пароль не прочитан")
	assert.Equal(t, "s3cret-pass", pass, "пароль прочитан не тот")
	assert.NotContains(t, errOut.String(), "\x1b[?2004h", "вставка из буфера идёт одним блоком и теряет перевод строки")
}
