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

func TestFormQuit(t *testing.T) {
	in := keyboard(t, true, "")
	setVar(t, &runForm, func(_ *cobra.Command, f *huh.Form) error {
		pump(f, f.Init())
		for _, r := range "hunter" {
			f.Update(tea.KeyPressMsg(tea.Key{Text: string(r), Code: r}))
		}
		return nil
	})
	code := 0
	setVar(t, &exit, func(c int) { code = c })
	c := &cobra.Command{}
	c.SetIn(in)
	_, err := askHidden(c, "Пароль")
	assert.Error(t, err, "недовведённый пароль принят")
	assert.Equal(t, interruptCode, code, "оборванная форма не завершила команду")
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
