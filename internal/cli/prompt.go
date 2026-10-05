package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"golang.org/x/term"
)

const minPassword = 8

var (
	isTerminal   = term.IsTerminal
	readPassword = term.ReadPassword
)

func prompt(in io.Reader, errOut io.Writer) (email, password string, err error) {
	fd, err := terminal(in)
	if err != nil {
		return "", "", err
	}
	fmt.Fprint(errOut, "Email: ")
	email, err = bufio.NewReader(in).ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || email == "") {
		return "", "", fmt.Errorf("чтение email: %w", err)
	}
	if password, err = hidden(fd, errOut, "Пароль: "); err != nil {
		return "", "", err
	}
	if utf8.RuneCountInString(password) < minPassword {
		return "", "", fmt.Errorf("пароль должен быть не короче %d символов", minPassword)
	}
	return email, password, nil
}
