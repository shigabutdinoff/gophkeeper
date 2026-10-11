package cli

import (
	"fmt"
	"slices"

	"charm.land/huh/v2"
)

type field struct {
	key, label string
	need       bool
	echo       huh.EchoMode
}

type kind struct {
	name, title string
	fields      []field
}

var kinds = []kind{
	{"", "логин и пароль", []field{{"login", "Логин", false, 0}, {"password", "Пароль", true, 0}}},
	{"text", "текст", []field{{"text", "Текст", true, 0}}},
	{"card", "банковская карта", []field{
		{"number", "Номер", true, huh.EchoModePassword}, {"expiry", "Срок", false, huh.EchoModeNormal},
		{"holder", "Имя", false, huh.EchoModeNormal}, {"code", "Код", false, huh.EchoModePassword},
	}},
}

func kindOf(name string) (kind, error) {
	i := slices.IndexFunc(kinds, func(k kind) bool { return k.name == name })
	if i < 0 {
		return kind{}, fmt.Errorf("неизвестный тип записи %q, допустимы text и card", name)
	}
	return kinds[i], nil
}
