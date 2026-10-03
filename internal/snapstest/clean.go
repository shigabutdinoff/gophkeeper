package snapstest

import (
	"log"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/rogpeppe/go-internal/testscript"
)

// CleanM запускает тесты пакета и возвращает ненулевой код, если снапшоты
// не проверены или среди них остались устаревшие.
type CleanM struct {
	// M запускает тесты пакета.
	*testing.M
}

var _ testscript.TestingM = CleanM{}

// Run выполняет тесты и проверку снапшотов и возвращает код завершения.
func (m CleanM) Run() int {
	code := m.M.Run()
	dirty, err := snaps.Clean(m.M)
	return exitCode(code, dirty, err)
}

func exitCode(code int, dirty bool, err error) int {
	if err != nil {
		log.Print("снапшоты не проверены: ", err)
	}
	if dirty || err != nil {
		code = 1
	}
	return code
}
