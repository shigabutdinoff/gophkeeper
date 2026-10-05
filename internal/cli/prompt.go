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
	f, ok := in.(interface{ Fd() uintptr })
	if !ok || !isTerminal(int(f.Fd())) {
		return "", "", errors.New("пароль вводится только с клавиатуры")
	}
	fmt.Fprint(errOut, "Email: ")
	email, err = bufio.NewReader(in).ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || email == "") {
		return "", "", fmt.Errorf("чтение email: %w", err)
	}
	fmt.Fprint(errOut, "Пароль: ")
	stop := restoreOnInterrupt(int(f.Fd()))
	pass, err := readPassword(int(f.Fd()))
	stop()
	fmt.Fprintln(errOut)
	if err != nil {
		return "", "", fmt.Errorf("чтение пароля: %w", err)
	}
	if utf8.RuneCount(pass) < minPassword {
		return "", "", fmt.Errorf("пароль должен быть не короче %d символов", minPassword)
	}
	return email, string(pass), nil
}
