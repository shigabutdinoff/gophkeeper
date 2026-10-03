package main

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func FuzzFormat(f *testing.F) {
	for _, tc := range formatCases {
		f.Add(tc.version, uint16(tc.count))
	}
	number := regexp.MustCompile(`^\d+[A-Z](\d+|5\d{3,}[a-z])$`)
	russian := regexp.MustCompile(`^(неверная версия|минор \d+ больше 25|бета \d+ больше 26)`)
	f.Fuzz(func(t *testing.T, s string, count uint16) {
		v, err := parse(s)
		if err != nil {
			assert.Regexp(t, russian, err.Error(), "ошибка разбора не на русском")
			return
		}
		assert.Regexp(t, number, format(v, int(count)), "номер сборки для %q не в форме Apple", s)
	})
}
